// Package service 提供业务逻辑层
package service

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"my-ledger/server/dto"
	"my-ledger/server/model"
	"my-ledger/server/repository"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CSV 导入每批处理的最大记录数
const importBatchSize = 200

// categoryKey 分类映射键：分类允许同名，靠 (引用名, 类型) 区分
type categoryKey struct {
	name string // CSV 引用名（全名或短名）
	typ  string // expense/income/transfer
}

// categoryRegistry 导入过程中的分类注册表，与导入事务同生命周期（跨批共享），用一个 struct 统一传递避免签名膨胀。
// 分类身份 = (name, type)：type 取自 parseType(类型列)，同名不同 type 是不同分类，
// 父分类 type 跟随创建它的子分类；DB 不加唯一索引，导入期间的去重与引用解析全由本注册表承担。
// 两个键空间：
//   - nameToID：引用解析空间，供 parseCSVRecord 解析与 ensureBatchNames 请求键存在性查表。
//     混合三类来源：短名占位（registerLeafKey）、请求键（registerRequested）、全名引用（registerFullNameKey）。
//     请求键的位由三步确立：markRequested 踢他支占位 → ensureBatchNames 优先按 fullNameToID 精确复用、
//     查不到才建行 → registerRequested 登记。因此墓碑拦不下"真实全名恰为请求名"的显式请求
//   - fullNameToID：真实全名空间，只存"节点真实全名 → ID"，供父链查表（createCategory 查父）
//     与请求键真实全名复用（ensureBatchNames）专用。保证命中的节点真实全名恰为查询名，
//     不会被他支链叶子的短名占位骗到（如 A-B 的叶子占位 (B) 不等于顶级节点 B）
//
// 另有：
//   - shortNameConflicts：短名冲突墓碑，粘滞作废歧义的副作用短名别名（registerLeafKey 不再注册该键）；
//     不拦截显式请求键——有真实全名节点即复用，无则解析阶段显式报"分类不存在"
//   - requested：CSV 显式请求键声明（收集阶段标记，副作用短名让位与裸名占位踢除的判断依据）
//
// 键空间前提（改动切分/注册逻辑时必须维护）：
//   - 链叶子短名键不含 "-"（createCategory 按 LastIndex 切分，child 段必不含 "-"）
//   - 顶级整名短名键含 "-" 时必为首段或末段为空的形态（如 "-B"/"A-"，因两段非空即被切分）
//   - 多段全名键首段与末段均非空，与上述两类短名键不相交
//     因此短名冲突判定只发生在短名键内部，不会误伤全名引用
type categoryRegistry struct {
	nameToID           map[categoryKey]int64
	fullNameToID       map[categoryKey]int64    // 真实全名空间：只存节点真实全名，父链查表专用
	shortNameConflicts map[categoryKey]struct{} // 冲突墓碑：短名键一旦冲突即粘滞失效
	requested          map[categoryKey]struct{} // 请求键声明：CSV 中显式出现过的引用键（收集阶段标记）
}

// 创建分类注册表
func newCategoryRegistry() *categoryRegistry {
	return &categoryRegistry{
		nameToID:           make(map[categoryKey]int64),
		fullNameToID:       make(map[categoryKey]int64),
		shortNameConflicts: make(map[categoryKey]struct{}),
		requested:          make(map[categoryKey]struct{}),
	}
}

// lookup 引用空间查表：供 parseCSVRecord 解析与 ensureBatchNames 请求键存在性检查使用
func (r *categoryRegistry) lookup(key categoryKey) (int64, bool) {
	id, ok := r.nameToID[key]
	return id, ok
}

// lookupFullName 父链查表：只接受真实全名恰为 key.name 的节点，
// 短名占位（他支链叶子）不会命中，避免父链静默挂错导致导出全名撒谎
func (r *categoryRegistry) lookupFullName(key categoryKey) (int64, bool) {
	id, ok := r.fullNameToID[key]
	return id, ok
}

// isConflicted 查询短名键是否已因冲突永久失效（墓碑只作废副作用短名别名，不拦显式请求键）
func (r *categoryRegistry) isConflicted(key categoryKey) bool {
	_, ok := r.shortNameConflicts[key]
	return ok
}

// markRequested 声明请求键（收集阶段、预扫描创建前全量调用）：
// 裸名请求若已被他支链叶子副作用占位，踢除该占位把位置还给显式请求，
// 防止"顶级行未建却查到他支分类"的静默错挂；
// 请求名恰为某节点真实全名（如 N-M 的父 N）时不踢，占位本就指向该节点，交由 ensureBatchNames 按 fullNameToID 精确复用；
// 含 "-" 的请求键其同串占位来自同链路径节点（真实全名相同），同样保留引用避免重复建行
func (r *categoryRegistry) markRequested(key categoryKey) {
	if _, ok := r.requested[key]; ok {
		return
	}
	r.requested[key] = struct{}{}
	if strings.Contains(key.name, "-") {
		return
	}
	// 已有真实全名恰为请求名的节点：不踢，ensureBatchNames 会命中 fullNameToID 复用它
	if _, ok := r.fullNameToID[key]; ok {
		return
	}
	// 踢除不打墓碑：后续由 ensureBatchNames 为该请求键建行并 registerRequested 填位；
	// 副作用侧由 requested 检查阻止复活，无需墓碑
	delete(r.nameToID, key)
}

// registerRequested 登记请求键的本体映射：预扫描为请求键创建的行以它为准，
// 保证请求键在引用空间必然可查（重复声明由 requested 集合去重，无需归属标记）
func (r *categoryRegistry) registerRequested(key categoryKey, id int64) {
	r.nameToID[key] = id
}

// registerLeafKey 注册副作用短名别名（链叶子/顶级整名的短名占位），冲突感知：
// 已在冲突集合中则不注册（墓碑粘滞）；是请求键则让位不注册（请求键解析由 fullNameToID 复用与
// registerRequested 保证，不依赖本通道，也不删请求键映射、不抢占空位）；
// 已被另一 ID 占有则删除别名并加入冲突集合；否则正常注册。
// 本通道写入 nameToID 的别名只服务两件事：异名同短名的冲突判定（产生墓碑）与 markRequested 的踢位对象，
// 不决定任何请求键的解析结果
func (r *categoryRegistry) registerLeafKey(key categoryKey, id int64) {
	if r.isConflicted(key) {
		return
	}
	if _, ok := r.requested[key]; ok {
		return
	}
	if existing, ok := r.nameToID[key]; ok && existing != id {
		delete(r.nameToID, key)
		r.shortNameConflicts[key] = struct{}{}
		return
	}
	r.nameToID[key] = id
}

// registerFullNameKey 在引用空间登记全名引用（供 parseCSVRecord 解析多段全名请求），
// 不受冲突集合与请求键让位影响，始终注册
func (r *categoryRegistry) registerFullNameKey(key categoryKey, id int64) {
	r.nameToID[key] = id
}

// registerRealFullName 在真实全名空间登记节点全名（父链查表专用），始终注册；
// 同一真实全名若出现重复节点（如被踢除占位后重建顶级），以最后建的为准（顺序由排序保证，行为确定）
func (r *categoryRegistry) registerRealFullName(key categoryKey, id int64) {
	r.fullNameToID[key] = id
}

// BackupService 备份导入导出业务逻辑层
type BackupService struct {
	txnRepo *repository.TransactionRepository
	accRepo *repository.AccountRepository
	catRepo *repository.CategoryRepository
}

// NewBackupService 创建备份服务实例
func NewBackupService(
	txnRepo *repository.TransactionRepository,
	accRepo *repository.AccountRepository,
	catRepo *repository.CategoryRepository,
) *BackupService {
	return &BackupService{
		txnRepo: txnRepo,
		accRepo: accRepo,
		catRepo: catRepo,
	}
}

// ExportCSV 导出全量交易记录为 CSV 文件内容
func (s *BackupService) ExportCSV() ([]byte, error) {
	transactions, err := s.txnRepo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("查询交易记录失败: %w", err)
	}

	// 预加载账户和分类映射，避免逐行查库
	accountMap, err := s.buildAccountMap()
	if err != nil {
		return nil, fmt.Errorf("加载账户数据失败: %w", err)
	}
	categoryMap, err := s.buildCategoryNameMap()
	if err != nil {
		return nil, fmt.Errorf("加载分类数据失败: %w", err)
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// 写入表头
	header := []string{"类型", "金额", "分类", "转出账户", "转入账户", "日期", "备注"}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("写入 CSV 表头失败: %w", err)
	}

	// 写入数据行
	for _, t := range transactions {
		// 金额：支出加负号
		amount := t.Amount
		if t.Type == "expense" {
			amount = -amount
		}

		// 分类名称（处理 nil）
		categoryName := ""
		if t.CategoryID != -1 {
			categoryName = categoryMap[t.CategoryID]
		}

		// 转入账户名称（处理 nil）
		toAccountName := ""
		if t.ToAccountID != -1 {
			toAccountName = accountMap[t.ToAccountID]
		}
		before, _, _ := strings.Cut(t.Date, "T")
		record := []string{
			translateType(t.Type),
			fmt.Sprintf("%f", amount.ToYuan()),
			categoryName,
			accountMap[t.AccountID],
			toAccountName,
			before,
			formatStringPtr(t.Note),
		}
		if err := writer.Write(record); err != nil {
			return nil, fmt.Errorf("写入 CSV 数据失败: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("刷新 CSV 缓冲区失败: %w", err)
	}

	return buf.Bytes(), nil
}

// 构建账户 ID -> 名称 映射
func (s *BackupService) buildAccountMap() (map[int64]string, error) {
	// 查询所有账户（包含已禁用的）
	var accounts []model.Account
	if err := s.txnRepo.DB.Select(&accounts, "SELECT id, name FROM accounts"); err != nil {
		return nil, err
	}

	m := make(map[int64]string, len(accounts))
	for _, a := range accounts {
		m[a.ID] = a.Name
	}
	return m, nil
}

// 构建分类 ID -> 完整名称 映射（子类拼接为 "父类-子类"）
func (s *BackupService) buildCategoryNameMap() (map[int64]string, error) {
	// 查询所有分类
	var categories []model.Category
	if err := s.txnRepo.DB.Select(&categories, "SELECT id, name, parent_id FROM categories"); err != nil {
		return nil, err
	}

	// 构建 ID -> 分类 映射
	catByID := make(map[int64]*model.Category, len(categories))
	for i := range categories {
		catByID[categories[i].ID] = &categories[i]
	}

	// 递归拼接分类名
	getFullName := func(cat *model.Category) string {
		name := cat.Name
		parent := cat.ParentID
		for parent != nil {
			if p, ok := catByID[*parent]; ok {
				name = p.Name + "-" + name
				parent = p.ParentID
			} else {
				break
			}
		}
		return name
	}

	m := make(map[int64]string, len(categories))
	for i := range categories {
		m[categories[i].ID] = getFullName(&categories[i])
	}
	return m, nil
}

// 将交易类型翻译为中文
func translateType(t string) string {
	switch t {
	case "expense":
		return "支出"
	case "income":
		return "收入"
	case "transfer":
		return "转账"
	default:
		return t
	}
}

// ImportCSV 从 CSV 流式导入交易记录（边读边批处理，每批最多 importBatchSize 条，文件不再整体读入内存）
// 编排流程：BOM → 表头 → 事务 → 映射 → 分批回调 → 提交；分批读取委托给 readAndProcessBatches，
// 每攒满一批或处理尾批时调用 onBatch 回调（当批预扫描 + 处理本批）
// 错误路径显式 return nil, err，不返回半截 result；
// 事务为无条件回滚，任一错误路径中途 return 时自动回滚，避免自动创建账户/分类失败时遗漏回滚
func (s *BackupService) ImportCSV(r io.Reader) (*dto.BatchImportResult, error) {
	// 跳过流开头可能存在的 UTF-8 BOM（只在流开头跳过一次，不影响后续数据）
	br := bufio.NewReader(r)
	if bom, _ := br.Peek(3); len(bom) == 3 && bom[0] == 0xEF && bom[1] == 0xBB && bom[2] == 0xBF {
		_, _ = br.Discard(3)
	}
	reader := csv.NewReader(br)

	// 读取并验证表头
	header, err := reader.Read()
	if err != nil {
		return nil, &repository.AppError{Code: 40001, Message: "读取 CSV 表头失败"}
	}
	if err := validateCSVHeader(header); err != nil {
		return nil, err
	}

	result := &dto.BatchImportResult{}

	// 开启事务（整个导入一个事务：账户/分类创建、交易导入、余额更新同事务提交或回滚）
	tx, err := s.txnRepo.DB.Beginx()
	if err != nil {
		return nil, err
	}
	// 无条件回滚：Commit 成功后 Rollback 返回 sql.ErrTxDone 为空操作；
	// 中途失败 return 时自动回滚，与 transaction_svc 保持一致
	defer tx.Rollback()

	// 导入视为全量导入，因此先清空整个数据库，避免已有数据冲突
	if err = s.clearDatabase(tx); err != nil {
		return nil, err
	}

	// 记录导入中需要创建的账户和类别（分类按 (引用名, 类型) 去重，含冲突墓碑跨批共享）
	accountNameToID := make(map[string]int64)
	categoryReg := newCategoryRegistry()

	// 处理一批：当批预扫描 → 事务内创建缺失账户/分类 → 处理本批
	// 作为 readAndProcessBatches 的回调，在攒满一批或尾批时被调用；
	// 批次释放由 readAndProcessBatches 在回调成功返回后统一处理
	onBatch := func(batch [][]string, lines []int) error {
		if nameErr := s.ensureBatchNames(tx, header, batch, accountNameToID, categoryReg); nameErr != nil {
			return nameErr
		}
		return s.processImportBatch(tx, header, batch, lines, accountNameToID, categoryReg, result)
	}

	// 流式读取 + 攒批 + 尾批触发；任一错误直接 return nil, err（不吞错、不返回半截 result），
	// 由 defer 无条件回滚事务
	if err := readAndProcessBatches(reader, result, onBatch); err != nil {
		return nil, err
	}
	// 提交事务
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// 清空数据库
func (s *BackupService) clearDatabase(exec repository.DBTX) error {
	if err := s.txnRepo.DeleteAll(exec); err != nil {
		slog.Error("清空交易表失败", "err", err)
		return err
	}
	if err := s.catRepo.DeleteAll(exec); err != nil {
		slog.Error("清空分类表失败", "err", err)
		return err
	}
	if err := s.accRepo.DeleteAll(exec); err != nil {
		slog.Error("清空账户表失败", "err", err)
		return err
	}
	return nil
}

// 流式读取 CSV 数据行并按批回调：攒满 importBatchSize 条调用一次 onBatch，
// 循环结束后补处理不足一批的尾批（尾批回调的错误同样直接返回）
// 错误处理分三支：EOF 正常结束；解析错误（*csv.ParseError）收集进 result.Errors 后跳过继续（方案A）；
// 底层 IO 错误与回调错误立即返回（defer 无条件回滚事务）——IO 错误必须立即失败，
// 否则 bufio 缓存会反复重现同一错误导致死循环（详见循环内注释）
// 行号规则：表头为第 1 行，数据行从 2 开始递增；空行跳过但行号照常推进，
// batchLines 与 batch 平行记录每条记录的真实行号
func readAndProcessBatches(
	reader *csv.Reader,
	result *dto.BatchImportResult,
	onBatch func(batch [][]string, lines []int) error,
) error {
	// 流式循环：读满 importBatchSize 条 → 处理一批 → 继续读取
	line := 1 // 当前 CSV 行号（表头为第 1 行）
	batch := make([][]string, 0, importBatchSize)
	batchLines := make([]int, 0, importBatchSize) // 与 batch 平行，记录每行在 CSV 中的真实行号（跳过错误行后仍对应）

	for {
		record, readErr := reader.Read()
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			// 区分错误来源，避免 bufio 缓存底层错误导致死循环：
			// - 解析类错误（*csv.ParseError）：由 csv 解析器自身产生，不缓存，
			//   下一次 Read 必然前进到后续数据，收集后跳过继续（方案A）
			// - 底层 IO 错误（非 ParseError，如临时文件失效、磁盘错误）：
			//   bufio.Reader 在 buffer 耗尽后每次都重现同一错误，
			//   若 continue 会无限循环（line/Errors 无限增长、事务永不结束、
			//   长期占住数据库连接、阻塞后续请求），必须整体失败回滚
			if _, ok := errors.AsType[*csv.ParseError](readErr); ok {
				line++
				result.Errors = append(result.Errors, dto.BatchItemError{
					Index:   line,
					Message: "读取 CSV 数据失败: " + readErr.Error(),
				})
				continue
			}
			// 底层 IO 错误：立即返回，调用方 return 时由 defer 无条件回滚事务
			return fmt.Errorf("读取 CSV 数据失败: %w", readErr)
		}
		line++

		// 跳过空行：不计入批次、不产生 error
		if len(record) == 0 || (len(record) == 1 && record[0] == "") {
			continue
		}

		batch = append(batch, record)
		batchLines = append(batchLines, line)
		if len(batch) >= importBatchSize {
			if err := onBatch(batch, batchLines); err != nil {
				return err
			}
			// 释放批次：复用底层数组，避免下一批重新分配
			batch = batch[:0]
			batchLines = batchLines[:0]
		}
	}

	// 处理尾批（不足 importBatchSize 的剩余记录）
	if len(batch) > 0 {
		return onBatch(batch, batchLines)
	}
	return nil
}

// 处理一批 CSV 记录（最多 importBatchSize 条）
// lines 与 records 平行，记录每行在 CSV 中的真实行号（reader 跳过错误行后行号不再连续）
// 解析错误收集到 result.Errors 后跳过继续；DB 错误立即返回非 nil error，触发事务回滚
func (s *BackupService) processImportBatch(
	tx repository.DBTX,
	header []string,
	records [][]string,
	lines []int,
	accountNameToID map[string]int64,
	categoryReg *categoryRegistry,
	result *dto.BatchImportResult,
) error {
	for i, record := range records {
		// 跳过空行
		if len(record) == 0 || (len(record) == 1 && record[0] == "") {
			continue
		}

		// CSV 中的实际行号（表头为第 1 行）
		line := lines[i]

		// 解析行数据：解析错误收集到错误列表，跳过该行继续处理
		txn, parseErr := s.parseCSVRecord(header, record, accountNameToID, categoryReg)
		if parseErr != nil {
			result.Errors = append(result.Errors, dto.BatchItemError{
				Index:   line,
				Message: parseErr.Error(),
			})
			continue
		}

		// 业务校验：日期格式与转账规则，失败收集到错误列表并跳过该行继续
		// 账户/分类存在性无需校验，当批预扫描阶段已保证其在事务中创建
		if validateErr := validateImportedTransaction(txn); validateErr != nil {
			result.Errors = append(result.Errors, dto.BatchItemError{
				Index:   line,
				Message: batchErrorMessage(validateErr),
			})
			continue
		}

		// 插入记录
		id, dbErr := s.txnRepo.Create(tx, txn)
		if dbErr != nil {
			slog.Error("CSV 导入 DB 错误", "index", line, "error", dbErr)
			return repository.ErrInternal
		}
		txn.ID = id

		// 应用余额变更
		if balanceErr := s.applyBalanceOps(tx, txn); balanceErr != nil {
			slog.Error("CSV 导入余额更新错误", "index", line, "error", balanceErr)
			return repository.ErrInternal
		}

		result.Imported++
	}
	return nil
}

// 当批预扫描：收集本批记录用到的账户名/分类名，缺失的在事务内自动创建
// 映射与冲突墓碑跨批共享（只增不减），已存在的名称不会重复创建
func (s *BackupService) ensureBatchNames(
	tx repository.DBTX,
	header []string,
	records [][]string,
	accountNameToID map[string]int64,
	categoryReg *categoryRegistry,
) error {
	accountNames, categoryNames := s.collectNamesFromRecords(header, records)

	// 创建缺失账户
	for _, name := range accountNames {
		if _, ok := accountNameToID[name]; !ok {
			id, err := s.createAccount(tx, name)
			if err != nil {
				return fmt.Errorf("创建账户 %q 失败: %w", name, err)
			}
			accountNameToID[name] = id
			slog.Info("自动创建账户", "name", name, "id", id)
		}
	}

	// 先全量声明本批请求键，再创建：避免链叶子副作用短名抢占同批稍后出现的裸名请求键
	for _, key := range categoryNames {
		categoryReg.markRequested(key)
	}

	// 分类按 name 再 typ 排序，消除 map 迭代随机对创建顺序（含短名冲突判定）的影响
	sort.Slice(categoryNames, func(i, j int) bool {
		if categoryNames[i].name != categoryNames[j].name {
			return categoryNames[i].name < categoryNames[j].name
		}
		return categoryNames[i].typ < categoryNames[j].typ
	})

	// 创建缺失分类
	for _, key := range categoryNames {
		// 请求键优先按真实全名空间精确复用（越过墓碑）：
		// 命中"真实全名恰为请求名"的节点（如批1 X-X 立墓碑后，顶级 X 仍登记在 fullNameToID），
		// 登记引用空间后直接复用，不新建重复行、不丢合法行
		if id, ok := categoryReg.lookupFullName(key); ok {
			categoryReg.registerRequested(key, id)
			continue
		}
		// 无真实全名节点可复用，且短名键已因冲突粘滞失效：不创建，解析阶段显式报"分类不存在"
		if categoryReg.isConflicted(key) {
			continue
		}
		if _, ok := categoryReg.lookup(key); !ok {
			// 请求键本体映射在此登记：保证预扫描为请求键创建的行，解析时必然可查
			id, err := s.createCategory(tx, key.name, key.typ, categoryReg)
			if err != nil {
				return fmt.Errorf("创建分类 %q(type=%s) 失败: %w", key.name, key.typ, err)
			}
			categoryReg.registerRequested(key, id)
			slog.Info("自动创建分类", "name", key.name, "type", key.typ, "id", id)
		}
	}
	return nil
}

// 扫描一批 CSV 记录，收集本批用到的账户名和分类名
// 分类按 (引用名, 类型) 收集：类型取 parseType("类型") 的原始结果，
// 类型解析失败的行跳过分类收集（该行后续在解析阶段报类型错误进 Errors）
func (s *BackupService) collectNamesFromRecords(header []string, records [][]string) (accountNames []string, categoryNames []categoryKey) {
	colIndex := make(map[string]int)
	for i, h := range header {
		colIndex[h] = i
	}

	getField := func(record []string, name string) string {
		if idx, ok := colIndex[name]; ok && idx < len(record) {
			return record[idx]
		}
		return ""
	}

	// 在 Go 语言中，没有直接提供 set 这样的集合类型。但是，Map 的键是唯一的，不能重复，这正好符合集合的特性。
	// struct{} 是一个空结构体，不包含任何字段，不占用任何内存空间（大小为 0 字节）
	accSet := make(map[string]struct{})
	catSet := make(map[categoryKey]struct{})

	for _, record := range records {
		if len(record) == 0 || (len(record) == 1 && record[0] == "") {
			continue
		}

		if v := getField(record, "转出账户"); v != "" {
			accSet[v] = struct{}{}
		}
		if v := getField(record, "转入账户"); v != "" {
			accSet[v] = struct{}{}
		}
		if v := getField(record, "分类"); v != "" {
			// 类型为空的行不收集分类，交给解析阶段报类型错误
			tType := parseType(getField(record, "类型"))
			if tType == "" {
				continue
			}
			catSet[categoryKey{name: v, typ: tType}] = struct{}{}
		}
	}

	for name := range accSet {
		accountNames = append(accountNames, name)
	}
	for key := range catSet {
		categoryNames = append(categoryNames, key)
	}
	return
}

// 创建账户（默认类型 e-pay，币种 CNY）
func (s *BackupService) createAccount(tx repository.DBTX, name string) (int64, error) {
	account := &model.Account{
		Name:       name,
		Type:       "e-pay",
		Currency:   "CNY",
		IsActive:   true,
		CreateTime: model.DateTimeStr(time.Now()),
		UpdateTime: model.DateTimeStr(time.Now()),
	}
	result, err := tx.NamedExec(`
		INSERT INTO accounts (name, type, currency, balance, is_active, create_time, update_time)
		VALUES (:name, :type, :currency, :balance, :is_active, :create_time, :update_time)`,
		account,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// 创建分类（支持 "父类-子类-孙类" 多级格式，自动创建缺失的父分类）
// 从最后一个 "-" 切分：父为完整父链（如 "餐饮-外卖"），子为末级名（如 "午餐"）
// 父分类的全名显式传给 insertCategory 注册映射，而非反查 map（map 迭代随机会丢失组合名）
// typ 决定写入分类表的 type 列，父分类 type 跟随创建它时那条子分类的 type
// 切分出的任一段为空（前导/尾随短横，如 "-B"/"A-"/"-"）时不按父子切分，
// 整名作为顶级分类创建：请求键以整名注册，且不落库空名垃圾行、不污染末段短名键
func (s *BackupService) createCategory(tx repository.DBTX, fullName string, typ string, categoryReg *categoryRegistry) (int64, error) {
	idx := strings.LastIndex(fullName, "-")
	// idx<=0：无短横或前导短横（parent 段为空）；idx==len-1：尾随短横（child 段为空）
	if idx <= 0 || idx == len(fullName)-1 {
		// 顶级分类（含 "-B"/"A-"/"-" 等整名形态）
		return s.insertCategory(tx, fullName, typ, nil, "", categoryReg)
	}

	// 子分类：先确保父分类（完整父链）存在
	parentFullName := fullName[:idx]
	childName := fullName[idx+1:]

	// 父链查表只走真实全名空间：只有"真实全名恰为 parentFullName"的节点才复用，
	// 他支链叶子的短名占位（如 A-B 的叶子 (B)）不会被误当顶级 B 复用；查不到则按全名递归新建
	parentID, ok := categoryReg.lookupFullName(categoryKey{name: parentFullName, typ: typ})
	if !ok {
		// 父分类也不存在，递归创建（同一 type）
		// 父键的映射由递归内 insertCategory 按冲突规则注册，此处不直接赋值，
		// 避免绕过冲突墓碑把已失效的短名键复活
		var err error
		parentID, err = s.createCategory(tx, parentFullName, typ, categoryReg)
		if err != nil {
			return 0, fmt.Errorf("创建父分类 %q 失败: %w", parentFullName, err)
		}
	}

	// 显式传入父分类的完整名称，确定性注册 "父-子" 全名映射
	return s.insertCategory(tx, childName, typ, &parentID, parentFullName, categoryReg)
}

// 插入分类记录
// parentFullName 为父分类在 CSV 中的引用名（可为空表示顶级分类），用于确定性地注册 "父-子" 全名映射
// 短名键按副作用注册（registerLeafKey）：请求键让位、冲突墓碑粘滞失效，
// 后续同名短名查询显式报"分类不存在"而不是静默挂错
func (s *BackupService) insertCategory(tx repository.DBTX, name string, typ string, parentID *int64, parentFullName string, categoryReg *categoryRegistry) (int64, error) {
	// 空名守卫：空名行无引用价值且污染短名键空间，任何路径都不允许落库
	if name == "" {
		return 0, fmt.Errorf("分类名不能为空")
	}
	result, err := tx.Exec(`
		INSERT INTO categories (name, type, parent_id, is_active, create_time, update_time)
		VALUES (?, ?, ?, ?, ?, ?)`,
		name, typ, parentID, 1, model.DateTimeStr(time.Now()), model.DateTimeStr(time.Now()),
	)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// 登记节点真实全名（父链查表空间）：顶级=自身名，子类=父全名-名；
	// 父已按真实全名查得/递归创建，故 parentFullName+"-"+name 即该节点真实全名
	realFullName := name
	if parentID != nil && parentFullName != "" {
		realFullName = parentFullName + "-" + name
	}
	categoryReg.registerRealFullName(categoryKey{name: realFullName, typ: typ}, id)

	// 短名占位按副作用冲突感知注册（请求键让位/墓碑粘滞）；
	// 子类另在引用空间登记全名引用，供 parseCSVRecord 解析多段全名请求
	categoryReg.registerLeafKey(categoryKey{name: name, typ: typ}, id)
	if parentID != nil && parentFullName != "" {
		categoryReg.registerFullNameKey(categoryKey{name: realFullName, typ: typ}, id)
	}

	return id, nil
}

// 校验 CSV 表头格式
func validateCSVHeader(header []string) error {
	expected := []string{"类型", "金额", "分类", "转出账户", "转入账户", "日期", "备注"}
	if len(header) != len(expected) {
		return &repository.AppError{Code: 40001, Message: fmt.Sprintf("CSV 表头列数不正确，期望 %d 列，实际 %d 列", len(expected), len(header))}
	}
	for i, h := range expected {
		if header[i] != h {
			return &repository.AppError{Code: 40001, Message: fmt.Sprintf("CSV 表头第 %d 列应为 %q，实际为 %q", i+1, h, header[i])}
		}
	}
	return nil
}

// 解析 CSV 行记录为交易对象（需传入账户和分类注册表）
func (s *BackupService) parseCSVRecord(header []string, record []string, accountNameToID map[string]int64, categoryReg *categoryRegistry) (*model.Transaction, error) {
	// 建立列名到索引的映射
	colIndex := make(map[string]int)
	for i, h := range header {
		colIndex[h] = i
	}

	getField := func(name string) string {
		if idx, ok := colIndex[name]; ok && idx < len(record) {
			return record[idx]
		}
		return ""
	}

	// 解析类型（支持中文）；tType 保留 parseType 原始结果用于分类查表，
	// 负数金额只覆盖最终交易类型 txnType，不影响分类 type
	tType := parseType(getField("类型"))
	if tType == "" {
		return nil, fmt.Errorf("类型字段不能为空")
	}
	txnType := tType

	// 解析金额（支持负数表示支出）
	amountStr := getField("金额")
	if amountStr == "" {
		return nil, fmt.Errorf("金额字段不能为空")
	}
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return nil, fmt.Errorf("金额字段解析失败: %s", amountStr)
	}
	// 负数转正数，类型设为支出
	if amount < 0 {
		amount = -amount
		txnType = "expense"
	}
	if amount <= 0 {
		return nil, fmt.Errorf("金额必须大于 0")
	}

	// 解析日期
	date := getField("日期")
	if date == "" {
		return nil, fmt.Errorf("日期字段不能为空")
	}

	// 解析分类（可选，按名称查找 ID；空字段记 -1“无分类”哨兵值，与账户口径一致，
	// 用每行局部变量取址避免共享）
	var categoryID int64 = -1
	categoryName := getField("分类")
	if categoryName != "" {
		cid, ok := categoryReg.lookup(categoryKey{name: categoryName, typ: tType})
		if !ok {
			return nil, fmt.Errorf("分类 %q(type=%s) 不存在", categoryName, tType)
		}
		categoryID = cid
	}

	// 解析转出账户（按名称查找 ID；空字段表示该交易无此账户，记 -1“无账户”哨兵值）
	accountName := getField("转出账户")
	var accountID int64 = -1
	if accountName != "" {
		aid, ok := accountNameToID[accountName]
		if !ok {
			return nil, fmt.Errorf("转出账户 %q 不存在", accountName)
		}
		accountID = aid
	}

	// 解析转入账户（可选；空字段同样记 -1“无账户”哨兵值，用每行局部变量取址避免共享）
	var toAccountID int64 = -1
	toAccountName := getField("转入账户")
	if toAccountName != "" {
		tid, ok := accountNameToID[toAccountName]
		if !ok {
			return nil, fmt.Errorf("转入账户 %q 不存在", toAccountName)
		}
		toAccountID = tid
	}

	// 解析备注（可选）
	var note *string
	noteStr := getField("备注")
	if noteStr != "" {
		note = &noteStr
	}

	return &model.Transaction{
		Type:        txnType,
		Amount:      model.NewMoneyYuan(amount),
		CategoryID:  categoryID,
		AccountID:   accountID,
		ToAccountID: toAccountID,
		Date:        date,
		Note:        note,
		CreateTime:  model.DateTimeStr(time.Now()),
		UpdateTime:  model.DateTimeStr(time.Now()),
	}, nil
}

// 解析交易类型（支持中文）
func parseType(s string) string {
	switch s {
	case "支出", "expense":
		return "expense"
	case "收入", "income":
		return "income"
	case "转账", "transfer":
		return "transfer"
	default:
		return ""
	}
}

// 校验导入交易的格式与转账规则（不依赖 DB 查询，账户/分类存在性由导入预扫描保证，此处只解析并规范化日期、校验转账业务规则）
func validateImportedTransaction(t *model.Transaction) error {
	// 校验并规范化日期，写回 t.Date（校验通过后同一 txn 直接入库，入库值即规范化日期）
	normalized, err := parseDate(t.Date)
	if err != nil {
		return &repository.AppError{Code: 40002, Message: "日期格式无效"}
	}
	t.Date = normalized

	// 转账特殊校验（-1 为“无账户”哨兵值：任一端为 -1 即非真实账户，不构成同账户冲突，故同为 -1 或一端为 -1 均放行）
	if t.Type == "transfer" {
		// 仅来源、目标均为真实账户且相同时才拦截
		if t.AccountID != -1 && t.ToAccountID == t.AccountID {
			return &repository.AppError{Code: 40001, Message: "转账来源和目标不能相同"}
		}
	} else {
		// 非转账类型不能有目标账户（-1 表示未指定目标账户，不算违规，放行）
		if t.ToAccountID != -1 {
			return &repository.AppError{Code: 40001, Message: "非转账交易不能指定目标账户"}
		}
	}

	return nil
}

// 提取批量导入单条错误信息（AppError 只取 Message，避免 code 前缀干扰用户）
func batchErrorMessage(err error) string {
	if appErr, ok := errors.AsType[*repository.AppError](err); ok {
		return appErr.Message
	}
	return err.Error()
}

// 应用余额变更
func (s *BackupService) applyBalanceOps(tx repository.DBTX, t *model.Transaction) error {
	ops := calcBalanceOps(t, 1)
	for _, op := range ops {
		if err := s.accRepo.AdjustBalance(tx, op.accountID, op.delta); err != nil {
			return fmt.Errorf("更新账户 %d 余额失败: %w", op.accountID, err)
		}
	}
	return nil
}

// 格式化可空字符串
func formatStringPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
