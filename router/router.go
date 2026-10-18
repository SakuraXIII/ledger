// Package router 提供路由注册
package router

import (
	"my-ledger/server/handler"

	"github.com/gin-gonic/gin"
)

// Setup 注册所有路由
func Setup(accountHandler *handler.AccountHandler, categoryHandler *handler.CategoryHandler, transactionHandler *handler.TransactionHandler, backupHandler *handler.BackupHandler, mode string) *gin.Engine {
	r := gin.Default()
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	_ = r.SetTrustedProxies([]string{"127.0.0.1"})

	registerSwagger(r, mode)

	// API v1 路由组
	v1 := r.Group("/api/v1")
	{
		// 健康检查
		v1.GET("/health", accountHandler.Health)

		// 账户管理
		accounts := v1.Group("/accounts")
		{
			accounts.GET("", accountHandler.ListAccounts)         // 查询全部
			accounts.POST("", accountHandler.CreateAccount)       // 新增
			accounts.GET("/:id", accountHandler.GetAccount)       // 查询单个
			accounts.PUT("/:id", accountHandler.UpdateAccount)    // 更新
			accounts.DELETE("/:id", accountHandler.DeleteAccount) // 删除
		}

		// 分类管理
		categories := v1.Group("/categories")
		{
			categories.GET("", categoryHandler.ListCategories)        // 查询列表
			categories.GET("/tree", categoryHandler.GetCategoryTree)  // 查询树结构
			categories.POST("", categoryHandler.CreateCategory)       // 新增
			categories.GET("/:id", categoryHandler.GetCategory)       // 查询单个
			categories.PUT("/:id", categoryHandler.UpdateCategory)    // 更新
			categories.DELETE("/:id", categoryHandler.DeleteCategory) // 删除
		}

		// 交易记录
		transactions := v1.Group("/transactions")
		{
			transactions.GET("", transactionHandler.List)          // 分页查询
			transactions.POST("", transactionHandler.Create)       // 新增
			transactions.GET("/daily", transactionHandler.Daily)   // 获取指定日期所有交易记录
			transactions.GET("/list", transactionHandler.Summary)  // 按天汇总统计
			transactions.GET("/:id", transactionHandler.Get)       // 查询单个
			transactions.PUT("/:id", transactionHandler.Update)    // 更新
			transactions.DELETE("/:id", transactionHandler.Delete) // 删除
		}

		// 备份导入导出
		backup := v1.Group("/backup")
		{
			backup.GET("/export", backupHandler.Export)  // 导出 CSV
			backup.POST("/import", backupHandler.Import) // 导入 CSV
		}
	}

	return r
}
