package handler

import (
	"my-ledger/server/dto"
	"my-ledger/server/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// TransactionHandler 交易 HTTP 处理器
type TransactionHandler struct {
	svc *service.TransactionService
}

// NewTransactionHandler 创建交易处理器实例
func NewTransactionHandler(svc *service.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

// List 分页查询交易记录
// @Summary      分页查询交易记录
// @Tags         交易记录
// @Produce      json
// @Param        type        query  string  false  "交易类型"  Enums(expense, income, transfer)
// @Param        category_id query  int     false  "分类 ID"
// @Param        account_id  query  int     false  "账户 ID"
// @Param        start_date  query  string  false  "开始日期 YYYY-MM-DD"
// @Param        end_date    query  string  false  "结束日期 YYYY-MM-DD"
// @Param        keyword     query  string  false  "备注模糊搜索"
// @Param        page_size   query  int     false  "每页条数"  default(20)
// @Param        page_num    query  int     false  "页码"  default(1)
// @Success      200  {object}  dto.TransactionPageResponse
// @Failure      500  {object}  dto.Response
// @Router       /transactions [get]
func (h *TransactionHandler) List(c *gin.Context) {
	var req dto.QueryTransactionRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "参数校验失败: "+err.Error()))
		return
	}

	result, err := h.svc.ListTransactions(&req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(result))
}

// Get 查询单条交易记录
// @Summary      查询单条交易记录
// @Tags         交易记录
// @Produce      json
// @Param        id   path      int  true  "交易 ID"
// @Success      200  {object}  dto.TransactionResponse
// @Failure      500  {object}  dto.Response
// @Router       /transactions/{id} [get]
func (h *TransactionHandler) Get(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "无效的 ID"))
		return
	}

	t, err := h.svc.GetTransaction(id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(t))
}

// Create 新增交易记录
// @Summary      新增交易记录
// @Tags         交易记录
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreateTransactionRequest  true  "交易信息"
// @Success      200   {object}  dto.TransactionResponse
// @Failure      500   {object}  dto.Response
// @Router       /transactions [post]
func (h *TransactionHandler) Create(c *gin.Context) {
	var req dto.CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "参数校验失败: "+err.Error()))
		return
	}

	t, err := h.svc.CreateTransaction(&req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(t))
}

// Update 更新交易记录
// @Summary      更新交易记录
// @Tags         交易记录
// @Accept       json
// @Produce      json
// @Param        id    path      int                         true  "交易 ID"
// @Param        body  body      dto.UpdateTransactionRequest  true  "更新字段（可选，categoryId/toAccountId 为 null 表示清空）"
// @Success      200   {object}  dto.TransactionResponse
// @Failure      500   {object}  dto.Response
// @Router       /transactions/{id} [put]
func (h *TransactionHandler) Update(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "无效的 ID"))
		return
	}

	var req dto.UpdateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "参数校验失败: "+err.Error()))
		return
	}

	t, err := h.svc.UpdateTransaction(id, &req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(t))
}

// Delete 删除交易记录
// @Summary      删除交易记录
// @Tags         交易记录
// @Produce      json
// @Param        id   path      int  true  "交易 ID"
// @Success      200  {object}  dto.NilResponse
// @Failure      500  {object}  dto.Response
// @Router       /transactions/{id} [delete]
func (h *TransactionHandler) Delete(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "无效的 ID"))
		return
	}

	if err := h.svc.DeleteTransaction(id); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(nil))
}

// Daily 获取指定日期所有交易记录（含关联信息）
// @Summary      获取指定日期所有交易记录
// @Tags         交易记录
// @Produce      json
// @Param        date  query  string  false  "日期 YYYY-MM-DD（默认当天）"
// @Success      200  {object}  dto.Response{result=[]dto.DailyTransactionItem}
// @Failure      500  {object}  dto.Response
// @Router       /transactions/daily [get]
func (h *TransactionHandler) Daily(c *gin.Context) {
	var req dto.DailyTransactionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "参数校验失败: "+err.Error()))
		return
	}

	result, err := h.svc.GetDailyTransactions(&req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(result))
}

// Summary 按天汇总统计（收入、支出、结余）
// @Summary      按天汇总统计
// @Tags         交易记录
// @Produce      json
// @Param        startDate  query  string  false  "开始日期 YYYY-MM-DD（默认当天）"
// @Param        endDate    query  string  false  "结束日期 YYYY-MM-DD（默认当天）"
// @Success      200  {object}  dto.Response{result=[]dto.DailySummaryItem}
// @Failure      500  {object}  dto.Response
// @Router       /transactions/list [get]
func (h *TransactionHandler) Summary(c *gin.Context) {
	var req dto.DailySummaryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "参数校验失败: "+err.Error()))
		return
	}

	result, err := h.svc.GetDailySummary(&req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(result))
}
