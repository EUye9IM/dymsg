# dymsg:动态消息编解码库

## 背景

某在线服务需要在运行时动态定义消息结构:消息的 schema 来自配置文件,而非编译期写死的 Go 结构体。不同服务间通过 JSON 或 Protobuf 交换这些消息,服务内部则需要对这些消息做字段级的读取与写入。

## 使用场景

该库运行在一个长期驻留的线上服务进程中。服务启动时从配置文件加载消息 schema,并且可能在运行期间**不停机地注册新的消息类型**——例如运维通过下发新配置,动态扩展可识别的消息结构。与此同时,服务持续处理已注册消息的编码、解码与字段访问。

## 需求

在`/workspace`中实现 `dymsg` 包,提供以下能力:

1. **动态注册**:从 JSON 配置文件解析消息 schema,并按类型 ID 注册。
2. **字段访问**:按字段路径对消息取值 / 赋值,支持嵌套字段与 repeated 下标;repeated 支持追加、求长与元素访问。
3. **双编解码**:同一消息可编码为 JSON 或 Protobuf wire format,并可解码还原。
4. **深拷贝**:支持消息整体复制,且与源消息完全独立。

具体接口契约与语义规范见后文规范,它是唯一权威约定，需要有测试和注释。

## 约束

- 纯 Go 语言实现,不依赖 cgo 或汇编。
- 性能尽可能好:字段访问与编解码应避免不必要的开销。

## 验收标准

`go test ./...` 全部通过。

# dymsg 规范

本文件是 `dymsg` 包的**唯一权威约定**。实现须严格遵循本规范,测试将据此验收。

## 1. 概述

`dymsg` 是一个动态消息编解码库:消息结构由配置文件定义(而非编译期写死的 Go 结构体),运行时按类型 ID 注册,支持字段级取值/赋值与 JSON / Protobuf 双编解码。

设计要点:

- **读写分离**:读用 `Get` 返回一个**值对象 `Value`**(一次调用拿到值,无二次解包);写用 `Set` / `Append` / `Clear` 返回 `error`。
- **显式存在性(presence)**:「字段不存在」「字段存在但未设置」「字段已设置(含零值)」三种状态可被 `Value` 明确区分,不再用 `nil` 一值多义。
- **repeated 一等公民**:通过 `Append` 追加、`Len` 求长、`Index` 取元素,无需手工预分配数组。

## 2. 核心类型与接口

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
```

### 2.2 字段定义(`FieldSchema`)

```go
type FieldSchema struct {
    Name     string         // 字段名,同时作为 JSON key
    Num      int            // proto 字段号,范围 [1, 65535]
    Type     FieldType
    Repeated bool           // 是否为数组
    Schema   MessageSchema  // Type == message 时指向内联嵌套 schema
}
```

### 2.3 消息 schema(`MessageSchema`)

```go
type MessageSchema struct {
    // 内部字段由实现者自由设计,但必须能支撑第 5、6、9、10 节的取值与编解码语义。
}
```

- `ParseSchema` 返回的每个 `MessageSchema` 代表一个顶层消息类型。

### 2.4 消息(`Message`)

```go
type Message struct {
    // 内部字段由实现者自由设计。
}

func (m *Message) Get(path string) Value
func (m *Message) Set(path string, value any) error
func (m *Message) Append(path string, value any) error
func (m *Message) Clear(path string) error
func (m *Message) Has(path string) bool
func (m *Message) SetFields() []string
func (m *Message) EncodeJSON() ([]byte, error)
func (m *Message) EncodeProto() ([]byte, error)
func (m *Message) DecodeJSON(data []byte) error
func (m *Message) DecodeProto(data []byte) error
```

`Message` 只表示一个**结构化消息实例**(拥有 schema 与字段)。取值结果一律用 `Value` 表达(见 2.5),`Message` 不再兼任「值包装节点」。

### 2.5 取值结果(`Value`)

`Get` 返回一个值对象 `Value`,用于读取字段/元素的值、存在性与错误。

```go
type Value struct {
    // 内部字段由实现者自由设计。
}

func (v Value) Exists() bool      // 路径是否合法且字段存在
func (v Value) IsSet() bool       // 字段是否已设置(presence)
func (v Value) Err() error        // 路径解析/查找错误;成功时为 nil
func (v Value) Any() any          // 原生值(未设置/不存在时为 nil)

func (v Value) String() string
func (v Value) Int32() int32
func (v Value) Int64() int64
func (v Value) Uint32() uint32
func (v Value) Uint64() uint64
func (v Value) Float32() float32
func (v Value) Float64() float64
func (v Value) Bool() bool
func (v Value) Bytes() []byte
func (v Value) Message() *Message // 嵌套消息(含 repeated 消息元素)

func (v Value) Len() int          // repeated 数组长度;非 repeated/未设置/不存在为 0
func (v Value) Index(i int) Value // 数组第 i 个元素

func (v Value) Strings() []string
func (v Value) Int32s() []int32
func (v Value) Int64s() []int64
func (v Value) Uint32s() []uint32
func (v Value) Uint64s() []uint64
func (v Value) Float32s() []float32
func (v Value) Float64s() []float64
func (v Value) Bools() []bool
func (v Value) BytesSlice() [][]byte
func (v Value) Messages() []*Message
```

### 2.6 注册与构造

```go
func ParseSchema(data []byte) ([]MessageSchema, error)
func Register(s MessageSchema) error
func New(typeID uint16) (*Message, error)
```

`Register` 按类型 ID 注册,`ParseSchema` 得到的 schema 必须先注册才能 `New`。语义:

- 重复注册**相同内容**(相同 typeID 且字段定义一致)是幂等的,成功返回 `nil`;重复注册**不同内容**的同一 typeID → `ErrDuplicateID`。
- `typeId` 必须位于 [1, 65535];`typeId` 为 0 或越界 → `ParseSchema` 返回 `ErrMalformedData`。
- `Register` 与 `New` 均并发安全。

### 2.7 哨兵错误

```go
var (
    ErrDuplicateID     // 重复注册不同内容的同一 typeID
    ErrUnknownTypeID   // 遇到未注册 typeID
    ErrFieldNotFound   // 字段不存在 / 路径非法(如对非 repeated 字段使用下标)
    ErrIndexOutOfRange // repeated 下标越界(负下标、非数值或超出数组长度)
    ErrTypeMismatch    // 赋值/追加类型无法转换(含数值溢出、schema 不匹配)
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
- 每个字段:`name`(必填,同时作为 JSON key)、`type`(必填,取值见 2.1)、`num`(必填,proto 字段号,范围 [1, 65535])。
- `repeated`(可选,默认 false)表示字段为数组。
- `schema` 仅当 `type == "message"` 时出现,内联定义嵌套消息结构;嵌套 schema 无 `typeId`。
- 同一消息类型内字段 `num` 必须唯一;字段 `name` 必须唯一。
- 配置非法(JSON 语法错误、类型名非法、typeId/num 越界或重复、`name` 含 `.`/`[`/`]`、非 message 字段带 `schema`、message 字段缺 `schema`)时,`ParseSchema` 返回 `ErrMalformedData`。

## 4. 字段路径语法

`Get` / `Set` / `Append` / `Clear` / `Has` 的 `path` 参数是一个路径表达式:

| 形式              | 含义                    |
| ----------------- | ----------------------- |
| `""`              | 当前消息自身            |
| `"name"`          | 字段名                  |
| `"addr.city"`     | 嵌套字段(`.` 分隔)      |
| `"tags[0]"`       | repeated 字段的下标元素 |
| `"items[0].name"` | 下标 + 嵌套组合         |

规则与异常:

- 路径用 `.` 分隔字段名,`[n]` 表示下标(`n` 为非负十进制整数)。
- 字段名不得包含 `.`、`[`、`]` 字符(配置解析阶段即拒绝,见第 3 节)。
- 下标 `n` 必须为非负十进制整数:
  - 负下标(`tags[-1]`)、非数值(`tags[abc]`)、空下标(`tags[]`)→ `ErrIndexOutOfRange`。
  - 下标后不是 `]` 结尾(如 `tags[0`、`tags[0]x`)→ 非法路径 → `ErrFieldNotFound`。
- 下标仅适用于 repeated 字段;对非 repeated 字段使用下标(如 `name[0]`)→ `ErrFieldNotFound`。
- 对非 message 字段继续下钻子路径(如 `name.x`、`tags[0].x`)→ `ErrFieldNotFound`。

## 5. 读取:Get 与 Value

### 5.1 `Get` 语义

`Get(path string) Value` 返回路径所指字段的取值结果,**不返回 error**;错误内嵌在 `Value.Err()` 中,便于链式读取。

- `Get("")` 返回当前消息自身(`v.Message()` 即 `*Message` 自身,`v.Any()` 亦为自身)。
- 路径非法或字段不存在 → `Value.Err()` 非 nil,`Exists()` 为 false。
- 字段存在但未设置 → `Value.Err()` 为 nil,`Exists()` 为 true,`IsSet()` 为 false。
- 字段已设置 → `Exists()` 为 true,`IsSet()` 为 true。

### 5.2 `Value` 状态模型

`Value` 用三个正交状态区分不同情形:

| 情形                        | `Exists()` | `IsSet()` | `Err()`            | 类型 getter |
| --------------------------- | ---------- | --------- | ------------------ | ----------- |
| 字段不存在 / 路径非法       | false      | false     | 对应哨兵错误       | 零值        |
| 下标越界 / 非法下标         | false      | false     | `ErrIndexOutOfRange` | 零值      |
| 字段存在但未设置            | true       | false     | nil                | 零值        |
| 字段已设置(含零值)          | true       | true      | nil                | 实际值      |

### 5.3 `Value` 类型化访问器

- 各类型 getter(`String`/`Int32`/… )在「未设置 / 不存在 / 类型不符」时返回对应类型的**零值**;调用方可用 `IsSet()`/`Err()` 区分。
- `Any()` 返回原生 Go 值:标量返回原生类型,嵌套消息返回 `*Message`,repeated 返回对应切片(`[]T` / `[][]byte` / `[]*Message`);未设置/不存在时返回 nil。
- `Message()` 返回嵌套消息的 `*Message`;仅当字段为 `message` 类型且已设置时非 nil。
- 类型 getter 与字段类型的对应关系见 2.1 表;对 `message` 字段调用标量 getter 返回零值(应使用 `Message()`)。

### 5.4 repeated 访问

- `v.Len()` 返回数组长度;仅当字段为 repeated 且已设置时 > 0,其余情形为 0。
- `v.Index(i)` 返回第 `i` 个元素的 `Value`;`i` 越界 → `Value.Err() == ErrIndexOutOfRange`、`Exists() == false`。
- 标量 repeated 切片 getter(如 `Int32s()`)返回整个数组;未设置/不存在时返回 nil。
- 消息 repeated 通过 `v.Messages()` 返回 `[]*Message`,或 `v.Index(i).Message()` 取单个元素。

## 6. 写入:Set / Append / Clear

所有写入均采用**深拷贝**(见第 8 节)。

### 6.1 `Set(path, value any) error`

按路径整体赋值。

- `Set("", m)`:用 `value` 覆盖当前消息内容(整体复制);`value` 须为同构的 `*Message`,否则 `ErrTypeMismatch`。
- `Set("", nil)` 或 `Set(path, nil)`:清除(置未设置),等价于 `Clear`(见 6.3)。
- 标量字段:`value` 为对应原生类型或可转换类型(见第 7 节)。
- 嵌套字段(`message`):`value` 为 `*Message`(须 schema 同构)。
- repeated 字段:`value` 为对应切片(`[]T`、`[][]byte` 或 `[]*Message`);`[]*Message` 中允许 nil 元素(表示该位置为空,见 9.3/10.4)。
- 下标形式 `Set("tags[0]", v)`:设置单个元素;仅 repeated 字段允许下标,且要求数组已存在(否则 `ErrIndexOutOfRange`)。
- 中间路径为 message 且未设置时,`Set` 会**自动创建**中间嵌套消息;中间路径为标量或标量 repeated 时 → `ErrFieldNotFound`。

错误:`ErrFieldNotFound`(字段不存在 / 非法路径)、`ErrIndexOutOfRange`(下标越界)、`ErrTypeMismatch`(类型无法转换 / 数值溢出 / schema 不匹配)。

### 6.2 `Append(path, value any) error`

向 repeated 字段**追加**一个元素。

- `path` 指向 repeated 字段(不带下标);若字段未设置,先自动创建空数组再追加。
- 元素转换规则同 `Set` 单个元素:标量元素可折中转换(第 7 节),消息元素须 schema 同构的 `*Message`。
- 字段不存在 → `ErrFieldNotFound`。
- 字段非 repeated → `ErrTypeMismatch`(追加目标不是数组)。
- 元素转换失败 / 数值溢出 / schema 不匹配 → `ErrTypeMismatch`。

### 6.3 `Clear(path) error`

将字段置为未设置(presence = false)。

- `Clear("")` 清除整个消息的全部字段。
- 字段不存在 → `ErrFieldNotFound`。
- 对已未设置的字段调用 `Clear` 是幂等的(不报错)。

### 6.4 presence 查询

- `Has(path) bool`:字段存在且已设置时返回 true;字段不存在、路径非法或未设置均返回 false。
- `SetFields() []string`:返回当前**已设置**字段名,按 schema 声明顺序;无字段时返回空切片。

## 7. 折中类型转换(标量 Set / Append)

| 场景                          | 行为                                              |
| ----------------------------- | ------------------------------------------------- |
| 类型完全相同                  | 直接赋值                                          |
| 数值互转(int/uint/float 变体) | 转换;溢出报 `ErrTypeMismatch`                     |
| 浮点 → 整数                   | 截断(向零取整);NaN/Inf 或越界报 `ErrTypeMismatch` |
| string ↔ 数值                 | 用 `strconv` 解析/格式化;失败报 `ErrTypeMismatch` |
| []byte ↔ string               | 允许                                              |
| 底层类型相同的别名类型        | 允许                                              |
| 其他                          | 报 `ErrTypeMismatch`                              |

## 8. 深拷贝

`Set("", m)`、`Set("field", msg)`(嵌套)、`Set`/`Append` 的 repeated 消息数组均执行**深拷贝**:目标与源此后完全独立,修改一方不影响另一方。

## 9. JSON 编解码

### 9.1 编码(`EncodeJSON`)

- JSON key 使用字段 `name`;字段按 schema 声明顺序输出。
- 各类型映射:整数/浮点 → JSON number;`bool` → JSON bool;`string` → JSON string;`bytes` → base64 字符串;`message` → 嵌套 JSON 对象;repeated → JSON 数组。
- 未设置字段不输出(缺 key);已设置字段输出(含零值);非 repeated 的 `message` 字段为 nil 视为未设置。
- repeated 消息数组中的 nil 元素编码为 `null`。
- 浮点值为 NaN 或 ±Inf 时编码失败(JSON 无法表示)。

### 9.2 解码(`DecodeJSON`)

- **整体替换**:解码前清空所有字段,再按输入填充;输入中未出现的字段解码后为未设置。
- 忽略未知 JSON key。
- 顶层输入必须是 JSON 对象;顶层 `null` 为特殊情形(见 9.3)。
- 空输入 → `ErrTruncated`。

### 9.3 `null` 处理

| 输入                         | 行为                                   |
| ---------------------------- | -------------------------------------- |
| 字段值整体为 `null`(任意类型) | 该字段置为未设置                       |
| 顶层为 `null`                | 清空整个消息的全部字段(等价 `Clear("")`) |
| 消息 repeated 数组内的 `null` 元素 | 保留为 nil 元素                     |
| 标量 repeated 数组内的 `null` 元素 | `ErrMalformedData`                |

### 9.4 错误与边界

- JSON 语法错误 / 顶层非对象(数组、字符串、数字)→ `ErrMalformedData`。
- 字段值与 schema 类型不符(如字符串给数值字段、数组给标量字段、标量给 repeated 字段)→ `ErrMalformedData`。
- 数值溢出超出字段范围 → `ErrMalformedData`。
- `bytes` 字段 base64 解码失败 → `ErrMalformedData`。
- 输入结尾有多余内容 → `ErrMalformedData`。
- 空输入(0 字节)→ `ErrTruncated`。

## 10. Protobuf wire format

自实现的基础子集,不依赖 `google.golang.org/protobuf`。

### 10.1 varint

无符号 64 位整数编码为变长字节序列:每个字节低 7 位承载数据,最高位(MSB)为 1 表示还有后续字节、为 0 表示结束。

- `uint32`/`uint64`:直接 varint 编码。
- `int32`/`int64`:按 int64 补码对应的 uint64 varint 编码。
- `bool`:varint,`true`=1、`false`=0。

### 10.2 字段 key 与 wire type

每个字段前导一个 varint 编码的 key:`key = (field_number << 3) | wire_type`。

| wire type        | 值  | 用途                           |
| ---------------- | --- | ------------------------------ |
| varint           | 0   | int32/int64/uint32/uint64/bool |
| fixed64          | 1   | double                         |
| length-delimited | 2   | string/bytes/message           |
| fixed32          | 5   | float                          |

### 10.3 各类型编码

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
- repeated 消息数组中为 nil 的元素不编码(跳过)。

### 10.4 解码与校验

- **整体替换**:解码前清空所有字段,再按输入填充;输入中未出现的字段解码后为未设置。
- 字段号为 0 → `ErrMalformedData`;未知字段号(不在 schema 中)→ 按 wire type 跳过该字段(向后兼容)。
- 非法 wire type(6/7)→ `ErrMalformedData`。
- 已知字段号但 wire type 与字段类型不符 → `ErrMalformedData`。
- 标量 repeated 字段同时接受 packed(wire type 2)与 unpacked(原 wire type)两种形式,可混用(逐个追加)。
- 0 字节输入是**合法空消息**的编码,解码成功(全部字段未设置),不报错。
- 数据在字段中间被截断(长度声明超出实际剩余)→ `ErrTruncated`。

## 11. 快速示例

```go
schemas, _ := dymsg.ParseSchema(cfg)
for _, s := range schemas {
    dymsg.Register(s)
}
m, _ := dymsg.New(1001)

m.Set("name", "alice")
m.Set("addr.city", "beijing")       // 自动创建中间嵌套消息
m.Append("tags", "a")
m.Append("tags", "b")

name := m.Get("name").String()       // "alice"
city := m.Get("addr.city").String()  // "beijing"
for i := 0; i < m.Get("tags").Len(); i++ {
    println(m.Get("tags").Index(i).String())
}
if v := m.Get("age"); v.IsSet() {    // 未设置时 IsSet 为 false
    println(v.Int32())
}
```

## 12. 边缘与异常场景速查表

### 12.1 路径解析

| 输入            | `Get`/`Value.Err()`          | `Set` 错误            |
| --------------- | ---------------------------- | --------------------- |
| `""`            | 自身(无错误)                 | 覆盖/清除自身         |
| `unknown`       | `ErrFieldNotFound`           | `ErrFieldNotFound`    |
| `name[0]`       | `ErrFieldNotFound`           | `ErrFieldNotFound`    |
| `tags[-1]`      | `ErrIndexOutOfRange`         | `ErrIndexOutOfRange`  |
| `tags[abc]`     | `ErrIndexOutOfRange`         | `ErrIndexOutOfRange`  |
| `tags[]`        | `ErrIndexOutOfRange`         | `ErrIndexOutOfRange`  |
| `name.x`        | `ErrFieldNotFound`           | `ErrFieldNotFound`    |
| `addr.city.x`   | `ErrFieldNotFound`           | `ErrFieldNotFound`    |

### 12.2 Get / Value

| 情形                          | 结果                                                        |
| ----------------------------- | ----------------------------------------------------------- |
| 未设置字段                    | `Exists=true`、`IsSet=false`、getter 返回零值               |
| 不存在字段                    | `Exists=false`、`Err=ErrFieldNotFound`、getter 返回零值      |
| 未设置 nested message         | `Message()==nil`                                            |
| 未设置 repeated               | `Len()==0`、切片 getter 返回 nil                            |
| 越界下标                      | `Err=ErrIndexOutOfRange`                                    |
| 对 message 调用标量 getter    | 零值                                                        |

### 12.3 Set

| 场景                          | 行为                                   |
| ----------------------------- | -------------------------------------- |
| `Set(path, nil)`              | 清除该字段(等价 `Clear`)               |
| `Set("", nil)`                | 清空全部字段                           |
| `Set("", m)`(不同构)          | `ErrTypeMismatch`                      |
| `Set("addr.city", v)`(addr 未设置) | 自动创建 addr 再设置 city          |
| `Set("tags[0]", v)`(tags 未设置)   | `ErrIndexOutOfRange`               |
| `Set("name[0]", v)`           | `ErrFieldNotFound`                     |
| 数值溢出                      | `ErrTypeMismatch`                      |

### 12.4 Append / Clear

| 场景                        | 行为                         |
| --------------------------- | ---------------------------- |
| `Append("tags", v)`(未设置) | 自动建空数组后追加           |
| `Append("name", v)`         | `ErrTypeMismatch`(非 repeated) |
| `Append("unknown", v)`      | `ErrFieldNotFound`           |
| `Append` 元素转换失败        | `ErrTypeMismatch`            |
| `Clear("")`                 | 清空全部字段                 |
| `Clear` 已未设置字段         | 幂等,不报错                  |
| `Clear("unknown")`          | `ErrFieldNotFound`           |

### 12.5 JSON

| 输入                        | 行为                         |
| --------------------------- | ---------------------------- |
| 空输入                      | `ErrTruncated`               |
| `null`(顶层)                | 清空全部字段                 |
| `[]` / `"x"` / `123`(顶层)  | `ErrMalformedData`           |
| `{"unknown":1}`             | 忽略未知 key                 |
| `{"name":null}`             | name 置未设置                |
| `{"scores":[1,null]}`(标量) | `ErrMalformedData`           |
| `{"contacts":[null,{}]}`    | 保留 nil 元素                |
| `{"age":"18"}`              | `ErrMalformedData`(类型不符) |
| `{"age": 2147483648}`       | `ErrMalformedData`(溢出)     |

### 12.6 Proto

| 输入                          | 行为                          |
| ----------------------------- | ----------------------------- |
| 0 字节                        | 成功,空消息                   |
| 字段号 0                      | `ErrMalformedData`            |
| 非法 wire type(6/7)           | `ErrMalformedData`            |
| wire type 与字段类型不符       | `ErrMalformedData`            |
| 未知字段号                    | 跳过(向后兼容)                |
| packed / unpacked / 混用       | 均正确解析                    |
| 截断                          | `ErrTruncated`                |

### 12.7 并发

- `Register` / `New` 并发安全(可并发注册与构造)。
- 单个 `Message` 实例**不保证**并发安全:同一实例的并发读写需调用方自行加锁。
