# msgcodec:代码 Agent 评测任务设计文档(骨架 + TODO 方案)

> 目标:设计一个用于评测**现成代码 agent** 的 Go 编程任务。
> 核心指标:**正确性优先**(性能为加分项)。
> 执行方式:手动执行单个任务。
> 验证方式:**混合** —— 自动化测试(`go test` / `-race` / `go vet`)+ 人工审阅 diff。
> 任务形态:**骨架 + TODO**(提供类型定义/签名/简单实现,核心逻辑由 agent 填充)。

---

## 1. 任务形态

不采用"埋缺陷"方案(测修复能力,需预先实现正确版再反向埋雷),采用**骨架 + TODO**方案:

- 评测方提供:`go.mod`、完整接口签名、类型定义、哨兵错误、信封常量、JSON 编解码参考实现、公开测试。
- agent 填充:`registry` 全局注册表与 `typeDesc` 缓存、反射分析、proto wire 编解码、Get/Set 嵌套路径与类型转换、信封恶意输入校验。
- 评测点不是"发现已埋的雷",而是"**是否主动把核心逻辑写对、写稳**"(并发安全、边界校验、错误契约),由隐藏测试验收。

## 2. 目录结构

```
msgcodec/
├── README.md          # 任务描述(issue 风格,给 agent;并发零提示)
├── SPEC.md            # 设计约定文档:tag/信封/wire/转换/错误契约(给 agent,中性事实)
├── go.mod
├── message.go         # Message 接口契约(Get/Set/Fields/双编解码)
├── registry.go        # 全局注册表 + 类型描述符惰性缓存  ← TODO,并发考点
├── field.go           # FieldDescriptor 定义;反射字段分析、Get/Set、折中转换  ← TODO
├── codec.go           # 信封常量;JSON(参考实现)+ proto wire + 信封  ← proto 为 TODO
├── codec_test.go      # 公开测试(给 agent 看的部分)
├── golden_test.go     # 隐藏黄金测试(评测用,agent 看不到)
└── reference.md       # 参考实现 + 评分说明(评测人用)
```

## 3. 接口契约

### 3.1 函数签名(泛型注册)

```go
package msgcodec

// Register 注册消息类型,绑定类型 ID。内部取 reflect.TypeOf((*T)(nil)).Elem()。
// 重复注册同一类型(相同 ID)= 幂等,不报错。
// 不同类型抢注同一 ID = 返回 ErrDuplicateID。
func Register[T any](typeID uint16) error

// Wrap 包装一个消息实例,后续取值/赋值/编解码都通过它。
func Wrap(v any) (Message, error)

// New 按类型 ID 创建空消息(用于反序列化)。
func New(typeID uint16) (Message, error)

// Message 是接口契约:取值/赋值/编解码能力。内部实现结构不预置,
// 由 agent 自由设计,仅以本接口为约束。
type Message interface {
	Get(field string) (any, error)     // 支持 "addr.city"
	Set(field string, value any) error
	Fields() []FieldDescriptor
	EncodeJSON() ([]byte, error)
	EncodeProto() ([]byte, error)
	DecodeJSON(data []byte) error
	DecodeProto(data []byte) error
}

// —— 裸编解码(不带信封,供嵌套/未注册类型独立序列化与观测) ——
func MarshalJSON(v any) ([]byte, error)
func UnmarshalJSON(data []byte, v any) error
func MarshalProto(v any) ([]byte, error)
func UnmarshalProto(data []byte, v any) error
```

带信封的 `Encode*/Decode*` 内部复用裸编解码。

### 3.2 哨兵错误(契约判定的依据)

```go
var (
    ErrDuplicateID   // 不同类型抢注同一 ID
    ErrUnknownTypeID // New / Decode 遇到未注册的 ID
    ErrFieldNotFound // Get/Set 字段或路径不存在 / 中间节点不是 struct
    ErrTypeMismatch  // 折中转换也转不了的赋值(含数值溢出)
    ErrMalformedData // 信封 / payload 格式错误、字段号越界或重复
    ErrTruncated     // 数据被截断
)
```

## 4. Tag 规范(单一 tag)

单一 `msg` tag 同时承载 JSON key 与 proto 字段号:`msg:"<jsonKey>,<fieldNumber>"`。

```go
type Address struct {
    City string `msg:"city,1"`
    Zip  string `msg:"zip,2"`
}

type User struct {
    Name  string   `msg:"name,1"`
    Age   int32    `msg:"age,2"`
    Addr  Address  `msg:"addr,3"` // 内联嵌套,不单独注册
    Tags  []string `msg:"tags,4"`
}
```

规则:
- 字段号范围:**[1, 65535]**;同一 struct 内必须唯一,越界/重复 → `ErrMalformedData`
- 嵌套 struct 作为**内联字段**递归处理;只有顶层消息才 `Register` 分配 typeID
- **无 `msg` tag 的字段**:JSON 用 Go 字段名兜底,proto 编解码时忽略
- 裸编解码 API 使嵌套/未注册类型也能独立序列化与观测,无需 typeID

## 5. 取值/赋值语义

### 5.1 折中类型转换规则(Set 的判定依据)

| 场景 | 行为 |
|------|------|
| 类型完全相同 | 直接赋值 |
| 数值互转(int/uint/float 变体) | 转换,溢出报 `ErrTypeMismatch` |
| string ↔ 数值 | `strconv` 解析,失败报 `ErrTypeMismatch` |
| []byte ↔ string | 允许 |
| 底层类型相同的别名类型 | 允许 |
| **value 为 nil** | 将字段设为其类型的零值(空结构体、nil 切片等) |
| 其他(struct ↔ int 等) | 报 `ErrTypeMismatch` |

### 5.2 嵌套路径语义

- 分隔符 `.`,例如 `Set("addr.city", "beijing")`
- 中间节点必须是 struct 或指向 struct 的指针,否则 `ErrFieldNotFound`
- 中间节点为 nil 指针时:**Set 自动零值初始化**;Get 遇 nil 中间节点返回该节点零值

## 6. 信封与编解码格式

### 6.1 信封(Envelope)

```
[Magic 0x1234: uint16 BE][TypeID: uint16 BE][Encoding: uint8 (0=JSON, 1=Proto)][PayloadLen: uint32 BE][Payload]
```

### 6.2 Proto wire format(自实现基础子集)

> 决策:不用 `google.golang.org/protobuf`,自实现基础子集,零外部依赖。
> SPEC.md 中给出**完整规格**(自包含,不引导 agent 查外部 proto3 文档,避免引入本任务不支持的 packed/sint/map 等)。

| Go 类型 | wire type | 编码 |
|---------|-----------|------|
| int32/int64/uint32/uint64/sint | 0 (varint) | 变长整数 |
| bool | 0 (varint) | 0/1 |
| string/[]byte | 2 (length-delimited) | 长度前缀 + 字节 |
| 嵌套消息 | 2 (length-delimited) | 长度前缀 + 内层消息字节 |
| 切片 | 重复字段 | 每个元素按字段号各编码一次 |

- 字段 key = `(field_number << 3) | wire_type`
- 解码须校验:字段号在 typeDesc 内存在、wire type 与字段类型匹配、长度不越界

## 7. 骨架提供 vs 留空(TODO)分工

| 文件 | 骨架提供 | 留空(TODO) |
|------|---------|------------|
| message.go | Message 接口契约(方法签名与语义注释) | — |
| registry.go | 函数签名 | 注册表与类型描述符缓存的设计与实现(含并发安全) |
| field.go | FieldDescriptor 定义、函数签名 | 反射字段分析、嵌套路径解析、折中转换 |
| codec.go | 信封常量、JSON 编解码参考实现(标准库) | proto wire 编解码、信封打包/解析、恶意输入校验 |
| 其他 | go.mod、哨兵错误、公开测试 codec_test.go | — |

设计原则:接口契约与"送分"部分(JSON 用标准库)给足,让 agent 聚焦核心评测点(反射分析、proto wire、并发安全、边界校验)。

## 8. 评测点设计(三层考点,并发零提示)

| 层 | 考点 | 验证 |
|----|------|------|
| **契约层** | 注册冲突 → `ErrDuplicateID`;幂等注册;`Get` 不存在字段错误契约;`Decode*` 对空/截断输入报错而非静默成功 | golden test 断言哨兵错误 |
| **边界/安全层** | 信封 PayloadLen 恶意值(超大/与剩余字节不一致)不得 make 溢出 panic;proto 字段号越界/重复报错;折中转换溢出报错;截断 payload → `ErrTruncated` | golden test 喂恶意字节流,断言报错不崩 |
| **并发层** | 位置 A:**惰性 typeDesc 缓存**并发首次访问 → 并发写 map;位置 B:**Register 与 Encode/Decode 并发**叠加 | golden test 并发用例 + `go test -race` |

### 并发零提示设计(关键)

- README 与 SPEC 均**不提及**"并发"、"线程安全"、"-race"
- README 验收标准只写:`go test ./... 通过`(不写 `-race`)
- 评测方用 `go test -race ./...` 验收,agent 不知情
- 骨架中的全局注册表**不加锁、不预置 sync.Map**,让 agent 自行决定是否加锁
- 效果:agent 大概率写出无锁版本(单测全绿)→ golden 并发用例触发 panic/竞态 → 评测其回修能力

## 9. 隐藏测试覆盖矩阵(golden_test.go)

```
契约层:
  - Register 冲突(不同类型同 ID)→ ErrDuplicateID
  - Register 幂等(同类型同 ID)→ 不报错
  - Get 不存在字段 / 不存在的嵌套路径 → ErrFieldNotFound
  - DecodeJSON / DecodeProto 空输入 → 报错(非静默成功)
边界层:
  - 信封 magic 错误 → ErrMalformedData
  - PayloadLen 超大 / 与剩余字节不一致 → 返回错误,不 panic / 不越界读
  - proto 字段号越界、重复 tag → ErrMalformedData
  - Set 给 int 字段塞 string → ErrTypeMismatch;数值溢出 → ErrTypeMismatch
  - 截断的 proto payload → ErrTruncated
功能层:
  - JSON 往返:EncodeJSON → DecodeJSON → Get 取值一致
  - Proto 往返:EncodeProto → DecodeProto → Get 取值一致
  - 嵌套字段:Set("addr.city") → Get("addr.city") → 编解码后仍一致
  - 切片字段往返一致
  - Set(field, nil) 置零、nil 中间节点自动初始化
并发层(配合 -race):
  - 多 goroutine 并发首次 Wrap/New 不同类型 → 不 panic、无竞态(位置 A)
  - 并发 Register 与 Encode/Decode 同时进行 → 无竞态(位置 B)
```

## 10. 公开测试(codec_test.go)设计

给 agent 的测试,仅覆盖**单线程基本功能**:
- 一个类型注册 + JSON 往返
- 一个类型注册 + proto 往返
- 基本 Get/Set(不含嵌套路径恶意输入)
- **不含**:并发用例、恶意输入、注册冲突断言、-race

## 11. 评分标准(混合)

| 等级 | 自动判定 | 人工判定 |
|------|---------|---------|
| 通过 | `go test` + `go test -race` 全绿 | diff 干净、无 hack、未退化成串行 |
| 部分 | 部分测试通过(按比例) | 契约/边界做对但未处理并发 |
| 失败 | 测试失败 / 编译失败 | 误改无关代码 / 硬编码返回值 / 吞掉错误 |

人工审阅聚焦三问:
1. diff 是否只改了必要代码(过度改动扣分)?
2. 是否自己写了/跑了验证(工程习惯)?
3. 是否 hack(如让并发测试退化成串行、强制 `-race` 失效)?

### 性能加分项(不设硬分数)

- 静态审查(人工):typeDesc 是否缓存、tag 是否只解析一次、Encode 是否避免重复反射
- 可选 benchmark(golden 内):记录 `allocs/op` 作参考

## 12. README 与 SPEC 写作要点

**README.md**(issue 风格,给 agent):
- 描述"消息系统编解码库"背景 + 需求(按 SPEC 实现接口)
- 指向 SPEC.md 为唯一权威约定
- 验收标准仅写:"`go test ./...` 通过"
- **零提示**:不点名注册表/缓存,不写并发相关字眼

**SPEC.md**(中性事实,给 agent):
- 接口签名、哨兵错误、tag 格式、信封格式、wire format 完整规格、折中转换规则、字段号上限
- 只陈述功能契约,**不陈述任何并发/性能要求**

## 13. 参考解答(reference.md)大纲

- 正确实现全文(用于自测与对照)
- 每层的正确做法、常见错误方向(无锁 map、忽略恶意输入、吞错误、硬编码)
- 并发修复分层:`sync.RWMutex`(及格)→ `sync.Map`(良好)→ 预构建/写时复制(优秀)
- 性能加分点说明

## 14. 实施步骤

1. 写骨架:`go.mod`、哨兵错误、类型定义、函数签名、信封常量、JSON 参考实现
2. 写**正确版核心实现**(registry/field/codec,即参考答案)
3. 写 golden_test.go(验证:正确版全绿、无锁版确实红 → 保证区分度)
4. 写 codec_test.go(公开)
5. 写 SPEC.md + README.md
6. 写 reference.md
7. 自检:用强 agent 跑一遍,确认可完成 / 难度合理 / 无歧义 / 无法 hack 绕过

---

## 已定决策汇总

- [x] 骨架 + TODO(非全量、非埋缺陷)
- [x] 泛型注册 `Register[T any](typeID uint16)`
- [x] 单一 tag `msg:"<jsonKey>,<fieldNumber>"`
- [x] 字段号范围 [1, 65535],重复/越界 → ErrMalformedData
- [x] 无 msg tag:JSON 用字段名,proto 忽略
- [x] 裸编解码 API(不带信封),解决嵌套/未注册类型的独立观测
- [x] Set(field, nil) 置零值;Set 遇 nil 中间节点自动初始化
- [x] 信封 Magic = 0x1234
- [x] proto wire 自实现基础子集,SPEC 自包含给完整规格
- [x] 性能作为加分项(静态审查 + 可选 benchmark),不设硬分数
- [x] 并发考点:位置 A(惰性 typeDesc 缓存)+ 位置 B(Register 与编解码并发)
- [x] 并发**零提示**(README/SPEC 不提并发、不提 -race;验收标准只写 `go test ./...`)
