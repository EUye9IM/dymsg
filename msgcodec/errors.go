package msgcodec

import "errors"

var (
	// ErrDuplicateID 表示重复注册了同一个类型 ID。
	ErrDuplicateID = errors.New("msgcodec: duplicate type id")
	// ErrUnknownTypeID 表示遇到未注册的类型 ID。
	ErrUnknownTypeID = errors.New("msgcodec: unknown type id")
	// ErrFieldNotFound 表示字段不存在。
	ErrFieldNotFound = errors.New("msgcodec: field not found")
	// ErrIndexOutOfRange 表示 repeated 字段下标越界。
	ErrIndexOutOfRange = errors.New("msgcodec: index out of range")
	// ErrTypeMismatch 表示赋值时类型无法转换(含数值溢出)。
	ErrTypeMismatch = errors.New("msgcodec: cannot convert value")
	// ErrMalformedData 表示编解码数据格式错误(含字段号越界/重复)。
	ErrMalformedData = errors.New("msgcodec: malformed data")
	// ErrTruncated 表示数据被截断。
	ErrTruncated = errors.New("msgcodec: truncated data")
)
