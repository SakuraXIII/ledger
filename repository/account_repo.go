package repository

import (
	"my-ledger/server/dto"
	"my-ledger/server/model"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// AccountRepository 账户数据访问层
type AccountRepository struct {
	*BaseRepository
}

// NewAccountRepository 创建账户仓库实例
func NewAccountRepository(db *sqlx.DB) *AccountRepository {
	return &AccountRepository{BaseRepository: NewBaseRepository(db)}
}

// FindAll 查询账户列表，支持按名称（模糊）/类型/币种筛选 + 分页
func (r *AccountRepository) FindAll(req *dto.QueryAccountRequest) ([]model.Account, error) {
	var accounts = make([]model.Account, 0)
	var querySql = "SELECT * FROM accounts"
	var args []any
	var conditions []string

	if req.Name != "" {
		conditions = append(conditions, "name LIKE ?")
		args = append(args, "%"+req.Name+"%")
	}
	if req.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, req.Type)
	}
	if req.Currency != "" {
		conditions = append(conditions, "currency = ?")
		args = append(args, req.Currency)
	}

	// 页码从 1 开始，OFFSET 需换算为 (pageNum-1)*pageSize，直接用页码当 OFFSET 会跳过前面的记录
	offset := (req.PageNum - 1) * req.PageSize
	args = append(args, req.PageSize, offset)
	if conditions != nil {
		querySql += " WHERE " + strings.Join(conditions, " AND ")
	}
	// create_time 相同（秒级精度）时以 id 倒序兜底，保证分页排序稳定不重不漏
	querySql += " ORDER BY create_time DESC, id DESC LIMIT ? OFFSET ?"

	err := r.DB.Select(&accounts, querySql, args...)
	return accounts, err
}

// FindByID 根据 ID 查询单个账户
func (r *AccountRepository) FindByID(id int64) (*model.Account, error) {
	var account model.Account
	err := r.DB.Get(&account, "SELECT * FROM accounts WHERE id = ?", id)
	if err != nil {
		return nil, WrapNotFoundError(err)
	}
	return &account, nil
}

// Create 新增账户，返回新账户 ID
// description 可为 nil（CreateAccount 请求未赋值时），NamedExec 绑定 nil 写入 NULL
func (r *AccountRepository) Create(account *model.Account) (int64, error) {
	result, err := r.DB.NamedExec(`
		INSERT INTO accounts (name, type, currency, balance, is_active, description, create_time, update_time)
		VALUES (:name, :type, :currency, :balance, :is_active, :description, :create_time, :update_time)`,
		account,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Update 更新账户，只更新指定字段
func (r *AccountRepository) Update(id int64, fields map[string]interface{}) error {
	return updateAccountFields(r.DB, id, fields)
}

// UpdateWithTx 在事务内更新账户，只更新指定字段
func (r *AccountRepository) UpdateWithTx(tx DBTX, id int64, fields map[string]interface{}) error {
	return updateAccountFields(tx, id, fields)
}

// 动态构建 SET 子句更新账户字段，并刷新 update_time
// rows==0 表示目标不存在，返回 ErrRecordNotFound
func updateAccountFields(exec DBTX, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}

	query := "UPDATE accounts SET update_time = ?"
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

// AdjustBalance 调整账户余额（delta 单位：分，带符号，可为负）
// 在 SQL 内做 balance + delta 原子增减，避免读-改-写并发丢失更新
// delta 为 float64 但语义恒为整数分（由 Money int64 转换而来），
// 用 CAST(? AS INTEGER) 落库，避免 sqlite 将 balance 列值存成 REAL
func (r *AccountRepository) AdjustBalance(exec DBTX, id int64, delta float64) error {
	result, err := exec.Exec(
		"UPDATE accounts SET balance = balance + CAST(? AS INTEGER), update_time = ? WHERE id = ?",
		delta, time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// Delete 删除账户（逻辑删除：置 is_active=0）
// 选择逻辑删除而非物理删除：transactions.account_id 大量引用账户，
// 物理删除会产生悬空引用；备份清库场景由 DeleteAll 统一物理删除
func (r *AccountRepository) Delete(id int64) error {
	result, err := r.DB.Exec("UPDATE accounts SET is_active = 0, update_time = ? WHERE id = ?", time.Now(), id)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// DeleteAll 清空账户表（物理删除，供备份导入清库用）
func (r *AccountRepository) DeleteAll(exec DBTX) error {
	_, err := exec.Exec("DELETE FROM accounts")
	if err != nil {
		return err
	}
	return nil
}
