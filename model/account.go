// Package model 定义数据模型
package model

// Account 账户模型
type Account struct {
	ID          int64       `json:"id"        db:"id"`
	Name        string      `json:"name"      db:"name"`
	Type        string      `json:"type"      db:"type"`     // debit / credit / cash / investment
	Currency    string      `json:"currency"  db:"currency"` // 货币代码，如 CNY
	Balance     Money       `json:"balance"   db:"balance"`
	IsActive    bool        `json:"isActive"  db:"is_active"`
	CreateTime  DateTimeStr `json:"createTime" db:"create_time"`
	UpdateTime  DateTimeStr `json:"updateTime" db:"update_time"`
	Description *string     `json:"description" db:"description"`
}
