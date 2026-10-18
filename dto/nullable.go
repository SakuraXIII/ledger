// Package dto 定义请求和响应的数据传输对象
package dto

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// NullableInt64 可空 int64，用于区分 JSON 中「字段不存在」「null」「具体数值」三种状态。
// 场景：更新分类请求中 parentId 字段缺失表示不更新，null 表示改为顶级分类，数字表示指定父分类。
type NullableInt64 struct {
	Value   int64 // 字段有值时的具体数值
	Present bool  // JSON 中是否存在该字段
	IsNull  bool  // 字段存在且值为 null
}

// UnmarshalJSON 实现 json.Unmarshaler：区分字段未传、null 和数字
func (n *NullableInt64) UnmarshalJSON(data []byte) error {
	n.Present = true
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		n.IsNull = true
		n.Value = 0
		return nil
	}
	n.IsNull = false
	if err := json.Unmarshal(trimmed, &n.Value); err != nil {
		return fmt.Errorf("parentId 必须是整数或 null: %w", err)
	}
	return nil
}
