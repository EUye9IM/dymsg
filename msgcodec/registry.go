package msgcodec

// TODO: 类型注册与类型描述符缓存设计(注意并发访问安全,评测会以 -race 验收):
//
//	1. 维护 typeID 与类型的双向映射(Register 写入)。
//	2. 惰性构建并缓存每个消息类型的反射分析结果:字段列表、JSON key、
//	   proto 字段号、嵌套路径,供 Wrap/Get/Set/编解码共用。
//	3. 首次并发访问缓存时需保证并发安全。
//
// 注意:内部描述符结构可自行设计,对外契约仅见 message.go 的 Message 接口。

// Register 注册消息类型并绑定类型 ID。
// 重复注册同一类型(相同 ID)为幂等操作;不同类型抢注同一 ID 返回 ErrDuplicateID。
func Register[T any](typeID uint16) error {
	// TODO
	return nil
}

// Wrap 包装一个消息实例,返回可进行取值/赋值/编解码的 Message。
// 类型未注册时返回 ErrUnknownTypeID。
func Wrap(v any) (Message, error) {
	// TODO
	return nil, nil
}

// New 按类型 ID 创建空消息。
// ID 未注册时返回 ErrUnknownTypeID。
func New(typeID uint16) (Message, error) {
	// TODO
	return nil, nil
}
