package msgcodec

import (
	"reflect"
	"sort"
	"sync"
)

// runtimeField 是字段的运行时描述。
type runtimeField struct {
	name     string
	num      int
	typ      FieldType
	repeated bool
	child    *runtimeSchema // message 字段的嵌套 schema
}

// runtimeSchema 是消息类型的运行时描述,由 Register 构建并缓存。
type runtimeSchema struct {
	typeID uint16
	decl   []*runtimeField // 声明顺序
	numAsc []*runtimeField // proto 字段号升序
	byName map[string]*runtimeField
	byNum  map[int]*runtimeField
}

var regMu sync.RWMutex

// reg 按 typeID 索引已注册 schema。
var reg = map[uint16]*runtimeSchema{}

// regObj 按 typeID 记录原始 schema,用于幂等判断。
var regObj = map[uint16]MessageSchema{}

func buildRuntime(fs *FieldSchema, child *runtimeSchema) (*runtimeField, error) {
	rf := &runtimeField{name: fs.Name, num: fs.Num, typ: fs.Type, repeated: fs.Repeated, child: child}
	if fs.Type != FieldMessage && child != nil {
		return nil, ErrMalformedData
	}
	if fs.Type == FieldMessage && child == nil {
		return nil, ErrMalformedData
	}
	return rf, nil
}

func buildSchema(s MessageSchema, requireID bool) (*runtimeSchema, error) {
	if s == nil {
		return nil, ErrMalformedData
	}
	id := s.TypeID()
	if requireID && id == 0 {
		return nil, ErrMalformedData
	}
	rs := &runtimeSchema{
		typeID: id,
		byName: make(map[string]*runtimeField),
		byNum:  make(map[int]*runtimeField),
	}
	seenName := make(map[string]bool)
	seenNum := make(map[int]bool)
	for _, fs := range s.Fields() {
		if !validFieldName(fs.Name) || seenName[fs.Name] {
			return nil, ErrMalformedData
		}
		seenName[fs.Name] = true
		if fs.Num < 1 || fs.Num > 65535 || seenNum[fs.Num] {
			return nil, ErrMalformedData
		}
		seenNum[fs.Num] = true
		var child *runtimeSchema
		if fs.Type == FieldMessage {
			sub, err := buildSchema(fs.Schema, false)
			if err != nil {
				return nil, err
			}
			child = sub
		}
		rf, err := buildRuntime(fs, child)
		if err != nil {
			return nil, err
		}
		rs.decl = append(rs.decl, rf)
		rs.byName[rf.name] = rf
		rs.byNum[rf.num] = rf
	}
	rs.numAsc = append([]*runtimeField(nil), rs.decl...)
	sort.Slice(rs.numAsc, func(i, j int) bool { return rs.numAsc[i].num < rs.numAsc[j].num })
	return rs, nil
}

// Register 注册一个顶层消息 schema。
// 重复 typeID 返回 ErrDuplicateID;相同类型重复注册为幂等。
func Register(s MessageSchema) error {
	if s == nil {
		return ErrMalformedData
	}
	rs, err := buildSchema(s, true)
	if err != nil {
		return ErrMalformedData
	}
	regMu.Lock()
	defer regMu.Unlock()
	if prev, ok := regObj[rs.typeID]; ok {
		if reflect.DeepEqual(prev, s) {
			return nil
		}
		return ErrDuplicateID
	}
	regObj[rs.typeID] = s
	reg[rs.typeID] = rs
	return nil
}

// New 按类型 ID 创建一个空消息。
// ID 未注册时返回 ErrUnknownTypeID。
func New(typeID uint16) (Message, error) {
	regMu.RLock()
	rs, ok := reg[typeID]
	regMu.RUnlock()
	if !ok {
		return nil, ErrUnknownTypeID
	}
	return newMessage(rs), nil
}
