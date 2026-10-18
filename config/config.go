// Package config 提供应用配置加载：合并默认值、环境变量与命令行参数
package config

import (
	"flag"
	"os"
	"strconv"
)

// Config 应用配置
type Config struct {
	Port int    // HTTP 服务端口
	DB   string // SQLite 数据库文件路径
	Mode string // 运行模式：debug / release
}

// Load 加载配置，优先级：命令行参数 > 环境变量 > 默认值
// 默认值：port=8080，db=db/ledger.db，mode=debug
// 环境变量：LEDGER_PORT / LEDGER_DB / LEDGER_MODE
// 命令行参数：-port / -db / -mode
func Load() *Config {
	// flag 默认值取零值，Load 内统一按「命令行是否显式传入」判断优先级
	port := flag.Int("port", 0, "HTTP 服务端口")
	db := flag.String("db", "", "SQLite 数据库文件路径")
	mode := flag.String("mode", "", "运行模式：debug/release")
	flag.Parse()

	// 先取默认值
	cfg := &Config{
		Port: 8080,
		DB:   "db/ledger.db",
		Mode: "debug",
	}

	// 环境变量覆盖默认值（非法端口值忽略，保持默认）
	if v := os.Getenv("LEDGER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("LEDGER_DB"); v != "" {
		cfg.DB = v
	}
	if v := os.Getenv("LEDGER_MODE"); v != "" {
		cfg.Mode = v
	}

	// 命令行优先级最高：仅在显式传入时覆盖
	if *port != 0 {
		cfg.Port = *port
	}
	if *db != "" {
		cfg.DB = *db
	}
	if *mode != "" {
		cfg.Mode = *mode
	}
	return cfg
}
