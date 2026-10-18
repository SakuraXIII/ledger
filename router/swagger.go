package router

import (
	_ "my-ledger/server/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/gin-gonic/gin"
)

// registerSwagger 注册 Swagger 文档路由（仅 debug 模式）
func registerSwagger(r *gin.Engine, mode string) {
	if mode == "debug" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
}
