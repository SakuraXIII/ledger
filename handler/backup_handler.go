// Package handler 提供 HTTP 处理层
package handler

import (
	"fmt"
	"my-ledger/server/dto"
	"my-ledger/server/service"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// BackupHandler 备份导入导出 HTTP 处理器
type BackupHandler struct {
	svc *service.BackupService
}

// NewBackupHandler 创建备份处理器实例
func NewBackupHandler(svc *service.BackupService) *BackupHandler {
	return &BackupHandler{svc: svc}
}

// Export 导出全量交易记录为 CSV 文件
// @Summary      导出交易记录为 CSV
// @Tags         备份导入导出
// @Produce      text/csv
// @Success      200  {file}   file
// @Failure      500  {object} dto.Response
// @Router       /backup/export [get]
func (h *BackupHandler) Export(c *gin.Context) {
	data, err := h.svc.ExportCSV()
	if err != nil {
		handleError(c, err)
		return
	}

	// 拼接 BOM 头（Excel 识别 UTF-8）
	body := append([]byte{0xEF, 0xBB, 0xBF}, data...)

	// 设置响应头，触发浏览器下载
	// 使用 RFC 5987 编码中文文件名，兼容所有浏览器
	utf8Filename := fmt.Sprintf("交易备份_%s.csv", time.Now().Format("2006年01月02日_150405"))
	encodedFilename := url.PathEscape(utf8Filename)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, encodedFilename))
	c.Header("Content-Length", fmt.Sprintf("%d", len(body)))

	c.Data(http.StatusOK, "text/csv; charset=utf-8", body)
}

// Import 从 CSV 文件导入交易记录
// @Summary      从 CSV 导入交易记录
// @Tags         备份导入导出
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "CSV 文件"
// @Success      200   {object}  dto.Response{result=dto.BatchImportResult}
// @Failure      400   {object}  dto.Response
// @Failure      500   {object}  dto.Response
// @Router       /backup/import [post]
func (h *BackupHandler) Import(c *gin.Context) {
	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "获取上传文件失败: "+err.Error()))
		return
	}

	// 校验文件大小（限制 10MB）
	if file.Size > 10*1024*1024 {
		c.JSON(http.StatusOK, dto.Error(40001, "文件大小超过限制（最大 10MB）"))
		return
	}

	// 校验文件扩展名
	if file.Filename == "" {
		c.JSON(http.StatusOK, dto.Error(40001, "文件名不能为空"))
		return
	}

	// 打开文件（multipart.File 本身实现 io.Reader，直接流式传给 service，不再整体读入内存）
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusOK, dto.Error(40001, "打开文件失败: "+err.Error()))
		return
	}
	// nolint:errcheck
	defer f.Close()

	// 调用 service 层流式导入（文件由下方 defer 在 ImportCSV 返回后关闭）
	result, err := h.svc.ImportCSV(f)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.Success(result))
}
