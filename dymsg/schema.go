package dymsg

// FieldType 是字段类型。
type FieldType string

const (
	FieldInt32   FieldType = "int32"
	FieldInt64   FieldType = "int64"
	FieldUint32  FieldType = "uint32"
	FieldUint64  FieldType = "uint64"
	FieldFloat   FieldType = "float"
	FieldDouble  FieldType = "double"
	FieldBool    FieldType = "bool"
	FieldString  FieldType = "string"
	FieldBytes   FieldType = "bytes"
	FieldMessage FieldType = "message"
)

// MessageSchema 抽象消息类型的结构,内部实现自由。
type MessageSchema struct {
	// TODO
}

// ParseSchema 解析配置文件(JSON),返回顶层消息类型列表。
func ParseSchema(data []byte) ([]MessageSchema, error) {
	// TODO
	return nil, nil
}
