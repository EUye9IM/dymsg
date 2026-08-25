# dymsg 两个模型实现对比评测报告

- 日期：2026-08-25
- 评测对象：`archive/impl-flash-0.tar.gz`（FLASH）与 `archive/impl-pro-0.tar.gz`（PRO）
- 评测脚本：`eval/test_by_code.py`
- 环境：go 1.27.0，黑盒测试 `go test -race -count=1`，共 125 个用例

> **评测体系更新记录（2026-08-25 晚）**：原 `size` 维度已更名为 **`architecture`**，改为基于
> `go/ast + go/types` 静态分析（函数行数 P90、圈复杂度分级、文件依赖环、上帝文件占比、
> 全局状态），由 `test_files/archcheck` 提供精确数据；权重同步调整为
> **architecture 30% / testing 20% / correctness 30% / health 10% / performance 10%**。
> **本报告第 1-5 节及 A-C 节的分值均为旧体系（size + 权重 10/30/30/10/20）下的历史结果，
> 与新体系不可直接对比**；新体系下的重评见文末 **D 节**。

---

## 1. 实现概览

| | FLASH | PRO |
|---|---|---|
| 文件数 | 5（dymsg.go / json.go / proto.go / go.mod / dymsg_test.go） | 8（types / schema / message / value / convert / json / proto / go.mod） |
| 源码行数 | 2489（dymsg.go 单文件 1503 行） | 2766 |
| go 版本 | go 1.21 | go 1.24.4 |
| 自带测试 | 有（dymsg_test.go 1281 行 / 63 个用例） | 无 <sup>1</sup> |
| 组织风格 | 单文件大函数 | 模块化、按职责分文件 |

> <sup>1</sup> **PRO 无自带测试的背景**：prompt.md 的「验收标准」一节（第 27-29 行）说明验收时以 `replace dymsg => /workspace` 的方式引用该包，并由外部黑盒测试据此验收。PRO 模型反馈：它原本写了自测，但担心自测中的 `Register` 调用会在验收时与黑盒测试注册相同的类型 ID（msgid），触发「重复注册不同内容 → ErrDuplicateID」冲突，因此在交付前删除了测试文件。该理由是模型自述，未在产物中验证（本报告按实际交付内容评测，测试维度按 0 计分）。

---

## 2. 五维得分对比（旧体系：size + 权重 10/30/30/10/20）

| 维度（权重） | FLASH | PRO | 胜出 |
|---|---|---|---|
| size（10%） | 47.4（avg_comp 8.06，6 个超长函数） | 57.6（avg_comp 6.43，5 个超长函数） | PRO |
| testing（30%） | 68.4（覆盖率 66.5%，63 测试，test/src=0.51） | 0.0（覆盖率 0%） | FLASH |
| correctness（30%） | 93.6（117/125，8 失败） | 89.6（112/125，13 失败） | FLASH |
| health（10%） | 0.0（gofmt 未格式化 + 33 个导出无注释） | 40.0（gofmt/vet 干净，17 个导出无注释） | PRO |
| performance（20%） | 50.7（time 49.4 / mem 52.1） | 47.4（time 44.7 / mem 50.2） | FLASH |
| **加权总分** | **0.63** | **0.46** | **FLASH** |
| resolved | False | False | — |

两个实现均未达 `resolved`（要求 correctness = 1.0）。

---

## 3. 正确性差异（失败测试分类）

### 3.1 FLASH 通过、PRO 失败（6 个）——FLASH 的转换层更健壮

| 测试 | 失败点 | 根因（PRO 侧） |
|---|---|---|
| TestSetRepeated | `Set("ri32", [3]int32{...})` 数组输入报 type mismatch | convertRepeatedSlice 只接受精确类型 slice，不支持 array |
| TestSetMessageSliceVariants | `Set` array `[]*Message` 报 type mismatch | 同上，不支持 array/`[]any` |
| TestConvertOverflowEdges | typed-nil `*Message` 赋给 int32 报 nil | Set 顶层把 typed-nil 当清空处理 |
| TestConvertMessageTypedNil | typed-nil message 赋给 message 字段报 nil | 同上 |
| TestSetSelfTypedNil | `Set("", typed-nil)` 报 nil（期望 ErrTypeMismatch） | 同上 |
| TestDecodeJSONWhitespace | 纯空白输入报 ErrMalformedData（期望 ErrTruncated） | DecodeJSON 未区分错误码 |

### 3.2 PRO 通过、FLASH 失败（1 个）——PRO 的 Get 语义更正确

| 测试 | 失败点 | 根因（FLASH 侧） |
|---|---|---|
| TestGetIndexedNestedStates | `Get("rmsg[0].city")` 当 rmsg[0] 为 nil 时返回 ErrFieldNotFound（期望 exists=true/set=false） | FLASH 的 Get 遇到 nil 中间元素直接返回 ErrFieldNotFound，缺少 schema 感知 |

### 3.3 共同失败（7 个）——两者的共同短板

| 测试 | 共同问题 |
|---|---|
| TestParseSchemaEmpty | `ParseSchema({})`/`{"types":[]}` 报 ErrMalformedData，期望返回 0 个 schema、无错误 |
| TestAppend | 错误码边界（FLASH：空路径报 ErrTypeMismatch 而非 ErrFieldNotFound；PRO：追加不匹配 message 未报错） |
| TestClearPathIdempotentWhenParentUnset | Clear 未设置嵌套的未知字段未报 ErrFieldNotFound |
| TestClearRepeatedElement | Clear 标量 repeated 元素未报 ErrFieldNotFound |
| TestSetElementMessage | `Set("rs[0]", nil)` 标量元素 nil 未报 ErrTypeMismatch |
| TestSetIndexedNested | Set 到 nil message 元素自动创建而非报 ErrFieldNotFound |
| TestClearIndexedNested | Clear 到 nil message 元素自动创建而非报 ErrFieldNotFound |

---

## 4. 根因分析

### 4.1 FLASH 强在哪

1. **转换层健壮**：`setField` 用 `reflect.ValueOf` 统一处理（dymsg.go:581），`convertRepeatedSlice`（dymsg.go:1159）逐元素走 `convertScalar`，天然支持 slice/array/`[]any` 与跨类型转换。
2. **typed-nil 语义正确**：`setField` 对 message 显式 `msg == nil → ErrTypeMismatch`（dymsg.go:600-601）。
3. **错误码细分**：JSON/Proto 解码区分 `ErrTruncated` 与 `ErrMalformedData`。
4. **自带测试**：1281 行、63 个用例、覆盖率 66.5%，testing 维度得分 68.4。

### 4.2 FLASH 弱在哪

1. **代码卫生差**：gofmt 未格式化（dymsg.go、dymsg_test.go），33 个导出符号无文档注释 → health = 0。
2. **可维护性差**：单文件堆 1503 行，avg_comp 8.06，6 个超长函数。
3. **Get 缺 schema 感知**：nil 中间元素的字段状态语义错误（dymsg.go:470-472）。

### 4.3 PRO 强在哪

1. **schema 感知设计**：`schemaOnlyGet` / `validatePathSchema`（message.go:176-239）在路径中间遇到未设置/nil message 时按 schema 树继续解析，返回 `exists:true/set:false` —— Get 语义最优雅、唯一通过 TestGetIndexedNestedStates 的实现。
2. **模块化与可读性**：types/schema/message/value/convert/json/proto 分文件，职责清晰，avg_comp 6.43。
3. **代码规范**：gofmt/vet 干净，导出符号文档注释更全（仅 17 个未注释）。

### 4.4 PRO 弱在哪

1. **转换不完整**：`convertRepeatedSlice`（convert.go:79）每类型只做精确 type switch，不支持 array、不支持 `[]any`、不支持元素级转换 —— 数组输入全挂。
2. **typed-nil 处理与测试期望冲突**：`Set` 顶层把 typed-nil 一律当"清空"（message.go:280），未按字段类型报 ErrTypeMismatch。
3. **零测试**：testing 维度 0 分。模型自述原因是担心自测 `Register` 的 msgid 与验收黑盒测试冲突而主动删除（见第 1 节注 1），但实际交付物中无测试，按 0 计分。

---

## 5. 结论

- **按当前加权 eval 评分，FLASH 明显更好**（0.63 vs 0.46）：在权重合计 60% 的 testing + correctness 两个维度全面领先，转换层更健壮（数组输入、typed-nil、错误码细分），且自带 66.5% 覆盖率的测试，性能也略优。
- **若从代码工程质量看，PRO 更优**：模块化、可读性、gofmt/vet 干净、schema 感知的 Get 语义更优雅，是"好代码"；但功能完成度低——转换不完整、多 5 个黑盒失败、零测试。
- **本质差异**：FLASH = 功能完成度高但代码粗糙（正确性优先）；PRO = 代码优雅但功能缺失（结构优先）。
- **两者共同短板**：边界错误码语义（Append/Clear 的错误码、`Set("rs[0]", nil)`）、空配置解析（`ParseSchema({})`）——这些是达成 `resolved`（correctness = 1.0）必须补齐的部分。

> 附注：评测期间 `workspace/` 目录被切换到 FLASH 实现以运行黑盒测试；`workspace-ref/` 为原参考实现。

---

# 追加：-1 版本评测（2026-08-25）

`archive/impl-flash-1.tar.gz` 与 `archive/impl-pro-1.tar.gz`，评测环境与方法同上。**本节的
得分与 A-C 结论均为旧体系（size 维度 + 权重 10/30/30/10/20）的历史结果**，新体系重评见 D 节。

## A. 与 -0 版本的得分对比

| 维度（权重） | flash-0 | **flash-1** | 变化 | pro-0 | **pro-1** | 变化 |
|---|---|---|---|---|---|---|
| size（10%） | 47.4 | **60.6** | +13.2（达标） | 57.6 | 57.5 | -0.1 |
| testing（30%） | 68.4 | **72.0** | +3.6（达标） | 0 | **53.5** | +53.5 |
| correctness（30%） | 93.6 | **91.2** | -2.4 | 89.6 | **92.0** | +2.4 |
| health（10%） | 0 | **100** | +100（达标） | 40 | **100** | +60（达标） |
| performance（20%） | 50.7 | **55.5** | +4.8 | 47.4 | **53.2** | +5.8 |
| **加权总分** | 0.63 | **0.76** | +0.13 | 0.46 | **0.70** | +0.24 |
| resolved | False | False | — | False | False | — |

- 两个 -1 版本均大幅提升；**PRO-1 进步最大（+0.24）**——补上了测试（coverage 56.8%）、修复了数组输入等转换缺陷、health 从 40 到 100。
- **FLASH-1 仍然领先（0.76 vs 0.70）**：size/testing/health 三维达标，是四个版本中唯一多个维度达标的实现；但 correctness 出现小幅回归。

## B. -1 版本之间的失败测试对比

| 失败测试 | FLASH-1 | PRO-1 |
|---|---|---|
| TestParseSchemaEmpty（空配置） | 通过 | 失败（`{}` 报 ErrMalformedData） |
| TestParseSchemaMalformed/message_null_schema | 失败（**回归**，null schema 未报错） | 通过 |
| TestAppend | 失败（带索引路径错误码：ErrTypeMismatch vs ErrFieldNotFound） | 失败（空路径错误码） |
| TestClearPathIdempotentWhenParentUnset | 失败（unset 嵌套 unknown 返回 nil） | 失败（unset 索引路径错误码） |
| TestClearRepeatedElement | 失败（标量 repeated 元素返回 nil） | 失败（索引路径错误码） |
| TestConvertOverflowEdges | 通过 | 失败（typed-nil → int32 返回 nil） |
| TestConvertMessageTypedNil | 失败（**回归**） | 失败 |
| TestSetElementMessage | 失败（标量元素 nil 返回 nil） | 失败（nil 元素设置错误码） |
| TestSetIndexedNested | 失败 | 通过（已修复） |
| TestClearIndexedNested | 失败 | 失败 |
| TestDecodeJSONWhitespace | 失败（**回归**，错误码） | 失败（错误码） |
| TestSetSelfTypedNil | 失败（**回归**） | 失败 |
| TestGetIndexedNestedStates | 通过（已修复） | 通过 |

**失败数**：FLASH-1 = 11（其中 **4 个为 -0 时代通过、重构引入的回归**：message_null_schema、TestConvertMessageTypedNil、TestDecodeJSONWhitespace、TestSetSelfTypedNil）；PRO-1 = 10（较 -0 修复 3 个：TestSetRepeated / TestSetMessageSliceVariants / TestSetIndexedNested）。

## C. 结论（-1 版本）

- **FLASH-1 仍整体更优（0.76 vs 0.70）**：达标维度更多（size/testing/health），testing 覆盖率更高（66.0%），性能略好（55.5 vs 53.2）。但重构中引入 4 个功能回归（typed-nil、错误码、schema null 校验），correctness 不升反降（93.6→91.2）。
- **PRO-1 进步最大**：从 0.46 到 0.70，正确性反超为 115/125 vs 114/125，工程规范（模块化 + schema 感知）依旧，且补齐了测试。主要短板仍是边界错误码与 typed-nil 语义。
- **两者共同短板不变**：边界错误码语义（Append/Clear）、typed-nil 处理、`ParseSchema({})` 空配置——达成 `resolved`（correctness = 1.0）仍需补齐这些。
- 若目标是当前加权评分，**FLASH-1 是最佳版本（0.76）**；若关注正确性上限与可维护性平衡，**PRO-1** 的修复方向更健康（无回归、结构更好），距离 resolved 同样有差距。

---

## D. 新体系重评 -1 版本（architecture 30% / testing 20% / correctness 30% / health 10% / performance 10%）

用改造后的 `architecture` 维度（`test_files/archcheck`，go/ast + go/types 精确分析）对 -1 版本重新评分：

| 维度（权重） | FLASH-1 | PRO-1 |
|---|---|---|
| architecture（30%） | 59.4 | 69.6 |
| testing（20%） | 72.0（覆盖率 66.0%） | 53.5（覆盖率 56.8%） |
| correctness（30%） | 91.2（114/125） | 92.0（115/125） |
| health（10%） | 100 | 100 |
| performance（10%） | 56.6 | 55.6 |
| **加权总分** | **0.75** | **0.75** |
| resolved | False | False |

### D.1 architecture 明细（新体系核心）

| 指标 | FLASH-1 | PRO-1 |
|---|---|---|
| 上帝文件占比 | dymsg.go **42.0%** | message.go **22.3%** |
| 文件数 | 4 | 7 |
| 依赖环 | **0**（json/message/proto 单向依赖 dymsg.go） | 1 个（5 文件强连通环） |
| 平均圈复杂度 | 7.81 | 7.29 |
| 复杂度 >15 函数数 | 12 | 15 |
| P90 函数行数 | 65 | 55 |
| 4 子分（复杂度/行数/模块/依赖） | 33.5 / 85.0 / 48.9 / 100 | 34.7 / 95.0 / 100 / 80 |

**架构洞察**：go/types 精确解析还原了正则启发式无法识别的真相——FLASH-1 依赖**无环**（但被 42% 上帝文件拖累），PRO-1 文件更均衡（22.3%）但存在一个 5 文件强连通依赖环。

### D.2 权重敏感性（同一组实现）

| 权重方案 | FLASH-1 | PRO-1 | 差距 |
|---|---|---|---|
| arch 10% / perf 20% | 0.76 | 0.70 | 0.06 |
| arch 20% / perf 10% | 0.77 | 0.73 | 0.04 |
| **arch 30% / testing 20%** | **0.75** | **0.75** | **0.00** |

架构权重每升 10%，PRO 的架构优势逐步抵消 FLASH 的测试优势，在 30/20 组合下恰好打平。

### D.3 结论（新体系）

- **新体系下 FLASH-1 与 PRO-1 平分（0.75 = 0.75）**，且原因完全对称：FLASH 测试更充分（72 vs 53.5）、PRO 架构更优（69.6 vs 59.4）。
- 旧体系"FLASH 明显领先"的结论已不成立——主要原因是架构维度从 10% 提到 30%，放大了 PRO 的模块化优势。
- `resolved` 仍为 False：两者 correctness 均未达 1.0，共同短板依旧是边界错误码语义、typed-nil 处理、`ParseSchema({})` 空配置。
