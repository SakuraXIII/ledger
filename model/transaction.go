// Package model 定义数据模型
package model

// Transaction 交易记录模型
type Transaction struct {
	ID          int64       `json:"id"          db:"id"`
	Type        string      `json:"type"        db:"type"`          // expense/income/transfer
	Amount      Money       `json:"amount"      db:"amount"`        // 金额，恒为正数
	CategoryID  int64       `json:"categoryId"  db:"category_id"`   // 分类 ID
	AccountID   int64       `json:"accountId"   db:"account_id"`    // 付款/收款账户
	ToAccountID int64       `json:"toAccountId" db:"to_account_id"` // 转账目标账户（仅 transfer 有意义）
	Date        string      `json:"date"        db:"date"`          // 交易日期 YYYY-MM-DD
	Note        *string     `json:"note"        db:"note"`          // 备注
	CreateTime  DateTimeStr `json:"createTime"  db:"create_time"`
	UpdateTime  DateTimeStr `json:"updateTime"  db:"update_time"`
}
