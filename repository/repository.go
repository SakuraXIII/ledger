// Package repository 提供数据访问层
package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// AppError 应用错误，包含业务状态码和错误信息
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("code=%d, message=%s", e.Code, e.Message)
}

// 预定义错误
var (
	ErrRecordNotFound = &AppError{Code: 40401, Message: "资源不存在"}
	ErrParamInvalid   = &AppError{Code: 40001, Message: "参数校验失败"}
	ErrInternal       = &AppError{Code: 50001, Message: "服务器内部错误"}
)

// IsNotFoundError 判断是否为记录不存在错误
func IsNotFoundError(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// WrapNotFoundError 将 sql.ErrNoRows 包装为 AppError
func WrapNotFoundError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRecordNotFound
	}
	return err
}

// DBTX 数据库操作接口，*sqlx.DB 和 *sqlx.Tx 均实现此接口
// 用于在普通查询和事务查询间共享同一套 repository 方法
type DBTX interface {
	Exec(query string, args ...any) (sql.Result, error)
	Get(dest any, query string, args ...any) error
	Select(dest any, query string, args ...any) error
	NamedExec(query string, args interface{}) (sql.Result, error)
}

// BaseRepository 基础仓库，持有数据库连接
type BaseRepository struct {
	DB *sqlx.DB
}

// NewBaseRepository 创建基础仓库
func NewBaseRepository(db *sqlx.DB) *BaseRepository {
	return &BaseRepository{DB: db}
}
