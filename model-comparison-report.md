# dymsg 两个模型实现对比评测报告

- 日期：2026-08-25
- 评测对象：`archive/impl-flash-0.tar.gz`（FLASH）与 `archive/impl-pro-0.tar.gz`（PRO）
- 评测脚本：`eval/test_by_code.py`（加权：size 10% / testing 30% / correctness 30% / health 10% / performance 20%）
- 环境：go 1.27.0，黑盒测试 `go test -race -count=1`，共 125 个用例

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

## 2. 五维得分对比

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
