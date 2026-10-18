// Package main 个人记账工具后端入口
package main

import (
	"fmt"
	"log/slog"

	"my-ledger/server/config"
	"my-ledger/server/database"
	"my-ledger/server/handler"
	"my-ledger/server/repository"
	"my-ledger/server/router"
	"my-ledger/server/service"
	"os"
)

func init() {
	//handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	//	AddSource: true,           // 显示文件行号，便于定位问题
	//	Level:     slog.LevelInfo, // 只输出 Info 及以上级别
	//})
	logHandle := handler.NewLogHandler(os.Stdout, slog.LevelInfo)
	slog.SetDefault(slog.New(logHandle))
}

// @title       My Ledger API
// @version     1.0
// @description 个人记账工具 API 文档
// @host        localhost:8080
// @BasePath    /api/v1
// @schemes     http
func main() {
	// 加载配置
	cfg := config.Load()
	slog.Info("启动配置: ", "port=%s", cfg.Port, "db=%s", cfg.DB, "mode=%s", cfg.Mode)

	// 初始化数据库连接
	db, dbErr := database.InitDB(cfg.DB)
	if dbErr != nil {
		slog.Error("数据库初始化失败: ", "err", dbErr)
	}
	defer db.Close()

	// 初始化各层依赖（构造函数注入）
	accountRepo := repository.NewAccountRepository(db)

	categoryRepo := repository.NewCategoryRepository(db)
	categorySvc := service.NewCategoryService(categoryRepo)
	categoryHandler := handler.NewCategoryHandler(categorySvc)

	transactionRepo := repository.NewTransactionRepository(db)
	accountSvc := service.NewAccountService(accountRepo, transactionRepo)
	accountHandler := handler.NewAccountHandler(accountSvc)
	transactionSvc := service.NewTransactionService(transactionRepo, accountRepo, categoryRepo)
	transactionHandler := handler.NewTransactionHandler(transactionSvc)

	backupSvc := service.NewBackupService(transactionRepo, accountRepo, categoryRepo)
	backupHandler := handler.NewBackupHandler(backupSvc)

	// 注册路由
	r := router.Setup(accountHandler, categoryHandler, transactionHandler, backupHandler, cfg.Mode)

	// 启动 HTTP 服务
	port := fmt.Sprintf(":%d", cfg.Port)
	slog.Info("服务启动在：", "http://localhost%s", port)
	if cfg.Mode == "debug" {
		slog.Info("Swagger enabled：", "http://localhost%s/swagger/index.html", port)
	}
	if ginErr := r.Run(port); ginErr != nil {
		slog.Error("服务启动失败: ", "err", ginErr)
	}
}
