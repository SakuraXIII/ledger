package service

import (
	"my-ledger/server/dto"
	"my-ledger/server/model"
	"my-ledger/server/repository"
	"time"
)

// CategoryService 分类业务逻辑层
type CategoryService struct {
	repo *repository.CategoryRepository
}

// NewCategoryService 创建分类服务实例
func NewCategoryService(repo *repository.CategoryRepository) *CategoryService {
	return &CategoryService{repo: repo}
}

// ListCategories 查询分类列表
func (s *CategoryService) ListCategories(req *dto.QueryCategoryRequest) ([]model.Category, error) {
	return s.repo.FindAll(req)
}

// GetCategoryTree 查询分类树结构
func (s *CategoryService) GetCategoryTree() ([]model.Category, error) {
	return s.repo.FindAllTree()
}

// GetCategory 查询单个分类
func (s *CategoryService) GetCategory(id int64) (*model.Category, error) {
	return s.repo.FindByID(id)
}

// CreateCategory 新增分类
func (s *CategoryService) CreateCategory(req *dto.CreateCategoryRequest) (*model.Category, error) {
	category := &model.Category{
		Name:       req.Name,
		Type:       req.Type,
		ParentID:   req.ParentID,
		IsActive:   true,
		CreateTime: model.DateTimeStr(time.Now()),
		UpdateTime: model.DateTimeStr(time.Now()),
	}

	id, err := s.repo.Create(category)
	if err != nil {
		return nil, err
	}

	return s.repo.FindByID(id)
}

// UpdateCategory 更新分类
func (s *CategoryService) UpdateCategory(id int64, req *dto.UpdateCategoryRequest) (*model.Category, error) {
	// 构建需要更新的字段
	fields := make(map[string]interface{})
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Type != nil {
		fields["type"] = *req.Type
	}
	// ParentID 区分三种情况：字段缺失=不更新；null=改为顶级（SQL NULL）；数字=指定父分类
	if req.ParentID.Present {
		if req.ParentID.IsNull {
			fields["parent_id"] = nil
		} else {
			fields["parent_id"] = req.ParentID.Value
		}
	}
	if req.IsActive != nil {
		fields["is_active"] = *req.IsActive
	}

	if len(fields) == 0 {
		// 没有需要更新的字段，直接返回当前记录
		return s.repo.FindByID(id)
	}

	if err := s.repo.Update(id, fields); err != nil {
		return nil, err
	}

	return s.repo.FindByID(id)
}

// DeleteCategory 删除分类
func (s *CategoryService) DeleteCategory(id int64) error {
	return s.repo.Delete(id)
}
