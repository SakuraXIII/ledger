package repository

import (
	"fmt"
	"my-ledger/server/dto"
	"my-ledger/server/model"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// TransactionRepository 交易记录数据访问层
type TransactionRepository struct {
	*BaseRepository
}

// NewTransactionRepository 创建交易仓库实例
func NewTransactionRepository(db *sqlx.DB) *TransactionRepository {
	return &TransactionRepository{BaseRepository: NewBaseRepository(db)}
}

// FindByID 根据 ID 查询单条交易记录
func (r *TransactionRepository) FindByID(exec DBTX, id int64) (*model.Transaction, error) {
	var t model.Transaction
	err := exec.Get(&t, "SELECT * FROM transactions WHERE id = ?", id)
	if err != nil {
		return nil, WrapNotFoundError(err)
	}
	return &t, nil
}

// FindPage 分页查询交易记录，返回列表和总数
func (r *TransactionRepository) FindPage(exec DBTX, req *dto.QueryTransactionRequest) ([]model.Transaction, int64, error) {
	clause, args := buildTxnFilter(req)

	// 查询总数
	var total int64
	countSQL := "SELECT COUNT(*) FROM transactions" + clause
	if err := exec.Get(&total, countSQL, args...); err != nil {
		return nil, 0, err
	}

	// 查询分页数据
	var items = make([]model.Transaction, 0)
	dataSQL := "SELECT * FROM transactions" + clause + " ORDER BY date DESC, id DESC LIMIT ? OFFSET ?"
	offset := (req.PageNum - 1) * req.PageSize
	args = append(args, req.PageSize, offset)
	if err := exec.Select(&items, dataSQL, args...); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Create 新增交易记录，返回新记录 ID
func (r *TransactionRepository) Create(exec DBTX, t *model.Transaction) (int64, error) {
	result, err := exec.NamedExec(`
		INSERT INTO transactions (type, amount, category_id, account_id, to_account_id, date, note, create_time, update_time)
		VALUES (:type, :amount, :category_id, :account_id, :to_account_id, :date, :note, :create_time, :update_time)`,
		t,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Update 更新交易记录，只更新指定字段
func (r *TransactionRepository) Update(exec DBTX, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}

	query := "UPDATE transactions SET update_time = ?"
	args := []interface{}{time.Now()}
	for k, v := range fields {
		query += ", " + k + " = ?"
		args = append(args, v)
	}
	query += " WHERE id = ?"
	args = append(args, id)

	result, err := exec.Exec(query, args...)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// Delete 删除交易记录（物理删除）
func (r *TransactionRepository) Delete(exec DBTX, id int64) error {
	result, err := exec.Exec("DELETE FROM transactions WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// DeleteAll 清空交易记录表（物理删除）
func (r *TransactionRepository) DeleteAll(exec DBTX) error {
	_, err := exec.Exec("DELETE FROM transactions")
	if err != nil {
		return err
	}
	return nil
}

// 构建查询条件，返回 WHERE 子句和参数
func buildTxnFilter(req *dto.QueryTransactionRequest) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	if req.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, req.Type)
	}
	if req.CategoryID != nil {
		conditions = append(conditions, "category_id = ?")
		args = append(args, *req.CategoryID)
	}
	if req.AccountID != nil {
		conditions = append(conditions, "account_id = ?")
		args = append(args, *req.AccountID)
	}
	if req.StartDate != "" {
		conditions = append(conditions, "date >= ?")
		args = append(args, req.StartDate)
	}
	if req.EndDate != "" {
		conditions = append(conditions, "date <= ?")
		args = append(args, req.EndDate)
	}
	if req.Keyword != "" {
		conditions = append(conditions, "note LIKE ?")
		args = append(args, "%"+req.Keyword+"%")
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

// BuildTxnFilter 导出给 service 层使用（批量导入计数等场景）
func BuildTxnFilter(req *dto.QueryTransactionRequest) (string, []interface{}) {
	return buildTxnFilter(req)
}

// FindAll 查询全量交易记录（按日期倒序）
func (r *TransactionRepository) FindAll() ([]model.Transaction, error) {
	var items = make([]model.Transaction, 0)
	if err := r.DB.Select(&items, "SELECT * FROM transactions ORDER BY date DESC, id DESC"); err != nil {
		return nil, err
	}
	return items, nil
}

// FindByIDPlain 查询单条交易记录（不包装错误，用于存在性检查）
func (r *TransactionRepository) FindByIDPlain(exec DBTX, id int64) (*model.Transaction, error) {
	var t model.Transaction
	err := exec.Get(&t, "SELECT * FROM transactions WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("transaction %d not found: %w", id, err)
	}
	return &t, nil
}

// FindDailyTransactions 查询指定日期的所有交易记录（关联账户和分类名称）
func (r *TransactionRepository) FindDailyTransactions(exec DBTX, date string) ([]dto.DailyTransactionItem, error) {
	var items = make([]dto.DailyTransactionItem, 0)
	query := `SELECT t.*,
			c.name AS category_name,
			a1.name AS account_name,
			a2.name AS to_account_name
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		LEFT JOIN accounts a1 ON t.account_id = a1.id
		LEFT JOIN accounts a2 ON t.to_account_id = a2.id
		WHERE t.date = ?
		ORDER BY t.create_time DESC`
	if err := exec.Select(&items, query, date); err != nil {
		return nil, err
	}
	return items, nil
}

// GetDailySummary 按天聚合统计收入、支出、结余
func (r *TransactionRepository) GetDailySummary(exec DBTX, startDate, endDate string) ([]dto.DailySummaryItem, error) {
	var items = make([]dto.DailySummaryItem, 0)
	query := `SELECT date,
			COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0) AS income,
			COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0) AS expense,
			COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0)
			- COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0) AS balance
		FROM transactions
		WHERE date >= ? AND date <= ?
		GROUP BY date
		ORDER BY date DESC`
	if err := exec.Select(&items, query, startDate, endDate); err != nil {
		return nil, err
	}
	return items, nil
}
