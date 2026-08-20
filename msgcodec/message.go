package msgcodec

// Message 封装一个已注册消息实例,提供取值/赋值/编解码能力。
// 内部实现结构可自由设计,仅以本接口契约为约束。
type Message interface {
	// Get 按字段名取值,支持 "addr.city" 嵌套路径。
	// 路径不存在、字段未找到或中间节点不是 struct 时返回 ErrFieldNotFound;
	// 中间节点为 nil 指针时返回该节点零值。
	Get(field string) (any, error)

	// Set 按字段名赋值,支持 "addr.city" 嵌套路径。
	// nil 中间节点自动零值初始化;value 为 nil 时置零值;
	// 类型不可转换时返回 ErrTypeMismatch(转换规则见 SPEC.md)。
	Set(field string, value any) error

	// Fields 返回消息的所有字段描述,按声明顺序排列。
	Fields() []FieldDescriptor

	// EncodeJSON 编码为带信封的 JSON 消息。
	EncodeJSON() ([]byte, error)

	// EncodeProto 编码为带信封的 Protobuf 消息。
	EncodeProto() ([]byte, error)

	// DecodeJSON 从带信封的 JSON 字节解码到消息。
	// 空输入返回 ErrTruncated;信封或 payload 非法返回 ErrMalformedData。
	DecodeJSON(data []byte) error

	// DecodeProto 从带信封的 Protobuf 字节解码到消息。
	// 空输入返回 ErrTruncated;信封或 payload 非法返回 ErrMalformedData。
	DecodeProto(data []byte) error
}
