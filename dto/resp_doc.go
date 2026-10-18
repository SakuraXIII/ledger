// Package dto 定义请求和响应的数据传输对象
package dto

import "my-ledger/server/model"

// HealthResponse 健康检查响应（文档用）
type HealthResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  struct {
		Status string `json:"status"`
	} `json:"result"`
}

// AccountResponse 账户详情响应（文档用）
type AccountResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Result  model.Account `json:"result"`
}

// AccountListResponse 账户列表响应（文档用）
type AccountListResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Result  []model.Account `json:"result"`
}

// CategoryResponse 分类详情响应（文档用）
type CategoryResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Result  model.Category `json:"result"`
}

// CategoryListResponse 分类列表响应（文档用）
type CategoryListResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Result  []model.Category `json:"result"`
}

// TransactionResponse 交易详情响应（文档用）
type TransactionResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Result  model.Transaction `json:"result"`
}

// TransactionPageResponse 交易分页响应（文档用）
type TransactionPageResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Result  PageResult `json:"result"`
}

// NilResponse 通用空结果响应（文档用）
type NilResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Result  interface{} `json:"result"`
}
