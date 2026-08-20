# msgcodec:代码 Agent 评测任务设计文档

> 目标:设计一个用于评测**现成代码 agent** 的 Go 编程任务。
> 核心指标:**正确性优先**。
> 执行方式:手动执行单个任务。
> 验证方式:**混合** —— 自动化测试(`go test` / `-race` / `go vet`)+ 人工审阅 diff。

---

## 1. 任务概述

任务要求 agent 完善一个结构化消息编解码库 `msgcodec`。该库的核心能力:

1. **注册消息类型**:为消息类型分配类型 ID。
2. **原生 Go 取值/赋值**:通过字段名对消息字段做 `Get` / `Set`,支持嵌套路径。
3. **双序列化**:同一消息既可按 **JSON** 编码,也可按 **Protobuf wire format** 编码,解码时按信封标识自动分发。

三者统一依赖一份**类型描述符 `typeDesc`**(反射分析结果:字段名、Go 类型、JSON 名、proto 字段号、嵌套关系)。任务的关键陷阱(并发缺陷)就埋在该描述符的**惰性构建缓存**中。

## 2. 目录结构

```
msgcodec/
├── README.md          # 任务描述(issue 风格,给 agent 看)
├── go.mod
├── registry.go        # 注册表 + typeDesc 惰性构建缓存  ← 并发缺陷藏身处
├── field.go           # 反射字段分析、Get/Set、折中类型转换
├── codec.go           # JSON/Proto 编解码 + 信封封装
├── codec_test.go      # 公开测试(给 agent 看的部分)
├── golden_test.go     # 隐藏黄金测试(评测用,agent 看不到)
└── reference.md       # 参考 diff + 评分说明(评测人用)
```

## 3. 接口契约

### 3.1 函数签名

```go
package msgcodec

// Register 注册消息类型,绑定类型 ID。
// 重复注册同一类型(相同 ID)= 幂等,不报错。
// 不同类型抢注同一 ID = 应返回 ErrDuplicateID。
func Register(v any, typeID uint16) error %% register 的v是不是用反射的Type比较合理

// Wrap 包装一个消息实例,后续取值/赋值/编解码都通过它。
func Wrap(v any) (*Message, error)

// New 按类型 ID 创建空消息(用于反序列化)。
func New(typeID uint16) (*Message, error)

type Message struct{ /* 内部持有 reflect.Value + *typeDesc */ }

// —— 原生 Go 取值/赋值 ——
func (m *Message) Get(field string) (any, error) // 支持 "addr.city"
func (m *Message) Set(field string, value any) error
func (m *Message) Fields() []FieldDescriptor

// —— 双编解码 ——
func (m *Message) EncodeJSON() ([]byte, error)
func (m *Message) EncodeProto() ([]byte, error)
func (m *Message) DecodeJSON(data []byte) error
func (m *Message) DecodeProto(data []byte) error
```

### 3.2 哨兵错误(契约层的判定依据)

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

## 4. Tag 规范

职责分离:`json` tag 存 JSON 名,`msg` tag 存 proto 字段号。 %% 是不是用同一个tag,规定一个格式获取proto编号和jsonkey

```go
type Address struct {
    City string `json:"city" msg:"1"`
    Zip  string `json:"zip"  msg:"2"`
}

type User struct {
    Name  string   `json:"name"  msg:"1"`
    Age   int32    `json:"age"   msg:"2"`
    Addr  Address  `json:"addr"  msg:"3"` // 内联嵌套,不单独注册
    Tags  []string `json:"tags"  msg:"4"`
}
```

规则:
- proto 字段号必须唯一、≥ 1;同一 struct 内不允许重复(否则 `ErrMalformedData`) %% proto字段号需要规定上限吗？
- 嵌套 struct 作为**内联字段**递归处理;只有顶层消息才 `Register` 分配 typeID %% 需要对内层的结构做观测怎么弄呢？需不需要提供一种不依赖typeid 序列化的方法
- 未带 `msg` tag 的字段:proto 无法编号,按错误处理(待定:报错 vs 忽略) %% 忽略吧

## 5. 取值/赋值语义

### 5.1 折中类型转换规则(Set 的判定依据)

| 场景 | 行为 |
|------|------|
| 类型完全相同 | 直接赋值 |
| 数值互转(int/uint/float 变体) | 转换,溢出报 `ErrTypeMismatch` |
| string ↔ 数值 | `strconv` 解析,失败报 `ErrTypeMismatch` |
| []byte ↔ string | 允许 |
| 底层类型相同的别名类型 | 允许 |
| 其他(struct ↔ int 等) | 报 `ErrTypeMismatch` |
%% nil赋值是不是也支持一下，设空结构体什么的

### 5.2 嵌套路径语义

- 分隔符 `.`,例如 `Get("addr.city")`
- 中间节点必须是 struct 或指向 struct 的指针,否则 `ErrFieldNotFound`
- 中间节点为 nil 指针时:Get 报错 / Set 可自动零值初始化(待定) %% 自动零值初始化

## 6. 信封与编解码格式

### 6.1 信封(Envelope)

```
[Magic 0x4D43: uint16 BE][TypeID: uint16 BE][Encoding: uint8 (0=JSON, 1=Proto)][PayloadLen: uint32 BE][Payload]
```
%% Magic 改0x1234

### 6.2 Proto wire format(自实现基础类型子集)

%% 这段需要提供完整提示吗？还是直接找一个proto3的协议文档
> 决策:不用 `google.golang.org/protobuf`,自实现基础子集,保证零外部依赖、评测环境可控。

| Go 类型 | wire type | 编码 |
|---------|-----------|------|
| int32/int64/uint32/uint64/sint | 0 (varint) | 变长整数 |
| bool | 0 (varint) | 0/1 |
| string/[]byte | 2 (length-delimited) | 长度前缀 + 字节 |
| 嵌套消息 | 2 (length-delimited) | 长度前缀 + 内层消息字节 |
| 切片 | 重复字段 | 每个元素按字段号各编码一次 |

- 字段 key = `(field_number << 3) | wire_type`
- 解码须校验:字段号在类型描述符内存在、wire type 与字段类型匹配、长度不越界

## 7. 三层缺陷设计

| 层 | 缺陷 | 测的能力 | 自动验证 |
|----|------|---------|---------|
| **契约层** | 不同类型抢注同一 ID **静默覆盖**而非 `ErrDuplicateID`;`Get` 不存在字段错误契约含糊;`Decode*` 对空/截断输入静默返回 | 读懂接口契约、错误处理 | golden test 断言哨兵错误 |
| **边界/安全层** | PayloadLen 恶意值(超大/与剩余字节不一致)直接 `make` 溢出 panic;proto 字段号越界/重复未校验;`Set` 数值转换溢出未处理 | 防御性编程、整数溢出意识 | golden test 喂恶意字节流,断言返回错误而非崩溃 |
| **并发层** | `typeDesc` 惰性构建后写回**无锁 map**,并发首次 `Wrap`/`New` 不同类型 → `concurrent map write` panic | 并发意识、Go 工程深度 | `go test -race` 自动抓 |

并发层要点:
- 惰性构建是"合理优化"(反射分析昂贵),但并发安全写回才是评测点
- README 不点名存在并发问题,agent 需自行发现并选择方案
- 修复方案分层:`sync.RWMutex`(及格)→ `sync.Map`(良好)→ 预构建/写时复制(优秀)


## 8. 隐藏测试覆盖矩阵(golden_test.go)

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
并发层:
  - 多个 goroutine 并发首次 Wrap/New 不同类型 → 不 panic(配合 -race)
  - 并发编解码同一类型 → 结果一致
```

## 9. 评分标准(混合)

| 等级 | 自动判定 | 人工判定 |
|------|---------|---------|
| 通过 | `go test` + `go test -race` 全绿 | diff 干净、无 hack、未退化成串行 |
| 部分 | 部分测试通过(按比例) | 修了契约/边界但未意识到并发问题 |
| 失败 | 测试失败 / 编译失败 | 误改无关代码 / 硬编码返回值 / 吞掉错误 |

人工审阅聚焦三问:
1. diff 是否只改了必要代码(过度改动扣分)?
2. 是否自己写了/跑了验证(工程习惯)?
3. 是否通过 hack 让测试变绿(如强制 `-race` 不生效 / 让并发测试串行化)?

%% 能不能加一个对性能的分析来评分？

## 10. 任务描述(README)写作要点

- 用 **issue 风格**描述缺陷现象,不直接点名"registry.go 有并发 bug"
- 只给**部分公开测试** `codec_test.go`,让 agent 有验证手段但不给全部答案
- 明确验收标准:"`go test ./...` 与 `go test -race ./...` 应全部通过"
- 留下探索空间:README 不点明存在并发问题

## 11. 参考解答(reference.md)大纲

- 每层缺陷的位置、触发条件、正确修复方式
- 常见错误方向:只加锁不修契约、用全局串行化、吞掉错误、硬编码
- 说明各修复方案的层次与扣分理由

## 12. 实施步骤(退出本设计后)

1. 写**正确版**实现(先写对,再反向埋缺陷——保证缺陷可修、可测)
2. 写 golden_test.go(先验证正确版全绿、错误版确实红 → 保证测试有区分度)
3. 写 reference.md
4. 写 README.md + 公开 codec_test.go
5. 任务自检:用最强 agent 跑一遍,确认可完成 / 难度合理 / 无歧义 / 无法 hack 绕过

---

## 待确认/可调整点

- [ ] proto 是否按推荐**自实现基础子集**(还是坚持真实 protobuf 库)
- [ ] 未带 `msg` tag 的字段:报错还是忽略
- [ ] Set 时中间 nil 指针:报错还是自动零值初始化
- [ ] 并发修复方案的分层评分是否合适
- [ ] 三层缺陷是否需要同时埋入,还是先埋两层
