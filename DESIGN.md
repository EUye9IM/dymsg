# msgcodec:代码 Agent 评测任务设计文档

> 目标:设计一个用于评测**现成代码 agent** 的 Go 编程任务。
> 核心指标:**正确性优先**(性能为加分项)。
> 执行方式:手动执行单个任务。
> 验证方式:**混合** —— 自动化测试(`go test` / `-race` / `go vet`)+ 人工审阅 diff。
> 任务形态:**骨架 + TODO**(提供接口契约,核心逻辑由 agent 填充)。

---

## 1. 定位

**schema-driven 动态消息编解码框架**:消息结构来自 JSON 配置文件,不依赖 Go 反射。核心能力:

1. **注册消息类型**:从配置文件加载 schema,绑定类型 ID。
2. **动态取值/赋值**:对消息字段做点路径 `Get` / `Set`,支持嵌套路径与 repeated 下标。
3. **双序列化**:同一消息既可按 **JSON** 编码,也可按 **Protobuf wire format** 编码。
4. **深拷贝**:`Set("", m)` 复制整个消息,`Set("field", msg)` 复制嵌套消息。

消息结构是运行时才知道的,因此字段值不总能退化为原生 Go 类型(嵌套字段无对应 Go struct),故统一以 `Message` 接口表示。

## 2. 目录结构

```
msgcodec/
├── README.md          # 任务描述(issue 风格,给 agent;并发零提示)
├── SPEC.md            # 设计约定文档(给 agent,中性事实:语义/格式/错误契约)
├── go.mod
├── errors.go          # 哨兵错误(骨架提供)
├── schema.go          # FieldType/FieldSchema/MessageSchema/ParseSchema(骨架 + TODO)
├── registry.go        # Register/New + schema 注册表缓存(骨架 + TODO,并发考点)
├── message.go         # Message 接口契约(骨架提供)
├── codec_test.go      # 公开测试(给 agent 看的部分)
├── golden_test.go     # 隐藏黄金测试(评测用,agent 看不到)
└── reference.md       # 参考实现 + 评分说明(评测人用)
```

## 3. 接口契约

### 3.1 哨兵错误(`errors.go`)

```go
var (
    ErrDuplicateID    // 重复注册同一 typeID
    ErrUnknownTypeID  // 遇到未注册 typeID
    ErrFieldNotFound  // 字段不存在
    ErrIndexOutOfRange// repeated 下标越界
    ErrTypeMismatch   // 赋值类型无法转换(含数值溢出)
    ErrMalformedData  // 编解码数据格式错误(含字段号越界/重复)
    ErrTruncated      // 数据被截断
)
```

### 3.2 Schema(`schema.go`)

```go
type FieldType string

const (
    FieldInt32   FieldType = "int32"
    FieldInt64   FieldType = "int64"
    FieldUint32  FieldType = "uint32"
    FieldUint64  FieldType = "uint64"
    FieldFloat   FieldType = "float"
    FieldDouble  FieldType = "double"
    FieldBool    FieldType = "bool"
    FieldString  FieldType = "string"
    FieldBytes   FieldType = "bytes"
    FieldMessage FieldType = "message"
)

type FieldSchema struct {
    Name     string         // 字段名,同时作为 JSON key
    Num      int            // proto 字段号,范围 [1, 65535]
    Type     FieldType
    Repeated bool           // 是否为数组
    Schema   MessageSchema  // Type == message 时指向内联嵌套 schema
}

type MessageSchema interface {
    TypeID() uint16                          // 顶层注册类型返回 typeID;内联嵌套返回 0
    Fields() []*FieldSchema                  // 按声明顺序
    Field(name string) (*FieldSchema, bool)  // 按字段名查找
}

func ParseSchema(data []byte) ([]MessageSchema, error) // 解析配置文件(JSON)
```

### 3.3 注册与构造(`registry.go`)

```go
func Register(s MessageSchema) error    // 重复 typeID → ErrDuplicateID
func New(typeID uint16) (Message, error) // 未注册 → ErrUnknownTypeID
```

### 3.4 动态消息接口(`message.go`)

```go
type Message interface {
    Get(field string) (Message, error)
    Set(field string, value any) error
    Value() any
    EncodeJSON() ([]byte, error)
    EncodeProto() ([]byte, error)
    DecodeJSON(data []byte) error
    DecodeProto(data []byte) error
}
```

## 4. 语义规范

每个字段具有**存在性(presence)**:区分「未设置」与「显式设置为某值(含零值)」。未设置字段在 Proto/JSON 编码中不出现;已设置字段出现(即使值为零值)。`Get` 未设置字段返回 nil Message;`Set(field, nil)` 清除为未设置。

### 4.1 字段路径(字符串)

| 形式 | 含义 |
|------|------|
| `""` | 当前消息自身 |
| `"name"` | 字段名 |
| `"addr.city"` | 嵌套字段(点分隔) |
| `"tags[0]"` | repeated 字段的第 0 个元素 |
| `"items[0].name"` | 下标 + 嵌套组合 |

### 4.2 Get

- `Get("")` 返回自身;`Get("field")` 返回字段的 Message;`Get("field[n]")` 返回第 n 个元素
- 嵌套用点路径 `Get("addr.city")`;字段不存在 → `ErrFieldNotFound`;字段存在但未设置 → 返回 nil Message;下标越界 → `ErrIndexOutOfRange`

### 4.3 Set(均深拷贝)

- `Set("", m)` 深拷贝覆盖自身(复制);`Set(field, nil)` 清除字段(未设置)
- 标量字段:传原生值,折中转换(见 4.5)
- 嵌套字段:传 `Message`
- repeated 字段:传 `[]any`(标量)或 `[]Message`(消息);`Set("tags", make([]Message, x))` 得到 x 个空消息的数组
- 下标形式:`Set("tags[0]", v)` 设置单个元素

### 4.4 Value

- 标量消息 → 原生值(`int32`/`string`/…);复合消息 → 自身 `Message`
- repeated → `[]any`(标量元素)或 `[]Message`(消息元素)

### 4.5 折中类型转换规则(标量 Set 的判定依据)

| 场景 | 行为 |
|------|------|
| 类型完全相同 | 直接赋值 |
| 数值互转(int/uint/float 变体) | 转换,溢出报 `ErrTypeMismatch` |
| string ↔ 数值 | `strconv` 解析,失败报 `ErrTypeMismatch` |
| []byte ↔ string | 允许 |
| 底层类型相同的别名类型 | 允许 |
| 其他(struct ↔ int 等) | 报 `ErrTypeMismatch` |

### 4.6 深拷贝

`Set("", m)` 与 `Set("field", msg)` 均**深拷贝**,保证目标对象与源对象完全独立;浅拷贝(共享底层)视为缺陷。

## 5. 配置文件格式(JSON)

```json
{
  "types": [
    {
      "typeId": 1001,
      "fields": [
        {"name": "name", "type": "string", "num": 1},
        {"name": "age",  "type": "int32",  "num": 2},
        {"name": "addr", "type": "message", "num": 3, "schema": {
          "fields": [
            {"name": "city", "type": "string", "num": 1},
            {"name": "zip",  "type": "string", "num": 2}
          ]
        }},
        {"name": "tags", "type": "string", "num": 4, "repeated": true}
      ]
    }
  ]
}
```

- 嵌套 schema 内联表达,不单独注册、无独立 typeID
- `typeId` 仅顶层类型有,范围 [1, 65535]
- 字段 `num` 同一 struct 内唯一、范围 [1, 65535]

## 6. 编解码格式

### 6.1 JSON

- JSON key = 字段 `Name`
- 嵌套消息编码为嵌套对象;repeated 编码为数组

### 6.2 Proto wire format(自实现基础子集)

> 决策:不用 `google.golang.org/protobuf`,自实现基础子集,零外部依赖。

| FieldType | wire type | 编码 |
|-----------|-----------|------|
| int32/int64/uint32/uint64 | 0 (varint) | 变长整数 |
| float/double | 5/1 (fixed32/fixed64) | 定长 little-endian |
| bool | 0 (varint) | 0/1 |
| string/bytes | 2 (length-delimited) | 长度前缀 + 字节 |
| message | 2 (length-delimited) | 长度前缀 + 内层消息字节 |
| 标量 repeated | 2 (packed) | 单个 key + varint 总长度 + 连续元素 |
| string/bytes/message repeated | 2 (length-delimited) | 逐元素编码 |

- 字段 key = `(field_number << 3) | wire_type`
- 编码端:标量 repeated 用 packed;`string/bytes/message` 的 repeated 逐元素编码
- presence:未设置字段不编码;已设置字段编码(含零值);message 为 nil 视为未设置
- 解码端:未知字段号跳过(向后兼容);wire type 与字段类型不符 → `ErrMalformedData`;标量 repeated 同时接受 packed 与 unpacked(可混用);非法 wire type(6/7)与截断报错;字段缺失 → 保持未设置

## 7. 骨架提供 vs 留空(TODO)分工

| 文件 | 骨架提供 | 留空(TODO) |
|------|---------|------------|
| errors.go | 7 个哨兵错误 | — |
| schema.go | FieldType/FieldSchema/MessageSchema 接口、ParseSchema 签名 | ParseSchema 实现 |
| registry.go | Register/New 签名 | 注册表与运行时描述缓存的设计与实现(含并发安全) |
| message.go | Message 接口契约(含语义注释) | — |

设计原则:接口契约给足,内部实现(解析、缓存、动态消息存储、编解码)全交 agent。

## 8. 评测点设计(三层考点,并发场景暗示)

| 层 | 考点 | 验证 |
|----|------|------|
| **契约层** | 重复 typeID → `ErrDuplicateID`;未知 typeID → `ErrUnknownTypeID`;Get 不存在字段;下标越界;空输入报错 | golden test 断言哨兵错误 |
| **边界/安全层** | proto 字段号越界/重复;wire type 不匹配;非法 wire type(6/7);折中转换溢出;截断数据;下标负值/越界 | golden test 喂恶意输入,断言报错不崩 |
| **并发层** | schema 注册表 + 惰性构建的运行时描述缓存,并发首次 `New`/`Get` → 并发写 map | golden 并发用例 + `go test -race` |

### 并发提示策略(关键)

- README 与 SPEC 均**不直接出现**"并发"、"线程安全"、"-race" 等字眼
- 通过**使用场景**暗示:README 描述"长期驻留的线上服务,运行期间**不停机地注册新的消息类型**,与此同时服务持续处理已注册消息的编码/解码/字段访问"——暗示 `Register` 可能与消息处理并发
- README 验收标准只写:`go test ./... 通过`(不写 `-race`)
- 评测方用 `go test -race ./...` 验收,agent 不知情
- 骨架中的注册表**不加锁、不预置 sync.Map**,让 agent 自行决定
- 效果:agent 大概率写无锁版本(单测全绿)→ golden 并发用例触发 panic/竞态 → 评测其回修能力

## 9. 隐藏测试覆盖矩阵(golden_test.go)

```
契约层:
  - Register 重复 typeID → ErrDuplicateID
  - New 未注册 ID → ErrUnknownTypeID
  - Get 不存在字段 → ErrFieldNotFound
  - Get("tags[999]") 下标越界 → ErrIndexOutOfRange
  - DecodeJSON/DecodeProto 空输入 → 报错(非静默成功)
边界层:
  - proto 字段号越界/重复 → ErrMalformedData
  - wire type 与字段类型不匹配 → ErrMalformedData
  - 非法 wire type(6/7)→ ErrMalformedData
  - 未知字段号 → 跳过,不影响其余字段解析
  - 截断的 proto payload → ErrTruncated
  - Set 给 int32 字段塞 string → ErrTypeMismatch;数值溢出 → ErrTypeMismatch
  - Set 下标负值 → ErrIndexOutOfRange
功能层:
  - JSON 往返:EncodeJSON → DecodeJSON → Get 取值一致
  - Proto 往返:EncodeProto → DecodeProto → Get 取值一致
  - 嵌套字段:Set("addr.city") → Get("addr.city") → 编解码后仍一致
  - repeated 往返:Set([]any) / make([]Message, x) / 下标访问
  - 标量 repeated packed 编码 → 解码还原
  - unpacked 标量 repeated 输入 → 正确解析
  - packed 与 unpacked 混用 → 正确 append 全部元素
  - string/bytes/message 的 repeated 往返
  - presence:未设置字段 Get → nil,编码后字段不出现
  - presence:显式 Set 零值 → 编码后字段出现
  - Set(field, nil) 清除 → 回到未设置
  - 零值/未设置经 Proto、JSON 往返后保持区分
  - 深拷贝:Set("", m1) 后修改 m2 不影响 m1;嵌套 Set 同理
并发层(配合 -race):
  - 多 goroutine 并发首次 New/Get 不同类型 → 不 panic、无竞态
  - 并发 Register 与 New 同时进行 → 无竞态
```

## 10. 公开测试(codec_test.go)设计

给 agent 的测试,仅覆盖**单线程基本功能**:
- 一个 schema 加载 + 注册 + New
- JSON 往返 / Proto 往返各一例
- 基本 Get/Set(含嵌套、repeated 简单用例)
- **不含**:并发用例、恶意输入、注册冲突断言、下标越界断言、-race

## 11. 评分标准(混合)

| 等级 | 自动判定 | 人工判定 |
|------|---------|---------|
| 通过 | `go test` + `go test -race` 全绿 | diff 干净、无 hack、未退化成串行 |
| 部分 | 部分测试通过(按比例) | 契约/边界做对但未处理并发 |
| 失败 | 测试失败 / 编译失败 | 误改无关代码 / 硬编码返回值 / 吞掉错误 / 浅拷贝 |

人工审阅聚焦三问:
1. diff 是否只改了必要代码(过度改动扣分)?
2. 是否自己写了/跑了验证(工程习惯)?
3. 是否 hack(如让并发测试退化成串行、强制 `-race` 失效、浅拷贝冒充深拷贝)?

### 性能加分项(不设硬分数)

- 静态审查(人工):运行时描述是否缓存、字段查找是否 O(1)、编解码是否避免重复解析
- 可选 benchmark(golden 内):记录 `allocs/op` 作参考

## 12. README 与 SPEC 写作要点

**README.md**(issue 风格,给 agent):
- 描述"线上服务动态定义消息结构"的使用场景,自然带出"不停机注册新类型 + 同时处理消息"的并发暗示
- 约束:纯 Go(不用 cgo/汇编)、性能尽可能好(避免重复解析 schema、无谓分配)
- 指向 SPEC.md 为唯一权威约定
- 验收标准仅写:"`go test ./...` 通过"(不写 `-race`)
- 不直接出现"并发"、"线程安全"等字眼

**SPEC.md**(中性事实,给 agent):
- 接口签名、哨兵错误、字段路径语法、折中转换规则、深拷贝要求、配置文件格式、wire format 完整规格、字段号上限
- 只陈述功能契约,**不陈述任何并发/性能要求**

## 13. 参考解答(reference.md)大纲

- 正确实现全文(用于自测与对照)
- 每层的正确做法、常见错误方向(无锁 map、浅拷贝、忽略恶意输入、吞错误、硬编码)
- 并发修复分层:`sync.RWMutex`(及格)→ `sync.Map`(良好)→ 预构建/写时复制(优秀)
- 性能加分点说明

## 14. 实施步骤

1. 写骨架:`errors.go`、`schema.go`、`registry.go`、`message.go`(已完成)
2. 写**正确版核心实现**(ParseSchema / Register / New / 动态消息存储 / 编解码,即参考答案)
3. 写 golden_test.go(验证:正确版全绿、无锁版确实红 → 保证区分度)
4. 写 codec_test.go(公开)
5. 写 SPEC.md + README.md
6. 写 reference.md
7. 自检:用强 agent 跑一遍,确认可完成 / 难度合理 / 无歧义 / 无法 hack 绕过

---

## 已定决策汇总

- [x] schema-driven,消息结构来自 JSON 配置文件,不依赖 Go 反射
- [x] MessageSchema 定义为接口(内部实现自由);FieldSchema 为 struct
- [x] JSON key 直接用字段 Name,不单设字段
- [x] Message 为接口;Get/Set 空字符串表示自身;深拷贝
- [x] repeated 支持下标 `field[n]` 与 `make([]Message, x)`
- [x] 下标越界新增 `ErrIndexOutOfRange`
- [x] 嵌套 schema 内联表达,无独立 typeID
- [x] 裸编解码不做,直接用 `m.EncodeJSON`/`m.EncodeProto`
- [x] proto wire 自实现基础子集;字段号范围 [1, 65535]
- [x] 标量 repeated 编码用 packed;解码兼容 packed/unpacked 混用
- [x] 未知字段号跳过(向后兼容);wire type 不匹配报错;非法 wire type(6/7)报错
- [x] presence 语义:区分未设置与零值;`Set(field,nil)` 清除;JSON null=未设置;未设置在 proto/JSON 均不出现
- [x] 性能作为加分项,不设硬分数
- [x] 并发采用场景暗示(README 描述不停机注册场景;不提"并发"/"-race";验收只写 `go test ./...`)
