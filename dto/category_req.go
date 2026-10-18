// Package dto 定义请求和响应的数据传输对象
package dto

// QueryCategoryRequest 查询分类列表请求
type QueryCategoryRequest struct {
	Name     string  `form:"name"`      // 按名称模糊搜索
	Type     *string `form:"type"`      // 按类别筛选
	ParentID *int64  `form:"parent_id"` // 按父分类筛选
	PageSize int     `form:"page_size,default=100" binding:"min=1"`
	PageNum  int     `form:"page_num,default=1" binding:"min=1"`
}

// CreateCategoryRequest 新增分类请求
type CreateCategoryRequest struct {
	Name     string `json:"name"     binding:"required"`                               // 分类名称，必填
	Type     string `json:"type"     binding:"required,oneof=expense income transfer"` // 所属交易类别，必填
	ParentID *int64 `json:"parentId"`                                                  // 父分类 ID，可选
}

// UpdateCategoryRequest 更新分类请求，所有字段可选。
// ParentID 用 NullableInt64 区分：字段缺失=不更新；null=改为顶级分类；数字=指定父分类。
type UpdateCategoryRequest struct {
	Name     *string       `json:"name"`                                                          // 分类名称
	Type     *string       `json:"type"        binding:"omitempty,oneof=expense income transfer"` // 所属交易类别，可选
	ParentID NullableInt64 `json:"parentId" swaggertype:"integer"`                                // 父分类 ID，null 表示顶级分类，不存在表示不更新该字段别
	IsActive *bool         `json:"isActive"`                                                      // 是否启用
}
