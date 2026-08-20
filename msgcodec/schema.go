package msgcodec

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

// FieldSchema 描述消息类型的一个字段。
type FieldSchema struct {
	// Name 是字段名,同时作为 JSON key。
	Name string
	// Num 是 proto 字段号,范围 [1, 65535]。
	Num int
	// Type 是字段类型。
	Type FieldType
	// Repeated 表示字段是否为数组。
	Repeated bool
	// Schema 是 Type == FieldMessage 时指向的内联嵌套 schema。
	Schema MessageSchema
}

// MessageSchema 抽象消息类型的结构,内部实现自由。
type MessageSchema interface {
	// TypeID 返回顶层注册类型的 ID;内联嵌套 schema 返回 0。
	TypeID() uint16
	// Fields 按声明顺序返回字段描述。
	Fields() []*FieldSchema
	// Field 按字段名查找字段描述。
	Field(name string) (*FieldSchema, bool)
}

// ParseSchema 解析配置文件(JSON),返回顶层消息类型列表。
func ParseSchema(data []byte) ([]MessageSchema, error) {
	// TODO
	return nil, nil
}
