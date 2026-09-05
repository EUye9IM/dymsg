# dymsg 评测工作区

本仓库是 Go 动态消息编解码库 **dymsg** 的实现评测工作区。dymsg 的消息 schema 来自
JSON 配置而非编译期写死的 Go 结构体,运行时按类型 ID 注册,支持字段级读写、
presence 三态区分,以及 JSON / Protobuf 双编解码;本工作区围绕它提供**权威规范、
黑盒测试套件与五维评分脚本**,用于对多个实现做统一验收与性能对比。

## 目录结构

| 路径 | 说明 |
| --- | --- |
| `workspace/` | 被评测的 dymsg 实现(module `dymsg`;git 忽略,由评测流程填充) |
| `workspace-ref-impl/` | 参考实现(含性能优化:路径解析零分配、JSON 编码直写缓冲),自带单元测试 |
| `test_files/` | 外部黑盒测试(module `eval`):`dymsg_test.go` 125 个验收用例、`bench_test.go` 基准、`readme.md` 权威规范、`archcheck` 架构静态分析工具 |
| `eval/` | `test_by_code.py` 评分脚本及本地产物 `code_result.json`(已 gitignore) |
| `prompt.md` | 任务规范,内容与 `test_files/readme.md` 一致 |
| `model-comparison-report.md` | 两个模型实现的对比评测报告 |
| `archive/` | 历史实现归档 |

## dymsg 能力速览

- **动态注册**:`ParseSchema`(解析 JSON 配置)→ `Register`(按 `typeId` 注册,重复注册相同内容幂等,并发安全)→ `New(typeID)` 构造消息实例。
- **字段访问**:`Get(path)` 返回值对象 `Value`,以 `Exists()` / `IsSet()` / `Err()` 区分「不存在 / 存在但未设置 / 已设置(含零值)」三态,并提供类型化 getter 与 repeated 的 `Len()` / `Index(i)`;写入用 `Set` / `Append` / `Clear` / `Has` / `SetFields`。
- **路径语法**:`name`、嵌套 `addr.city`、下标 `tags[0]` 及组合 `items[0].name`;`Set` 自动创建中间嵌套消息。
- **双编解码**:`EncodeJSON` / `DecodeJSON`(整体替换语义,含 null 三态处理);`EncodeProto` / `DecodeProto`(自实现 protobuf wire format 子集,支持 packed / unpacked 及混用,未知字段号跳过)。
- **深拷贝写入**:所有 `Set` / `Append` 均深拷贝(嵌套消息、repeated 切片、`[]byte`),消息之间完全独立。
- **并发模型**:`Register` / `New` 并发安全;单个 `Message` 实例不保证并发安全,需调用方自行加锁。

```go
schemas, _ := dymsg.ParseSchema(cfg)
for _, s := range schemas {
    _ = dymsg.Register(s)
}
m, _ := dymsg.New(1001)

_ = m.Set("name", "alice")
_ = m.Set("addr.city", "beijing") // 自动创建中间嵌套消息
_ = m.Append("tags", "a")

name := m.Get("name").String()
if v := m.Get("addr"); v.IsSet() {
    city := v.Message().Get("city").String()
}
data, _ := m.EncodeJSON() // 或 m.EncodeProto()
```

完整接口契约、错误语义与边缘场景速查表见唯一权威规范
[`test_files/readme.md`](test_files/readme.md)(与 `prompt.md` 相同)。

## 构建与验证

环境要求:Go ≥ 1.24,无第三方依赖(protobuf 为自实现子集)。

**module replace 注意**:`test_files/go.mod` 通过 `replace` 指向被测实现——评测容器中
为 `replace dymsg => /workspace`;本地当前为 `replace dymsg => ../workspace`。切换被测
实现时修改该行即可,例如 `replace dymsg => ../workspace-ref-impl`。

```bash
# 黑盒验收(125 用例,含 -race;correctness 维度的唯一依据)
cd test_files && go test ./... -race

# 被测实现自身的静态检查与单元测试(testing / health 维度)
cd ../workspace && gofmt -l . && go vet ./... && go test ./...

# 性能基准(performance 维度,以 BenchmarkReference 归一)
cd ../test_files && go test -run XXX -bench . -benchmem

# 五维评分(默认评测 ../workspace,产出 eval/code_result.json)
python3 eval/test_by_code.py
```

## 评分维度

| 维度 | 权重 | 说明 |
| --- | --- | --- |
| architecture | 30% | 基于 `go/ast + go/types` 的静态分析(函数行数 P90、圈复杂度、依赖环、上帝文件占比、全局状态) |
| correctness | 30% | 黑盒套件全部通过(含 `-race`);整体 resolved 需该项满分 |
| testing | 20% | 只统计被测实现**自带**的测试覆盖(`workspace/*_test.go`) |
| health | 10% | gofmt、`go vet`、TODO/FIXME、导出符号 doc comment |
| performance | 10% | `test_files/bench_test.go` 基准,阈值相对 `BenchmarkReference` 归一 |

## 性能参考

同机单次测量(`go test -benchmem`,数值随机器浮动,仅供量级对比):

| Benchmark | `workspace/` | `workspace-ref-impl/`(含优化) |
| --- | --- | --- |
| GetScalar | ~69 ns,1 alloc | ~37 ns,**0 alloc** |
| SetScalar | ~105 ns | ~44 ns |
| EncodeJSON | ~9.5 µs,163 allocs | ~1.2 µs,**3 allocs** |
| DecodeJSON | ~38 µs | ~23 µs |
| EncodeProto | ~2.1 µs | ~0.8 µs |
| DecodeProto | ~8.2 µs | ~3.0 µs |
| CopyMessage | ~4.3 µs | ~1.4 µs |

## 相关文档

- [`test_files/readme.md`](test_files/readme.md) — dymsg 权威规范(接口契约、路径语法、编解码语义、边缘速查表)
- [`AGENTS.md`](AGENTS.md) — 工作区协作约定与评分规则的非显性细节
- [`model-comparison-report.md`](model-comparison-report.md) — 历史实现对比评测报告
