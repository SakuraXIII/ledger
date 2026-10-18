// Package model 定义数据模型
package model

// Category 分类模型，支持多级分类树
type Category struct {
	ID         int64       `json:"id"        db:"id"`
	Name       string      `json:"name"      db:"name"`
	Type       string      `json:"type"      db:"type"`
	ParentID   *int64      `json:"parentId"  db:"parent_id"` // 父分类 ID，nil 表示顶级分类
	IsActive   bool        `json:"isActive"  db:"is_active"`
	CreateTime DateTimeStr `json:"createTime" db:"create_time"`
	UpdateTime DateTimeStr `json:"updateTime" db:"update_time"`
	Children   []Category  `json:"children,omitempty" db:"-"` // 子分类列表，非数据库字段
}
