package dymsg

// TODO: schema 注册表与运行时描述缓存设计(注意并发访问安全,评测以 -race 验收):
//
//	1. 维护 typeID → MessageSchema 的映射(Register 写入)。
//	2. 惰性构建并缓存每个消息类型的运行时描述:字段索引、嵌套关系、
//	   proto 字段号→字段的映射等,供 New/Get/Set/编解码共用。
//	3. 首次并发访问缓存时需保证并发安全。
//
// 内部结构可自行设计,对外契约见 message.go。

// Register 注册一个顶层消息 schema。
// 重复 typeID 返回 ErrDuplicateID。
func Register(s MessageSchema) error {
	// TODO
	return nil
}

// New 按类型 ID 创建一个空消息。
// ID 未注册时返回 ErrUnknownTypeID。
func New(typeID uint16) (*Message, error) {
	// TODO
	return nil, nil
}
