package msgcodec

import "reflect"

// FieldDescriptor 是 Fields() 对外暴露的字段元数据。
type FieldDescriptor struct {
	// Name 是 Go 结构体字段名。
	Name string
	// JSONKey 是 JSON key。
	JSONKey string
	// ProtoNum 是 proto 字段号;0 表示 proto 编解码时忽略。
	ProtoNum uint16
	// Type 是字段的 Go 类型。
	Type reflect.Type
}
