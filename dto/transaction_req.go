// Package dto 定义请求和响应的数据传输对象
package dto

import "my-ledger/server/model"

// QueryTransactionRequest 分页查询交易记录请求
type QueryTransactionRequest struct {
	Type       string `form:"type" binding:"omitempty,oneof=expense income transfer"`
	CategoryID *int64 `form:"categoryId"`
	AccountID  *int64 `form:"accountId"`
	StartDate  string `form:"startDate"`
	EndDate    string `form:"endDate"`
	Keyword    string `form:"keyword"` // 模糊匹配 note
	PageSize   int    `form:"page_size,default=20" binding:"min=1,max=100"`
	PageNum    int    `form:"page_num,default=1" binding:"min=1"`
}

// CreateTransactionRequest 新增交易请求
type CreateTransactionRequest struct {
	Type        string      `json:"type"       binding:"required,oneof=expense income transfer"`
	Amount      model.Money `json:"amount"     binding:"required,gt=0"`
	CategoryID  int64       `json:"categoryId" binding:"required"` // 所有类型都必填分类
	AccountID   int64       `json:"accountId"  binding:"required"`
	ToAccountID *int64      `json:"toAccountId"`
	Date        string      `json:"date"       binding:"required"` // YYYY-MM-DD
	Note        *string     `json:"note"`
}

// UpdateTransactionRequest 更新交易请求，所有字段可选
type UpdateTransactionRequest struct {
	Type        *string      `json:"type"        binding:"required,oneof=expense income transfer"`
	Amount      *model.Money `json:"amount"`
	CategoryID  *int64       `json:"categoryId"  swaggertype:"integer"`
	AccountID   *int64       `json:"accountId"`
	ToAccountID *int64       `json:"toAccountId" swaggertype:"integer"`
	Date        *string      `json:"date"`
	Note        *string      `json:"note"` // 空字符串表示清空
}

// BatchImportResult 批量导入结果
type BatchImportResult struct {
	Imported int              `json:"imported"`
	Errors   []BatchItemError `json:"errors"`
}

// BatchItemError 批量导入单条错误
type BatchItemError struct {
	Index   int    `json:"index"`
	Message string `json:"message"`
}

// DailyTransactionsRequest 获取指定日期交易记录请求
type DailyTransactionsRequest struct {
	Date string `form:"date"` // 可选，默认当天，格式 YYYY-MM-DD
}

// DailyTransactionItem 交易记录项（含关联信息）
type DailyTransactionItem struct {
	ID            int64             `json:"id"            db:"id"`
	Type          string            `json:"type"          db:"type"`
	Amount        model.Money       `json:"amount"        db:"amount"`
	CategoryID    *int64            `json:"categoryId"    db:"category_id"`
	AccountID     int64             `json:"accountId"     db:"account_id"`
	ToAccountID   *int64            `json:"toAccountId"   db:"to_account_id"`
	Date          string            `json:"date"          db:"date"`
	Note          *string           `json:"note"          db:"note"`
	CreateTime    model.DateTimeStr `json:"createTime"    db:"create_time"`
	UpdateTime    model.DateTimeStr `json:"updateTime"    db:"update_time"`
	CategoryName  *string           `json:"categoryName"  db:"category_name"`
	AccountName   *string           `json:"accountName"   db:"account_name"`
	ToAccountName *string           `json:"toAccountName" db:"to_account_name"`
}

// DailySummaryRequest 按天汇总请求
type DailySummaryRequest struct {
	StartDate string `form:"startDate"` // 可选，默认当天
	EndDate   string `form:"endDate"`   // 可选，默认当天
}

// DailySummaryItem 每天的汇总数据
type DailySummaryItem struct {
	Date    model.DateTimeStr `json:"date"    db:"date"`
	Income  model.Money       `json:"income"  db:"income"`
	Expense model.Money       `json:"expense" db:"expense"`
	Balance model.Money       `json:"balance" db:"balance"`
}
