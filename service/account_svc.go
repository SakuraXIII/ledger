// Package service 业务逻辑层
package service

import (
	"log/slog"
	"my-ledger/server/dto"
	"my-ledger/server/model"
	"my-ledger/server/repository"
	"time"
)

// AccountService 账户业务逻辑层
type AccountService struct {
	repo    *repository.AccountRepository
	txnRepo *repository.TransactionRepository
}

// NewAccountService 创建账户服务实例
func NewAccountService(repo *repository.AccountRepository, txnRepo *repository.TransactionRepository) *AccountService {
	return &AccountService{repo: repo, txnRepo: txnRepo}
}

// ListAccounts 查询账户列表，支持按活跃状态筛选
func (s *AccountService) ListAccounts(req *dto.QueryAccountRequest) ([]model.Account, error) {
	return s.repo.FindAll(req)
}

// GetAccount 查询单个账户
func (s *AccountService) GetAccount(id int64) (*model.Account, error) {
	return s.repo.FindByID(id)
}

// CreateAccount 新增账户
func (s *AccountService) CreateAccount(req *dto.CreateAccountRequest) (*model.Account, error) {
	account := &model.Account{
		Name:       req.Name,
		Type:       req.Type,
		Currency:   defaultString(req.Currency, "CNY"),
		Balance:    req.Balance,
		IsActive:   true,
		CreateTime: model.DateTimeStr(time.Now()),
		UpdateTime: model.DateTimeStr(time.Now()),
	}

	id, err := s.repo.Create(account)
	if err != nil {
		return nil, err
	}

	return s.repo.FindByID(id)
}

// UpdateAccount 更新账户，若余额变更则自动创建一笔"余额校正"交易
func (s *AccountService) UpdateAccount(id int64, req *dto.UpdateAccountRequest) (*model.Account, error) {
	// 无余额变更时走简单路径
	if req.Balance == nil {
		return s.updateFieldsOnly(id, req)
	}
	slog.Info("更新了账户余额，增加交易记录")
	// 有余额变更，需要在事务内创建校正交易 + 更新其他字段
	tx, err := s.repo.DB.Beginx()
	if err != nil {
		return nil, err
	}
	// 无条件回滚：Commit 成功后 Rollback 返回 sql.ErrTxDone 为空操作；
	// 中途失败 return 时自动回滚，不依赖 err 变量判断回滚（避免 if err := 遮蔽导致漏回滚）
	defer tx.Rollback()

	// 读取旧账户（事务内读取，防止并发问题）
	old, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	// 计算余额差值（Money 单位：分）
	diff := *req.Balance - old.Balance

	// 余额有变化时，创建校正交易并联动调整余额
	if diff != 0 {
		txType := "income"
		if diff < 0 {
			txType = "expense"
		}
		t := &model.Transaction{
			Type:        txType,
			Amount:      diff.Abs(),
			CategoryID:  -1, // 无分类
			AccountID:   id,
			ToAccountID: -1, // 非转账交易无目标账户，归一化为 -1“无账户”哨兵值
			Date:        time.Now().Format(time.DateOnly),
			Note:        new("余额校正"),
			CreateTime:  model.DateTimeStr(time.Now()),
			UpdateTime:  model.DateTimeStr(time.Now()),
		}
		if _, err := s.txnRepo.Create(tx, t); err != nil {
			return nil, err
		}
		// 联动调整账户余额
		if err := s.repo.AdjustBalance(tx, id, float64(diff)); err != nil {
			return nil, err
		}
	}

	// 更新其他字段（不包含 balance，已由交易处理）
	otherFields := buildOtherFields(req)
	if len(otherFields) > 0 {
		if err := s.repo.UpdateWithTx(tx, id, otherFields); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.repo.FindByID(id)
}

// updateFieldsOnly 无余额变更时的简单更新
func (s *AccountService) updateFieldsOnly(id int64, req *dto.UpdateAccountRequest) (*model.Account, error) {
	fields := buildOtherFields(req)
	if len(fields) == 0 {
		return s.repo.FindByID(id)
	}
	if err := s.repo.Update(id, fields); err != nil {
		return nil, err
	}
	return s.repo.FindByID(id)
}

// buildOtherFields 构建除 balance 外的更新字段
func buildOtherFields(req *dto.UpdateAccountRequest) map[string]interface{} {
	fields := make(map[string]interface{})
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Type != nil {
		fields["type"] = *req.Type
	}
	if req.Currency != nil {
		fields["currency"] = *req.Currency
	}
	if req.IsActive != nil {
		fields["is_active"] = *req.IsActive
	}
	return fields
}

// int64Ptr 返回 int64 指针
func int64Ptr(v int64) *int64 {
	return &v
}

// DeleteAccount 删除账户
func (s *AccountService) DeleteAccount(id int64) error {
	return s.repo.Delete(id)
}

// defaultString 如果值为空则返回默认值
func defaultString(val, defaultVal string) string {
	if val == "" {
		return defaultVal
	}
	return val
}
