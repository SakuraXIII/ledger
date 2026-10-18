// Package database 提供数据库初始化
package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动，注册驱动名为 "sqlite"
)

// InitDB 初始化 SQLite 数据库连接：确保目录存在，并通过 DSN pragma 为每个连接配置 PRAGMA
// 返回 *sqlx.DB，调用方负责 Close
func InitDB(dbPath string) (*sqlx.DB, error) {
	// 确保数据库文件所在目录存在（如 db/ledger.db 的 db 目录）
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	// 通过 DSN pragma 参数配置连接，每个新建连接都会自动带上这些 PRAGMA：
	//   journal_mode=WAL  —— WAL 日志模式，实现读写并发互不阻塞（库文件级属性，设置一次持久生效）
	//   busy_timeout=5000 —— 遇到写锁冲突时最多等待 5000ms，避免并发写时立即 SQLITE_BUSY 失败
	//   foreign_keys=ON   —— 开启外键约束（仅对库中显式声明了 FOREIGN KEY 的约束生效，无论该声明来自何处）
	//   synchronous=NORMAL —— WAL 模式下的推荐同步级别，兼顾安全与性能
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)",
		dbPath,
	)

	// sqlx.Connect 会Ping确认连接可用；modernc sqlite 注册的驱动名为 "sqlite"
	db, err := sqlx.Connect("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}
