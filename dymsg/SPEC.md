# dymsg 规范

本文件是 `dymsg` 包的**唯一权威约定**。实现须严格遵循本规范,测试将据此验收。

## 1. 概述

`dymsg` 是一个动态消息编解码库:消息结构由配置文件定义(而非编译期写死的 Go 结构体),运行时按类型 ID 注册,支持字段级取值/赋值与 JSON / Protobuf 双编解码。

## 2. 接口契约

### 2.1 字段类型(`FieldType`)

| 常量           | 字符串      | 原生 Go 类型 |
| -------------- | ----------- | ------------ |
| `FieldInt32`   | `"int32"`   | `int32`      |
| `FieldInt64`   | `"int64"`   | `int64`      |
| `FieldUint32`  | `"uint32"`  | `uint32`     |
| `FieldUint64`  | `"uint64"`  | `uint64`     |
| `FieldFloat`   | `"float"`   | `float32`    |
| `FieldDouble`  | `"double"`  | `float64`    |
| `FieldBool`    | `"bool"`    | `bool`       |
| `FieldString`  | `"string"`  | `string`     |
| `FieldBytes`   | `"bytes"`   | `[]byte`     |
| `FieldMessage` | `"message"` | `*Message`   |

```go
type FieldType string

type FieldSchema struct {
    Name     string         // 字段名,同时作为 JSON key
    Num      int            // proto 字段号,范围 [1, 65535]
    Type     FieldType
    Repeated bool           // 是否为数组
    Schema   MessageSchema  // Type == message 时指向内联嵌套 schema
}
```

### 2.2 Schema 结构体(`MessageSchema`)

```go
type MessageSchema struct {
    // ...
}
```

- `ParseSchema` 返回的每个 `MessageSchema` 代表一个顶层消息类型。
- 结构体内部字段由实现者自由设计,但必须能支撑第 5、8、9 节的编解码与取值语义。

### 2.3 消息结构体(`Message`)

```go
type Message struct {
    // ...
}

func (m *Message) Get(field string) (*Message, error)
func (m *Message) Set(field string, value any) error
func (m *Message) Value() any
func (m *Message) EncodeJSON() ([]byte, error)
func (m *Message) EncodeProto() ([]byte, error)
func (m *Message) DecodeJSON(data []byte) error
func (m *Message) DecodeProto(data []byte) error
```

### 2.4 注册与构造

```go
func ParseSchema(data []byte) ([]MessageSchema, error)
func Register(s MessageSchema) error
func New(typeID uint16) (*Message, error)
```

### 2.5 哨兵错误

```go
var (
    ErrDuplicateID     // 重复注册同一 typeID
    ErrUnknownTypeID   // 遇到未注册 typeID
    ErrFieldNotFound   // 字段不存在
    ErrIndexOutOfRange // repeated 下标越界
    ErrTypeMismatch    // 赋值类型无法转换(含数值溢出)
    ErrMalformedData   // 数据格式错误或与 schema 不符
    ErrTruncated       // 数据被截断
)
```

## 3. 配置文件格式(JSON)

```json
{
  "types": [
    {
      "typeId": 1001,
      "fields": [
        { "name": "name", "type": "string", "num": 1 },
        { "name": "age", "type": "int32", "num": 2 },
        {
          "name": "addr",
          "type": "message",
          "num": 3,
          "schema": {
            "fields": [
              { "name": "city", "type": "string", "num": 1 },
              { "name": "zip", "type": "string", "num": 2 }
            ]
          }
        },
        { "name": "tags", "type": "string", "num": 4, "repeated": true }
      ]
    }
  ]
}
```

规则:

- 顶层是 `types` 数组,每个元素定义一个顶层消息类型。
- `typeId` 仅顶层类型有,范围 [1, 65535];同一配置文件内不得重复。
- 每个字段: `name`(必填,同时作为 JSON key)、`type`(必填,取值见 2.1)、`num`(必填,proto 字段号,范围 [1, 65535])。
- `repeated`(可选,默认 false)表示字段为数组。
- `schema` 仅当 `type == "message"` 时出现,内联定义嵌套消息结构;嵌套 schema 无 `typeId`。
- 同一消息类型内字段 `num` 必须唯一;字段 `name` 必须唯一。
- 配置非法(JSON 语法错误、类型名非法、typeId/num 越界或重复)时,`ParseSchema` 返回 `ErrMalformedData`。

## 4. 字段路径

`Get` / `Set` 的 `field` 参数是一个路径表达式:

| 形式              | 含义                    |
| ----------------- | ----------------------- |
| `""`              | 当前消息自身            |
| `"name"`          | 字段名                  |
| `"addr.city"`     | 嵌套字段(`.` 分隔)      |
| `"tags[0]"`       | repeated 字段的下标元素 |
| `"items[0].name"` | 下标 + 嵌套组合         |

- 路径用 `.` 分隔字段名,`[n]` 表示下标(`n` 为非负十进制整数)。
- 字段名不得包含 `.` 或 `[` 字符。

## 5. 取值/赋值语义

每个字段具有**存在性(presence)**:区分「未设置」与「显式设置为某值(含零值)」。

- 未设置的字段在 Proto/JSON 编码中**不出现**;已设置的字段出现(即使值为零值)。
- `Get` 未设置字段返回 `nil`(不报错)。
- `Set(field, nil)` 将字段清除为未设置。

### 5.1 `Get(field) (*Message, error)`

返回路径所指字段对应的 `*Message`。

- `Get("")` 返回当前消息自身。
- 路径中字段不存在 → `ErrFieldNotFound`。
- 字段存在但未设置 → 返回 `nil`(不报错)。
- repeated 下标越界 → `ErrIndexOutOfRange`。
- 对非 repeated 字段使用下标 → `ErrFieldNotFound`(下标仅适用于 repeated 字段)。
- `Get("tags")`(repeated 字段不加下标)返回整个数组对应的 `*Message`。

### 5.2 `Set(field, value any) error`

按路径赋值,均采用深拷贝(见第 7 节)。

- `value` 为 nil → 清除该字段(presence = false);`Set("", nil)` 清除整个消息的全部字段。
- `Set("", m)`:用 `value` 覆盖当前消息内容(整体复制)。`value` 须为同构的 `*Message`。
- 标量字段:`value` 为对应原生类型或可转换类型(见第 6 节)。
- 嵌套字段(`message`):`value` 为 `*Message`。
- repeated 字段:`value` 为 `[]any`(标量元素)或 `[]*Message`(消息元素);也可传 `make([]*Message, x)` 得到长度为 `x` 的空消息数组。
- 下标形式 `Set("tags[0]", v)`:设置单个元素;仅 repeated 字段允许下标。
- 对非 repeated 字段使用下标 → `ErrFieldNotFound`。
- 转换失败 → `ErrTypeMismatch`;字段不存在 → `ErrFieldNotFound`;下标越界 → `ErrIndexOutOfRange`。

### 5.3 `Value() any`

返回当前消息的原生值:

- 标量字段 → 对应原生 Go 类型(见 2.1 表)。
- 嵌套消息 → 自身 `*Message`。
- repeated 标量 → `[]T`(类型为对应原生类型)。
- repeated 消息 → `[]*Message`。

## 6. 折中类型转换(标量 Set)

| 场景                          | 行为                                              |
| ----------------------------- | ------------------------------------------------- |
| 类型完全相同                  | 直接赋值                                          |
| 数值互转(int/uint/float 变体) | 转换;溢出报 `ErrTypeMismatch`                     |
| string ↔ 数值                 | 用 `strconv` 解析/格式化;失败报 `ErrTypeMismatch` |
| []byte ↔ string               | 允许                                              |
| 底层类型相同的别名类型        | 允许                                              |
| 其他                          | 报 `ErrTypeMismatch`                              |

## 7. 深拷贝

`Set("", m)` 与 `Set("field", msg)`(嵌套)均为**深拷贝**:目标消息与源消息此后完全独立,修改一方不影响另一方。

## 8. JSON 编码

- JSON key 使用字段 `name`。
- 字段按 schema 声明顺序输出。
- 各类型映射:整数/浮点 → JSON number;`bool` → JSON bool;`string` → JSON string;`bytes` → base64 字符串;`message` → 嵌套 JSON 对象;`repeated` → JSON 数组。
- 未设置字段不输出(缺 key);已设置字段输出(含零值);`message` 为 nil 视为未设置。
- 解码时忽略未知 JSON key;字段值为 null → 视为未设置;字段值与 schema 类型不符 → `ErrMalformedData`;空输入 → `ErrTruncated`。

## 9. Protobuf wire format

自实现的基础子集,不依赖 `google.golang.org/protobuf`。

### 9.1 varint

无符号 64 位整数编码为变长字节序列:每个字节低 7 位承载数据,最高位(MSB)为 1 表示还有后续字节、为 0 表示结束。

- `uint32`/`uint64`:直接 varint 编码。
- `int32`/`int64`:按int64补码对应的uint64 varint 编码。
- `bool`:varint,`true`=1、`false`=0。

### 9.2 字段 key

每个字段前导一个 varint 编码的 key:`key = (field_number << 3) | wire_type`。

wire type:

| wire type        | 值  | 用途                           |
| ---------------- | --- | ------------------------------ |
| varint           | 0   | int32/int64/uint32/uint64/bool |
| fixed64          | 1   | double                         |
| length-delimited | 2   | string/bytes/message           |
| fixed32          | 5   | float                          |

### 9.3 各类型编码

| 类型                                                    | 编码                                                   |
| ------------------------------------------------------- | ------------------------------------------------------ |
| int32/int64/uint32/uint64/bool                          | wire type 0,varint                                     |
| float                                                   | wire type 5,4 字节 little-endian IEEE 754              |
| double                                                  | wire type 1,8 字节 little-endian IEEE 754              |
| string/bytes                                            | wire type 2,varint 长度 + 内容                         |
| message                                                 | wire type 2,varint 长度 + 内层消息字节                 |
| 标量 repeated(int32/64、uint32/64、bool、float、double) | packed:单个 key(wire type 2)+ varint 总长度 + 连续元素 |
| string/bytes/message 的 repeated                        | 逐元素:每个元素一个 key(wire type 2)+ 长度 + 内容      |

- 未设置的字段不编码;已设置的字段编码(即使值为零值)。`message` 字段为 nil 视为未设置。

### 9.4 解码校验

- 字段号为 0 或越界 → `ErrMalformedData`。
- 非法 wire type(6/7)→ `ErrMalformedData`。
- 未知字段号(不在 schema 中)→ 按 wire type 跳过该字段(向后兼容:新增字段不破坏旧解析器)。
- 已知字段号但 wire type 与字段类型不符 → `ErrMalformedData`(防止修改已有字段类型的兼容性变更)。
- 标量 repeated 字段同时接受 packed(wire type 2)与 unpacked(原 wire type)两种形式,可混用。
- 字段未出现在数据中 → 保持未设置(presence = false);字段出现 → 设置为解码值(presence = true)。
- 0 字节输入是**合法空消息**的编码,解码成功(全部字段未设置),不报错。
- 数据在字段中间被截断(长度声明超出实际剩余)→ `ErrTruncated`。
