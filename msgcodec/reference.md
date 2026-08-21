# msgcodec 参考实现与评分说明(评测人用)

本文件仅供评测人使用,不提供给被测 agent。

## 1. 参考实现

正确版实现即本包源码,架构:

| 文件 | 职责 |
|------|------|
| `errors.go` | 7 个哨兵错误 |
| `schema.go` | `FieldType`/`FieldSchema`/`MessageSchema` 接口、`schemaImpl`、`ParseSchema`(JSON 解析与校验) |
| `registry.go` | `runtimeSchema` 构建、`Register`(幂等/冲突)、`New`;`sync.RWMutex` 保护注册表 |
| `msg.go` | `msgImpl`(复合消息)+ `valueMsg`(标量/列表包装):点路径+下标 Get/Set、presence、深拷贝、折中转换 |
| `wire.go` | varint / fixed32 / fixed64 / append / decode / skipField |
| `proto.go` | `EncodeProto`(packed 标量 repeated、message 递归)、`DecodeProto`(packed/unpacked 兼容、未知字段跳过、wire type 校验) |
| `json.go` | `EncodeJSON`(按 schema 声明顺序)、`DecodeJSON`(null=未设置、未知 key 忽略) |

发布给 agent 时,将上述实现体替换回 `TODO` 骨架,仅保留公开接口签名与类型定义(即最初骨架形态),并附 `codec_test.go`(公开)与 `README.md`/`SPEC.md`。`golden_test.go` 与 `reference.md` 不随任务下发。

## 2. 评测流程

1. 收取 agent 实现(应包含 `msgcodec` 包完整源码)。
2. 将 agent 源码放入本目录,替换参考实现(保留接口契约)。
3. 执行验收:
   ```
   go build ./... && go vet ./...
   go test -run Public ./...        # 公开测试(agent 见过)
   go test -race -run Golden ./...  # 隐藏黄金测试,含并发
   ```
4. 结合人工审阅 diff 判定等级。

## 3. 三层考点判定

### 契约层
- 正确:`ErrDuplicateID`(重复 typeID)、`ErrUnknownTypeID`(未知 ID)、`ErrFieldNotFound`、`ErrIndexOutOfRange`、空输入 `ErrTruncated`。
- 常见错误:注册冲突静默覆盖;Get 不存在字段不报错;空输入静默成功;错误类型混淆。

### 边界/安全层
- 正确:schema 校验(字段号 [1,65535]、唯一、字段名合法);解码校验(未知字段跳过、wire type 不匹配报 `ErrMalformedData`、非法 wire type 6/7 报错、截断 `ErrTruncated`);折中转换溢出报 `ErrTypeMismatch`。
- 常见错误:length 字段超大导致 `make`/越界 panic;wire type 不匹配未检查;未知字段报错(破坏向后兼容);负下标未拦截。

### 并发层(场景暗示)
- README/SPEC 未直接提并发;评测以 `-race` 验收。
- 正确方案分层:
  - 及格:`sync.RWMutex` 保护注册表读写。
  - 良好:`sync.Map` 或分段锁。
  - 优秀:注册时预构建 + 写时复制 / 原子快照,读路径无锁。
- 常见错误(会触发 `-race` 失败):无锁 `map` 并发写;惰性缓存首次访问并发写;`Register` 与 `New` 并发读写无保护。

## 4. 深拷贝判定

- `Set("", m)`、`Set("field", msg)`、`Set("tags", []Message{...})` 必须深拷贝;修改目标不影响源。
- 常见错误(判失败或扣分):浅拷贝共享底层;`[]byte`/切片字段共享;子消息共享引用。

## 5. presence 判定

- 未设置字段:Get 返回 nil;Proto/JSON 均不输出。
- 显式零值:仍输出(与未设置可区分)。
- `Set(field, nil)` 清除;JSON 解码 `null` = 未设置。
- 常见错误:零值与未设置混淆;`Set(nil)` 语义错误。

## 6. 性能加分项(不设硬分数)

静态审查要点:
- 运行时描述是否缓存(Register 时构建一次),Get/Set/编解码不重复解析 schema。
- 字段查找是否 O(1)(按名/按号索引)。
- 编解码是否避免无谓分配(如 packed 一次分配)。
- `valueMsg` 包装是否轻量。

可选:golden 内 benchmark 记录 `allocs/op` 作参考,不作为通过条件。

## 7. 评分示例

| 等级 | 自动判定 | 人工判定 |
|------|---------|---------|
| 通过 | `go build/vet` + `-run Public` + `-race -run Golden` 全绿 | diff 干净、无 hack、深拷贝与 presence 正确、并发方案合理 |
| 部分 | 部分 Golden 通过 | 契约/边界做对但未处理并发(无锁版在 -race 崩溃) |
| 失败 | 编译失败 / Golden 大量失败 | 误改接口 / 硬编码返回值 / 吞掉错误 / 浅拷贝冒充深拷贝 |
