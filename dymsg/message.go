package dymsg

// Message 封装一个动态消息实例,提供取值/赋值/编解码能力。
// 内部字段不预置,由实现者自行设计(例如消息 schema 描述、字段值存储、
// presence 标记等)。
//
// 字段路径语法(见 Get/Set):
//   - ""            当前消息自身
//   - "name"        字段名
//   - "addr.city"   嵌套字段(点分隔)
//   - "tags[0]"     repeated 字段的下标元素
//   - "items[0].name" 下标 + 嵌套组合
type Message struct {
	// TODO: 内部字段自行设计。
}

// Get 按字段路径取值,返回字段对应的 Message。
// 空字符串返回当前消息自身;字段不存在返回 ErrFieldNotFound;
// 字段存在但未设置返回 nil;repeated 下标越界返回 ErrIndexOutOfRange。
func (m *Message) Get(field string) (*Message, error) { panic("todo") }

// Set 按字段路径赋值,均采用深拷贝。
// value 为 nil 时清除该字段(置为未设置);
// 空字符串用 value 覆盖当前消息内容(用于复制);
// 标量字段传原生值(折中转换,不可转换返回 ErrTypeMismatch);
// 嵌套字段传 *Message;
// repeated 字段传 []any(标量)或 []*Message(消息),
// 也可用 make([]*Message, x) 得到长度为 x 的空消息数组;
// 下标形式 Set("tags[0]", v) 设置单个元素。
func (m *Message) Set(field string, value any) error { panic("todo") }

// Value 返回当前消息的原生值:
// 标量消息返回 int32/string 等;复合消息返回自身;
// repeated 返回 []any(标量元素)或 []*Message(消息元素)。
func (m *Message) Value() any { panic("todo") }

// EncodeJSON 将当前消息编码为 JSON 字节。
func (m *Message) EncodeJSON() ([]byte, error) { panic("todo") }

// EncodeProto 将当前消息编码为 Protobuf wire format 字节。
func (m *Message) EncodeProto() ([]byte, error) { panic("todo") }

// DecodeJSON 从 JSON 字节解码到当前消息。
// 空输入返回 ErrTruncated,数据非法返回 ErrMalformedData。
func (m *Message) DecodeJSON(data []byte) error { panic("todo") }

// DecodeProto 从 Protobuf wire format 字节解码到当前消息。
// 空输入返回 ErrTruncated,数据非法返回 ErrMalformedData。
func (m *Message) DecodeProto(data []byte) error { panic("todo") }
