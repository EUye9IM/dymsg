package msgcodec

import "errors"

var (
	// ErrDuplicateID 表示不同类型抢注了同一个类型 ID。
	ErrDuplicateID = errors.New("msgcodec: duplicate type id")
	// ErrUnknownTypeID 表示遇到未注册的类型 ID。
	ErrUnknownTypeID = errors.New("msgcodec: unknown type id")
	// ErrFieldNotFound 表示 Get/Set 的字段或路径不存在,或中间节点不是 struct。
	ErrFieldNotFound = errors.New("msgcodec: field not found")
	// ErrTypeMismatch 表示赋值时类型无法转换(含数值溢出)。
	ErrTypeMismatch = errors.New("msgcodec: cannot convert value")
	// ErrMalformedData 表示信封或 payload 格式错误(含字段号越界/重复)。
	ErrMalformedData = errors.New("msgcodec: malformed data")
	// ErrTruncated 表示数据被截断。
	ErrTruncated = errors.New("msgcodec: truncated data")
)
