package handler

import (
	"log/slog"
	"my-ledger/server/dto"
	"net/http"

	"my-ledger/server/service"

	"github.com/gin-gonic/gin"
)

// CategoryHandler 分类 HTTP 处理器
type CategoryHandler struct {
	svc *service.CategoryService
}

// NewCategoryHandler 创建分类处理器实例
func NewCategoryHandler(svc *service.CategoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
}

// ListCategories 查询分类列表
// @Summary      查询分类列表
// @Tags         分类
// @Produce      json
// @Param        name       query  string  false  "分类名称模糊搜索"
// @Param        parent_id  query  int     false  "父分类 ID"
// @Param        page_size  query  int     false  "每页条数"  default(100)
// @Param        page_num   query  int     false  "页码"  default(1)
// @Success      200  {object}  dto.CategoryListResponse
// @Failure      400  {object}  dto.Response
// @Failure      500  {object}  dto.Response
// @Router       /categories [get]
func (h *CategoryHandler) ListCategories(c *gin.Context) {
	var queryDto dto.QueryCategoryRequest
	if err := c.ShouldBindQuery(&queryDto); err != nil {
		c.JSON(http.StatusBadRequest, dto.Error(40001, "无效请求参数"))
		return
	}

	categories, err := h.svc.ListCategories(&queryDto)
	if err != nil {
		slog.Error(err.Error())
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(categories))
}

// GetCategoryTree 查询分类树结构
// @Summary      查询分类树结构
// @Tags         分类
// @Produce      json
// @Success      200  {object}  dto.CategoryListResponse
// @Failure      500  {object}  dto.Response
// @Router       /categories/tree [get]
func (h *CategoryHandler) GetCategoryTree(c *gin.Context) {
	categories, err := h.svc.GetCategoryTree()
	if err != nil {
		slog.Error(err.Error())
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(categories))
}

// GetCategory 查询单个分类
// @Summary      查询单个分类
// @Tags         分类
// @Produce      json
// @Param        id   path      int  true  "分类 ID"
// @Success      200  {object}  dto.CategoryResponse
// @Failure      500  {object}  dto.Response
// @Router       /categories/{id} [get]
func (h *CategoryHandler) GetCategory(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "无效的 ID"))
		return
	}

	category, err := h.svc.GetCategory(id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(category))
}

// CreateCategory 新增分类
// @Summary      新增分类
// @Tags         分类
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreateCategoryRequest  true  "分类信息"
// @Success      200   {object}  dto.CategoryResponse
// @Failure      500   {object}  dto.Response
// @Router       /categories [post]
func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	var req dto.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "参数校验失败: "+err.Error()))
		return
	}

	category, err := h.svc.CreateCategory(&req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(category))
}

// UpdateCategory 更新分类
// @Summary      更新分类
// @Tags         分类
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "分类 ID"
// @Param        body  body      dto.UpdateCategoryRequest  true  "更新字段（可选，parentId 为 null 表示顶级分类）"
// @Success      200   {object}  dto.CategoryResponse
// @Failure      500   {object}  dto.Response
// @Router       /categories/{id} [put]
func (h *CategoryHandler) UpdateCategory(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "无效的 ID"))
		return
	}

	var req dto.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "参数校验失败: "+err.Error()))
		return
	}

	category, err := h.svc.UpdateCategory(id, &req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(category))
}

// DeleteCategory 删除分类
// @Summary      删除分类
// @Tags         分类
// @Produce      json
// @Param        id   path      int  true  "分类 ID"
// @Success      200  {object}  dto.NilResponse
// @Failure      500  {object}  dto.Response
// @Router       /categories/{id} [delete]
func (h *CategoryHandler) DeleteCategory(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "无效的 ID"))
		return
	}

	if err := h.svc.DeleteCategory(id); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(nil))
}
