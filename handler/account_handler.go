package handler

import (
	"my-ledger/server/dto"
	"net/http"
	"strconv"

	"my-ledger/server/service"

	"github.com/gin-gonic/gin"
)

// AccountHandler 账户 HTTP 处理器
type AccountHandler struct {
	svc *service.AccountService
}

// NewAccountHandler 创建账户处理器实例
func NewAccountHandler(svc *service.AccountService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

// Health 健康检查
// @Summary      健康检查
// @Tags         系统
// @Produce      json
// @Success      200  {object}  dto.HealthResponse
// @Router       /health [get]
func (h *AccountHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, dto.Success(gin.H{
		"status": "ok",
	}))
}

// ListAccounts 查询账户列表
// @Summary      查询账户列表
// @Tags         账户
// @Produce      json
// @Param        name      query  string  false  "账户名称模糊搜索"
// @Param        type      query  string  false  "账户类型"  Enums(e-pay, debit, credit, cash, investment)
// @Param        currency  query  string  false  "货币代码"
// @Param        page_size query  int     false  "每页条数"  default(15)
// @Param        page_num  query  int     false  "页码"  default(1)
// @Success      200  {object}  dto.AccountListResponse
// @Failure      400  {object}  dto.Response
// @Failure      500  {object}  dto.Response
// @Router       /accounts [get]
func (h *AccountHandler) ListAccounts(c *gin.Context) {
	var queryDto dto.QueryAccountRequest
	if err := c.ShouldBindQuery(&queryDto); err != nil {
		c.JSON(http.StatusBadRequest, dto.Error(40001, "无效请求参数"))
		return
	}

	accounts, err := h.svc.ListAccounts(&queryDto)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(accounts))
}

// GetAccount 查询单个账户
// @Summary      查询单个账户
// @Tags         账户
// @Produce      json
// @Param        id   path      int  true  "账户 ID"
// @Success      200  {object}  dto.AccountResponse
// @Failure      500  {object}  dto.Response
// @Router       /accounts/{id} [get]
func (h *AccountHandler) GetAccount(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "无效的 ID"))
		return
	}

	account, err := h.svc.GetAccount(id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(account))
}

// CreateAccount 新增账户
// @Summary      新增账户
// @Tags         账户
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreateAccountRequest  true  "账户信息"
// @Success      200   {object}  dto.AccountResponse
// @Failure      500   {object}  dto.Response
// @Router       /accounts [post]
func (h *AccountHandler) CreateAccount(c *gin.Context) {
	var req dto.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "参数校验失败: "+err.Error()))
		return
	}

	account, err := h.svc.CreateAccount(&req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(account))
}

// UpdateAccount 更新账户
// @Summary      更新账户
// @Tags         账户
// @Accept       json
// @Produce      json
// @Param        id    path      int                      true   "账户 ID"
// @Param        body  body      dto.UpdateAccountRequest  true   "更新字段（可选）"
// @Success      200   {object}  dto.AccountResponse
// @Failure      500   {object}  dto.Response
// @Router       /accounts/{id} [put]
func (h *AccountHandler) UpdateAccount(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "无效的 ID"))
		return
	}

	var req dto.UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "参数校验失败: "+err.Error()))
		return
	}

	account, err := h.svc.UpdateAccount(id, &req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(account))
}

// DeleteAccount 删除账户
// @Summary      删除账户
// @Tags         账户
// @Produce      json
// @Param        id   path      int  true  "账户 ID"
// @Success      200  {object}  dto.NilResponse
// @Failure      500  {object}  dto.Response
// @Router       /accounts/{id} [delete]
func (h *AccountHandler) DeleteAccount(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "无效的 ID"))
		return
	}

	if err := h.svc.DeleteAccount(id); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(nil))
}

// parseID 从 URL 参数中解析 ID
func parseID(c *gin.Context, param string) (int64, error) {
	return strconv.ParseInt(c.Param(param), 10, 64)
}
