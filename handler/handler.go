// Package handler 提供 HTTP 处理层
package handler

import (
	"errors"
	"log/slog"
	"my-ledger/server/dto"
	"net/http"

	"my-ledger/server/repository"

	"github.com/gin-gonic/gin"
)

// handleError 统一错误处理
// 将业务错误转换为标准 JSON 响应
func handleError(c *gin.Context, err error) {
	slog.Error(err.Error())
	if appErr, ok := errors.AsType[*repository.AppError](err); ok {
		c.JSON(http.StatusOK, dto.Error(appErr.Code, appErr.Message))
		return
	}

	// 未知错误返回服务器内部错误
	c.JSON(http.StatusInternalServerError, dto.Error(50001, "服务器内部错误"))
}
