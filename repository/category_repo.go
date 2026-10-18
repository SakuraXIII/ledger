package repository

import (
	"my-ledger/server/dto"
	"my-ledger/server/model"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// CategoryRepository 分类数据访问层
type CategoryRepository struct {
	*BaseRepository
}

// NewCategoryRepository 创建分类仓库实例
func NewCategoryRepository(db *sqlx.DB) *CategoryRepository {
	return &CategoryRepository{BaseRepository: NewBaseRepository(db)}
}

// FindAll 查询分类列表，支持多条件筛选
func (r *CategoryRepository) FindAll(req *dto.QueryCategoryRequest) ([]model.Category, error) {
	var categories = make([]model.Category, 0)
	var querySql = "SELECT * FROM categories"
	var args []any
	var conditions []string

	if req.Name != "" {
		conditions = append(conditions, "name LIKE ?")
		args = append(args, "%"+req.Name+"%")
	}
	if req.Type != nil && *req.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, *req.Type)
	}
	if req.ParentID != nil {
		if *req.ParentID == 0 {
			conditions = append(conditions, "parent_id IS NULL")
		} else {
			conditions = append(conditions, "parent_id = ?")
			args = append(args, *req.ParentID)
		}
	}

	args = append(args, req.PageSize, req.PageNum)
	if conditions != nil {
		querySql += " WHERE " + strings.Join(conditions, " AND ")
	}
	querySql += " ORDER BY create_time DESC, id DESC LIMIT ? OFFSET ?"

	err := r.DB.Select(&categories, querySql, args...)
	return categories, err
}

// FindAllTree 查询所有分类并构建树结构
func (r *CategoryRepository) FindAllTree() ([]model.Category, error) {
	var categories = make([]model.Category, 0)
	var querySql = "SELECT * FROM categories WHERE is_active = 1"
	var args []any

	querySql += " ORDER BY create_time DESC, id DESC"

	err := r.DB.Select(&categories, querySql, args...)
	if err != nil {
		return nil, err
	}

	// 构建树结构
	return buildCategoryTree(categories, nil), nil
}

// FindByID 根据 ID 查询单个分类
func (r *CategoryRepository) FindByID(id int64) (*model.Category, error) {
	var category model.Category
	err := r.DB.Get(&category, "SELECT * FROM categories WHERE id = ?", id)
	if err != nil {
		return nil, WrapNotFoundError(err)
	}
	return &category, nil
}

// Create 新增分类，返回新分类 ID
func (r *CategoryRepository) Create(category *model.Category) (int64, error) {
	result, err := r.DB.NamedExec(`
		INSERT INTO categories (name, type, parent_id, is_active, create_time, update_time)
		VALUES (:name, :type, :parent_id, :is_active, :create_time, :update_time)`,
		category,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Update 更新分类，只更新非 nil 字段
func (r *CategoryRepository) Update(id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}

	// 构建 SET 子句
	query := "UPDATE categories SET update_time = ?"
	args := []interface{}{time.Now()}
	for k, v := range fields {
		query += ", " + k + " = ?"
		args = append(args, v)
	}
	query += " WHERE id = ?"
	args = append(args, id)

	result, err := r.DB.Exec(query, args...)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// Delete 删除分类（逻辑删除）
func (r *CategoryRepository) Delete(id int64) error {
	result, err := r.DB.Exec("UPDATE categories SET is_active = 0 WHERE id = ? OR parent_id = ?", id, id)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// DeleteAll 清空分类表（物理删除）
func (r *CategoryRepository) DeleteAll(exec DBTX) error {
	_, err := exec.Exec("DELETE FROM categories")
	if err != nil {
		return err
	}
	return nil
}

// buildCategoryTree 递归构建分类树
func buildCategoryTree(categories []model.Category, parentID *int64) []model.Category {
	var tree = make([]model.Category, 0)
	for _, cat := range categories {
		if (parentID == nil && cat.ParentID == nil) ||
			(parentID != nil && cat.ParentID != nil && *parentID == *cat.ParentID) {
			children := buildCategoryTree(categories, &cat.ID)
			cat.Children = children
			tree = append(tree, cat)
		}
	}
	return tree
}
