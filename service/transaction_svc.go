package service

import (
	"errors"
	"fmt"
	"my-ledger/server/dto"
	"my-ledger/server/model"
	"my-ledger/server/repository"
	"regexp"
	"strconv"
	"time"
)

// TransactionService 交易业务逻辑层
type TransactionService struct {
	txnRepo *repository.TransactionRepository
	accRepo *repository.AccountRepository
	catRepo *repository.CategoryRepository
}

// NewTransactionService 创建交易服务实例
func NewTransactionService(
	txnRepo *repository.TransactionRepository,
	accRepo *repository.AccountRepository,
	catRepo *repository.CategoryRepository,
) *TransactionService {
	return &TransactionService{
		txnRepo: txnRepo,
		accRepo: accRepo,
		catRepo: catRepo,
	}
}

// ListTransactions 分页查询交易记录
func (s *TransactionService) ListTransactions(req *dto.QueryTransactionRequest) (*dto.PageResult, error) {
	// 校验并规范化日期（可选过滤条件：空即不过滤；规范化后写回 req，
	// 使 repo 中 date >= ? / date <= ? 字符串比较按 YYYY-MM-DD 正确生效）
	if req.StartDate != "" {
		normalized, err := parseDate(req.StartDate)
		if err != nil {
			return nil, &repository.AppError{Code: 40002, Message: "日期格式无效"}
		}
		req.StartDate = normalized
	}
	if req.EndDate != "" {
		normalized, err := parseDate(req.EndDate)
		if err != nil {
			return nil, &repository.AppError{Code: 40002, Message: "日期格式无效"}
		}
		req.EndDate = normalized
	}

	items, total, err := s.txnRepo.FindPage(s.txnRepo.DB, req)
	if err != nil {
		return nil, err
	}
	return &dto.PageResult{
		Items:    items,
		Total:    total,
		PageSize: req.PageSize,
		PageNum:  req.PageNum,
	}, nil
}

// GetTransaction 查询单条交易记录
func (s *TransactionService) GetTransaction(id int64) (*model.Transaction, error) {
	return s.txnRepo.FindByID(s.txnRepo.DB, id)
}

// CreateTransaction 新增交易记录，联动更新账户余额
func (s *TransactionService) CreateTransaction(req *dto.CreateTransactionRequest) (*model.Transaction, error) {
	// 业务校验
	if err := s.validateCreate(req); err != nil {
		return nil, err
	}

	// 开启事务
	tx, err := s.txnRepo.DB.Beginx()
	if err != nil {
		return nil, err
	}
	// 无条件回滚：Commit 成功后 Rollback 返回 sql.ErrTxDone 为空操作；
	// 中途失败 return 时自动回滚，不依赖 err 变量判断回滚（避免 if err := 遮蔽导致漏回滚）
	defer tx.Rollback()

	// 构建交易模型
	t := &model.Transaction{
		Type:        req.Type,
		Amount:      req.Amount,
		CategoryID:  req.CategoryID,
		AccountID:   req.AccountID,
		ToAccountID: orNoAccountID(req.ToAccountID), // nil 归一化为 -1“无账户”哨兵值
		Date:        req.Date,
		Note:        req.Note,
		CreateTime:  model.DateTimeStr(time.Now()),
		UpdateTime:  model.DateTimeStr(time.Now()),
	}

	// 插入交易记录
	id, err := s.txnRepo.Create(tx, t)
	if err != nil {
		return nil, err
	}
	t.ID = id

	// 应用余额变更
	if err := s.applyBalanceOps(tx, t, 1); err != nil {
		return nil, err
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.txnRepo.FindByID(s.txnRepo.DB, id)
}

// UpdateTransaction 更新交易记录，联动更新账户余额（回滚旧值 + 应用新值）
func (s *TransactionService) UpdateTransaction(id int64, req *dto.UpdateTransactionRequest) (*model.Transaction, error) {
	// 开启事务
	tx, err := s.txnRepo.DB.Beginx()
	if err != nil {
		return nil, err
	}
	// 无条件回滚：Commit 成功后 Rollback 返回 sql.ErrTxDone 为空操作；
	// 中途失败 return 时自动回滚，不依赖 err 变量判断回滚（避免 if err := 遮蔽导致漏回滚）
	defer tx.Rollback()

	// 读取旧记录（事务内读取，防止并发问题）
	old, err := s.txnRepo.FindByID(tx, id)
	if err != nil {
		return nil, err
	}

	// 回滚旧余额
	if err := s.applyBalanceOps(tx, old, -1); err != nil {
		return nil, err
	}

	// 合并更新字段
	newTxn := s.mergeUpdate(old, req)

	// 更新转账目标账户校验：按生效类型判断（请求指定 type 优先，缺省沿用旧记录类型）。
	// 转账交易显式传 toAccountId:null 直接报错（与 create 口径一致）——本块虽在 mergeUpdate 之后，
	// 但读取的是请求原始三态（Present/IsNull），不受归一化影响；勿改成判断 newTxn.ToAccountID，
	// 其 null 已被归一化为 -1，会漏判成“转账→无账户”入库，资金无入账方。
	// 未携带该字段保留旧值；传数字/-1 放行走后续 validateMerged 校验；
	// 非转账时显式 null 仍由 mergeUpdate 归一化为 -1（清空目标场景，保留）
	effectiveType := old.Type
	if req.Type != nil {
		effectiveType = *req.Type
	}
	if effectiveType == "transfer" && req.ToAccountID == nil {
		return nil, &repository.AppError{Code: 40001, Message: "转账必须指定目标账户"}
	}

	// 业务校验（基于合并后的完整记录）
	if err := s.validateMerged(newTxn); err != nil {
		return nil, err
	}

	// 回写规范化日期到请求（validateMerged 已将 newTxn.Date 统一为 YYYY-MM-DD），
	// 保证 buildUpdateFields 生成的入库 date 字段为规范化值；
	// 保留 req.Date 指针本身与 nil 三态语义（nil 表示未携带，date 不进 fields）
	if req.Date != nil {
		*req.Date = newTxn.Date
	}

	// 更新数据库
	fields := s.buildUpdateFields(req)
	if len(fields) > 0 {
		if err := s.txnRepo.Update(tx, id, fields); err != nil {
			return nil, err
		}
	}

	// 应用新余额
	if err := s.applyBalanceOps(tx, newTxn, 1); err != nil {
		return nil, err
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.txnRepo.FindByID(s.txnRepo.DB, id)
}

// DeleteTransaction 删除交易记录，回滚账户余额
func (s *TransactionService) DeleteTransaction(id int64) error {
	// 开启事务
	tx, err := s.txnRepo.DB.Beginx()
	if err != nil {
		return err
	}
	// 无条件回滚：Commit 成功后 Rollback 返回 sql.ErrTxDone 为空操作；
	// 中途失败 return 时自动回滚，不依赖 err 变量判断回滚（避免 if err := 遮蔽导致漏回滚）
	defer tx.Rollback()

	// 读取旧记录
	old, err := s.txnRepo.FindByID(tx, id)
	if err != nil {
		return err
	}

	// 回滚余额
	if err := s.applyBalanceOps(tx, old, -1); err != nil {
		return err
	}

	// 删除记录
	if err := s.txnRepo.Delete(tx, id); err != nil {
		return err
	}

	return tx.Commit()
}

// GetDailyTransactions 获取指定日期的所有交易记录（含关联信息）
func (s *TransactionService) GetDailyTransactions(req *dto.DailyTransactionsRequest) ([]dto.DailyTransactionItem, error) {
	// 如果未指定日期，默认当天
	date := req.Date
	if date == "" {
		date = time.Now().Format(time.DateOnly)
	}

	// 校验并规范化日期（失败返回 40002，成功用规范化值继续查询）
	normalized, err := parseDate(date)
	if err != nil {
		return nil, &repository.AppError{Code: 40002, Message: "日期格式无效"}
	}

	return s.txnRepo.FindDailyTransactions(s.txnRepo.DB, normalized)
}

// GetDailySummary 获取按天汇总统计（收入、支出、结余）
func (s *TransactionService) GetDailySummary(req *dto.DailySummaryRequest) ([]dto.DailySummaryItem, error) {
	// 如果未指定日期，默认当天
	startDate := req.StartDate
	endDate := req.EndDate
	if startDate == "" {
		startDate = time.Now().Format(time.DateOnly)
	}
	if endDate == "" {
		endDate = startDate
	}

	// 校验并规范化日期（失败返回 40002；规范化后再比大小，保证 YYYY-MM-DD 字符串可比）
	normStart, err := parseDate(startDate)
	if err != nil {
		return nil, &repository.AppError{Code: 40002, Message: "日期格式无效"}
	}
	normEnd, err := parseDate(endDate)
	if err != nil {
		return nil, &repository.AppError{Code: 40002, Message: "日期格式无效"}
	}

	// 校验日期范围
	if normStart > normEnd {
		return nil, &repository.AppError{Code: 40003, Message: "开始日期不能大于结束日期"}
	}

	return s.txnRepo.GetDailySummary(s.txnRepo.DB, normStart, normEnd)
}

// balanceOp 余额操作
type balanceOp struct {
	accountID int64
	delta     float64
}

// calcBalanceOps 计算余额变更操作
// sign=+1 表示应用（新增），sign=-1 表示回滚（删除/更新前的旧值）
func calcBalanceOps(t *model.Transaction, sign float64) []balanceOp {
	var ops []balanceOp
	switch t.Type {
	case "expense":
		// 支出：从账户扣除（账户ID为-1表示虚拟账户，跳过余额变更）
		if t.AccountID != -1 {
			ops = append(ops, balanceOp{accountID: t.AccountID, delta: -float64(t.Amount) * sign})
		}
	case "income":
		// 收入：向账户增加（账户ID为-1表示虚拟账户，跳过余额变更）
		if t.AccountID != -1 {
			ops = append(ops, balanceOp{accountID: t.AccountID, delta: float64(t.Amount) * sign})
		}
	case "transfer":
		// 转账：从来源扣除，向目标增加（账户ID为-1表示虚拟账户，跳过余额变更）
		if t.AccountID != -1 {
			ops = append(ops, balanceOp{accountID: t.AccountID, delta: -float64(t.Amount) * sign})
		}
		if t.ToAccountID != -1 {
			ops = append(ops, balanceOp{accountID: t.ToAccountID, delta: float64(t.Amount) * sign})
		}
	}
	return ops
}

// applyBalanceOps 执行余额变更
func (s *TransactionService) applyBalanceOps(tx repository.DBTX, t *model.Transaction, sign float64) error {

	ops := calcBalanceOps(t, sign)
	for _, op := range ops {
		if err := s.accRepo.AdjustBalance(tx, op.accountID, op.delta); err != nil {
			return fmt.Errorf("更新账户 %d 余额失败: %w", op.accountID, err)
		}
	}
	return nil
}

// 校验新增交易请求（金额、日期、账户、分类及转账规则）
func (s *TransactionService) validateCreate(req *dto.CreateTransactionRequest) error {
	// 校验金额
	if req.Amount <= 0 {
		return repository.ErrParamInvalid
	}

	// 校验并规范化日期，写回 req.Date 使入库数据统一为 YYYY-MM-DD
	// （CreateTransaction 中 model 构建在本校验之后，取 req.Date 即规范化值）
	normalized, err := parseDate(req.Date)
	if err != nil {
		return &repository.AppError{Code: 40002, Message: "日期格式无效"}
	}
	req.Date = normalized

	// 校验账户存在（账户ID为-1表示虚拟账户，跳过校验）
	if req.AccountID != -1 {
		if _, err := s.accRepo.FindByID(req.AccountID); err != nil {
			return &repository.AppError{Code: 40402, Message: "付款账户不存在"}
		}
	}

	// 校验分类存在
	if req.CategoryID != -1 {
		if _, err := s.catRepo.FindByID(req.CategoryID); err != nil {
			return &repository.AppError{Code: 40403, Message: "分类不存在"}
		}
	}

	// 转账特殊校验（-1 为“无账户”哨兵值：任一端为 -1 即非真实账户，不构成同账户冲突，故同为 -1 或一端为 -1 均放行）
	if req.Type == "transfer" {
		// 转账必须指定目标账户（-1 是合法的“无账户”哨兵值，不算 nil）
		if req.ToAccountID == nil {
			return &repository.AppError{Code: 40001, Message: "转账必须指定目标账户"}
		}
		// 仅来源、目标均为真实账户且相同时才拦截
		if req.AccountID != -1 && *req.ToAccountID == req.AccountID {
			return &repository.AppError{Code: 40001, Message: "转账来源和目标不能相同"}
		}
		// 校验目标账户存在（账户ID为-1表示虚拟账户，跳过校验）
		if *req.ToAccountID != -1 {
			if _, err := s.accRepo.FindByID(*req.ToAccountID); err != nil {
				return &repository.AppError{Code: 40402, Message: "目标账户不存在"}
			}
		}
	} else {
		// 非转账类型不能有目标账户（-1 表示未指定目标账户，不算违规，放行）
		if req.ToAccountID != nil && *req.ToAccountID != -1 {
			return &repository.AppError{Code: 40001, Message: "非转账交易不能指定目标账户"}
		}
	}

	return nil
}

// 校验合并后的交易记录（更新场景）
func (s *TransactionService) validateMerged(t *model.Transaction) error {
	if t.Amount <= 0 {
		return repository.ErrParamInvalid
	}
	// 校验并规范化日期，写回 t.Date 保证合并后记录日期为 YYYY-MM-DD
	normalized, err := parseDate(t.Date)
	if err != nil {
		return &repository.AppError{Code: 40002, Message: "日期格式无效"}
	}
	t.Date = normalized
	// 校验账户存在（账户ID为-1表示虚拟账户，跳过校验）
	if t.AccountID != -1 {
		if _, err := s.accRepo.FindByID(t.AccountID); err != nil {
			return &repository.AppError{Code: 40402, Message: "付款账户不存在"}
		}
	}
	// 校验分类存在（-1 为“无分类”哨兵值（CSV 导入/余额校正写入），跳过校验
	if t.CategoryID != -1 {
		if _, err := s.catRepo.FindByID(t.CategoryID); err != nil {
			return &repository.AppError{Code: 40403, Message: "分类不存在"}
		}
	}
	// 转账特殊校验（-1 为“无账户”哨兵值：任一端为 -1 即非真实账户，不构成同账户冲突，故同为 -1 或一端为 -1 均放行）
	if t.Type == "transfer" {
		// 仅来源、目标均为真实账户且相同时才拦截
		if t.AccountID != -1 && t.ToAccountID == t.AccountID {
			return &repository.AppError{Code: 40001, Message: "转账来源和目标不能相同"}
		}
		// 校验目标账户存在（-1 同为“无账户”哨兵值，跳过校验）
		if t.ToAccountID != -1 {
			if _, err := s.accRepo.FindByID(t.ToAccountID); err != nil {
				return &repository.AppError{Code: 40402, Message: "目标账户不存在"}
			}
		}
	} else {
		// 非转账类型不能有目标账户（-1 表示未指定目标账户，不算违规，放行）
		if t.ToAccountID != -1 {
			return &repository.AppError{Code: 40001, Message: "非转账交易不能指定目标账户"}
		}
	}
	return nil
}

// 将更新请求合并到旧记录上，返回完整的新记录
func (s *TransactionService) mergeUpdate(old *model.Transaction, req *dto.UpdateTransactionRequest) *model.Transaction {
	merged := *old // 复制一份

	if req.Type != nil {
		merged.Type = *req.Type
	}
	if req.Amount != nil {
		merged.Amount = *req.Amount
	}
	if req.CategoryID != nil {
		merged.CategoryID = *req.CategoryID
	}
	if req.AccountID != nil {
		merged.AccountID = *req.AccountID
	}
	if req.ToAccountID != nil {
		merged.ToAccountID = *req.ToAccountID
	}
	if req.Date != nil {
		merged.Date = *req.Date
	}
	if req.Note != nil {
		merged.Note = req.Note
	}
	return &merged
}

// 构建更新字段 map
func (s *TransactionService) buildUpdateFields(req *dto.UpdateTransactionRequest) map[string]interface{} {
	fields := make(map[string]interface{})

	if req.Type != nil {
		fields["type"] = *req.Type
	}
	if req.Amount != nil {
		fields["amount"] = *req.Amount
	}
	if req.CategoryID != nil {
		fields["category_id"] = req.CategoryID
	}
	if req.AccountID != nil {
		fields["account_id"] = *req.AccountID
	}
	if req.ToAccountID != nil {
		fields["to_account_id"] = req.ToAccountID
	}
	if req.Date != nil {
		fields["date"] = *req.Date
	}
	if req.Note != nil {
		fields["note"] = *req.Note
	}
	return fields
}

// 日期时间正则：匹配 YYYY-M-D / YYYY-MM-DD，其后可跟一个空格与 HH:MM 或 HH:MM:SS；
// 除年份固定 4 位外，月日时分秒均为 1-2 位（前导零可选）。
// 6 个捕获组依次为年、月、日、时、分、秒，未带时间部分时后 3 组为空字符串。
var dateTimeRegex = regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})(?: (\d{1,2}):(\d{1,2})(?::(\d{1,2}))?)?$`)

// errInvalidDate 日期解析失败的统一错误：格式不匹配、日期不存在、时间越界均返回它
// （parseDate 内部使用，不构造 repository.AppError，由调用方包装为业务错误）
var errInvalidDate = errors.New("日期格式无效")

// 解析并规范化日期（或日期时间）字符串，供交易与备份等模块统一处理日期入参。
// 输入不规范的日期（如 2024-1-5）会被归一化；成功返回统一的 YYYY-MM-DD 格式字符串
// （time.DateOnly），失败统一返回 errInvalidDate（只表达"日期格式无效"，不区分
// 格式不匹配、日期不存在、时间越界），由调用方包装为业务错误。
//
// 支持格式：
//   - 纯日期：YYYY-M-D、YYYY-MM-DD（前导零可选）
//   - 日期时间：日期后跟空格与 HH:MM 或 HH:MM:SS（前导零可选）
//
// 校验语义：日期必须真实存在（含闰年 2 月 29 日），时间部分时 0-23、分秒 0-59；
// 时间部分仅参与合法性校验，校验通过后丢弃，返回值只保留日期部分
// （如 2024-1-5 8:30:59 → 2024-01-05）。
// 实现思路：正则提取各分量 → time.Date 构造时间（其会把越界值归一化，如 2024-02-30
// 变为 2024-03-01），回比各分量即可识别不存在的日期；时间分量仅做范围检查。
func parseDate(date string) (string, error) {
	m := dateTimeRegex.FindStringSubmatch(date)
	if m == nil {
		return "", errInvalidDate
	}
	// 分量均为纯数字，转换不会失败
	year, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	day, _ := strconv.Atoi(m[3])
	// time.Date 会归一化越界值（如 2024-02-30 变成 2024-03-01），回比即可识别非法日期
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Year() != year || int(t.Month()) != month || t.Day() != day {
		return "", errInvalidDate
	}
	// 未带时间部分，直接返回规范化日期
	if m[4] == "" {
		return t.Format(time.DateOnly), nil
	}
	hour, _ := strconv.Atoi(m[4])
	minute, _ := strconv.Atoi(m[5])
	if hour > 23 || minute > 59 {
		return "", errInvalidDate
	}
	if m[6] != "" {
		second, _ := strconv.Atoi(m[6])
		if second > 59 {
			return "", errInvalidDate
		}
	}
	// 时间部分校验合法后丢弃，只返回日期部分
	return t.Format(time.DateOnly), nil
}

// orNoAccountID 将目标账户 ID 归一化：nil 返回指向 -1“无账户”哨兵值的指针，非 nil 原样返回
func orNoAccountID(v *int64) int64 {
	if v == nil {
		return -1
	}
	return *v
}
