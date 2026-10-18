// Package dto 定义请求和响应的数据传输对象
package dto

import "my-ledger/server/model"

type QueryAccountRequest struct {
	Name     string `form:"name"`
	Type     string `form:"type"`
	Currency string `form:"currency"`
	PageSize int    `form:"page_size,default=15" binding:"min=1"`
	PageNum  int    `form:"page_num,default=1" binding:"min=1"`
}

// CreateAccountRequest 新增账户请求
type CreateAccountRequest struct {
	Name        string      `json:"name"     binding:"required"`                                          // 账户名称，必填
	Type        string      `json:"type"     binding:"required,oneof=e-pay debit credit cash investment"` // 账户类型，必填
	Currency    string      `json:"currency"`                                                             // 货币代码，可选
	Balance     model.Money `json:"balance"`                                                              // 初始金额，可选
	Description *string     `json:"description"`                                                          // 描述，可选
}

// UpdateAccountRequest 更新账户请求，所有字段可选
type UpdateAccountRequest struct {
	Name        *string      `json:"name"`                                                                   // 账户名称
	Type        *string      `json:"type"      binding:"omitempty,oneof=e-pay debit credit cash investment"` // 账户类型
	Currency    *string      `json:"currency"`                                                               // 货币代码
	Balance     *model.Money `json:"balance"`                                                                // 余额
	IsActive    *bool        `json:"isActive"`                                                               // 是否启用
	Description *string      `json:"description"`                                                            // 描述，可选
}
