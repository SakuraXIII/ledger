// Package model 定义数据模型
package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"time"
)

const dateTimeFormat = time.DateTime

// DateTimeStr 自定义时间类型，JSON 序列化为 "YYYY-MM-DD HH:mm:ss" 格式
type DateTimeStr time.Time

func (t DateTimeStr) Value() (driver.Value, error) {
	ti := time.Time(t)
	if ti.IsZero() {
		return nil, nil
	}
	return ti.Format(dateTimeFormat), nil
}

func (t *DateTimeStr) Scan(src interface{}) error {
	if src == nil {
		return errors.New("字段值为 NULL")
	}
	tv, ok := src.(time.Time)
	if !ok {
		return errors.New("无法从数据库转换类型为 DateTimeStr")
	}
	*t = DateTimeStr(tv)
	return nil
}

// MarshalJSON 实现 json.Marshaler 接口
func (t DateTimeStr) MarshalJSON() ([]byte, error) {
	tm := time.Time(t)
	if tm.IsZero() {
		return []byte(`"0000-00-00 00:00:00"`), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, tm.Format(dateTimeFormat))), nil
}

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (t *DateTimeStr) UnmarshalJSON(data []byte) error {
	str := string(data)
	if str == `""` || str == "null" {
		*t = DateTimeStr(time.Time{})
		return nil
	}
	// 去掉引号
	str = str[1 : len(str)-1]
	tm, err := time.Parse(dateTimeFormat, str)
	if err != nil {
		return err
	}
	*t = DateTimeStr(tm)
	return nil
}

// String 实现 fmt.Stringer 接口
func (t DateTimeStr) String() string {
	return time.Time(t).Format(dateTimeFormat)
}
