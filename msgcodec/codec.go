package msgcodec

// 信封常量(格式见 SPEC.md)。
const (
	// magic 是信封魔数 0x1234。
	magic = 0x1234
	// headerLen 是信封头长度:magic(2) + typeID(2) + encoding(1) + payloadLen(4)。
	headerLen = 9
	// encodingJSON 表示 payload 为 JSON 编码。
	encodingJSON byte = 0
	// encodingProto 表示 payload 为 Protobuf 编码。
	encodingProto byte = 1
)

// MarshalJSON 将任意值编码为 JSON 字节(不带信封)。
func MarshalJSON(v any) ([]byte, error) {
	// TODO
	return nil, nil
}

// UnmarshalJSON 从 JSON 字节解码到目标值(不带信封)。
func UnmarshalJSON(data []byte, v any) error {
	// TODO
	return nil
}

// MarshalProto 将任意值编码为 Protobuf 字节(不带信封)。
func MarshalProto(v any) ([]byte, error) {
	// TODO
	return nil, nil
}

// UnmarshalProto 从 Protobuf 字节解码到目标值(不带信封)。
func UnmarshalProto(data []byte, v any) error {
	// TODO
	return nil
}
