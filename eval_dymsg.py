#!/usr/bin/env python3
"""eval_dymsg.py — 从多个维度评价 dymsg 包。

维度(等权):
  1. size         规模与复杂度
  2. testing      测试充分性(覆盖率)
  3. health       代码健康度(gofmt/vet/TODO/文档注释)
  4. contract     契约一致性(SPEC.md 与源码比对)
  5. concurrency  并发与安全
  6. errors       错误处理
  7. performance  性能(go benchmark)

用法:
  python3 eval_dymsg.py [--no-go] [--quiet]
    --no-go   跳过 go 工具调用(仅静态分析)
    --quiet   只输出 JSON 到 stdout(不打印进度)

输出:JSON(含 meta / dimensions / total_score)。
纯 Python 3 标准库,零第三方依赖。
"""

import json
import os
import re
import subprocess
import sys

ROOT = os.path.dirname(os.path.abspath(__file__))
DYMSG_DIR = os.path.join(ROOT, "dymsg")
XTEST_DIR = os.path.join(ROOT, "dymsgxtest")

# ---- 全局配置(直接在此修改) ----
USE_GO = True    # 是否调用 go 工具(gofmt / vet / go test -cover / go test -bench)
QUIET = False    # 为 True 时不打印进度,只输出 JSONL
OUTPUT = "code_result.json"      # 输出文件路径;为空则写 stdout

# --------------------------------------------------------------------------
# 通用工具
# --------------------------------------------------------------------------

def run(cmd, cwd=None):
    try:
        p = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, timeout=120)
        return p.stdout, p.stderr, p.returncode
    except (subprocess.TimeoutExpired, FileNotFoundError) as e:
        return "", str(e), -1


def strip_comments(src):
    """去除 // 与 /* */ 注释(保留字符串/字符近似;足够静态统计)。"""
    out = []
    i, n = 0, len(src)
    in_str = in_char = in_line = in_block = False
    while i < n:
        c = src[i]
        c2 = src[i + 1] if i + 1 < n else ""
        if in_str:
            out.append(c)
            if c == "\\":
                out.append(c2)
                i += 2
                continue
            if c == '"':
                in_str = False
            i += 1
            continue
        if in_char:
            out.append(c)
            if c == "\\":
                out.append(c2)
                i += 2
                continue
            if c == "'":
                in_char = False
            i += 1
            continue
        if in_line:
            if c == "\n":
                in_line = False
                out.append(c)
            i += 1
            continue
        if in_block:
            if c == "*" and c2 == "/":
                in_block = False
                i += 2
            else:
                i += 1
            continue
        if c == '"':
            in_str = True
            out.append(c)
        elif c == "'":
            in_char = True
            out.append(c)
        elif c == "/" and c2 == "/":
            in_line = True
            i += 1
        elif c == "/" and c2 == "*":
            in_block = True
            i += 1
        else:
            out.append(c)
        i += 1
    return "".join(out)


def read_go_files():
    """实时扫描 dymsg 目录下的非测试 .go 源码文件,返回 {name: content}。"""
    files = {}
    for name in sorted(os.listdir(DYMSG_DIR)):
        if name.endswith(".go") and not name.endswith("_test.go"):
            path = os.path.join(DYMSG_DIR, name)
            with open(path, encoding="utf-8") as f:
                files[name] = f.read()
    return files


# --------------------------------------------------------------------------
# 维度 1:规模与复杂度
# --------------------------------------------------------------------------

def _count_lines(src):
    lines = src.splitlines()
    total = len(lines)
    code = 0
    blank = 0
    for ln in lines:
        s = ln.strip()
        if not s:
            blank += 1
        elif s.startswith("//") or s.startswith("/*") or s.startswith("*"):
            pass
        else:
            code += 1
    return total, code, blank


def _extract_funcs(files):
    """提取每个文件的函数: (name, start_line, end_line, complexity)。"""
    funcs = []
    for name, src in files.items():
        lines = src.splitlines()
        clean = strip_comments(src).splitlines()
        for i, ln in enumerate(clean):
            m = re.match(r"^\s*func\s+(?:\(\w+\s+\*?[\w.]+\)\s*)?(\w+)\s*\(", ln)
            if not m:
                continue
            fname = m.group(1)
            # 寻找函数体结束(粗略:按大括号平衡)
            depth = 0
            j = i
            while j < len(clean):
                depth += clean[j].count("{") - clean[j].count("}")
                if depth <= 0 and j > i:
                    break
                j += 1
            body = "\n".join(clean[i:j + 1])
            comp = len(re.findall(r"\b(if|for|switch|select|case|range)\b", body))
            # 未导出(小写开头)不参与复杂度评分
            funcs.append({
                "file": name, "name": fname, "line": i + 1,
                "end_line": j + 1, "complexity": comp,
                "exported": fname[0].isupper() and not fname.startswith("Test"),
            })
            i = j
    return funcs


def score_size(files):
    total_lines = code_lines = 0
    per_file = {}
    for name, src in files.items():
        t, c, b = _count_lines(src)
        total_lines += t
        code_lines += c
        per_file[name] = {"total": t, "code": c, "blank": b}
    funcs = _extract_funcs(files)
    nfunc = len(funcs)
    avg_comp = (sum(f["complexity"] for f in funcs) / nfunc) if nfunc else 0
    big_funcs = [f for f in funcs if f["end_line"] - f["line"] > 80]

    # 行数分:1500-4000 最佳
    if 1500 <= total_lines <= 4000:
        line_score = 100
    elif total_lines <= 0:
        line_score = 0
    else:
        line_score = max(0, 100 - abs(total_lines - 2500) / 30)

    # 复杂度分
    if avg_comp <= 2:
        comp_score = 100
    elif avg_comp <= 4:
        comp_score = 80
    elif avg_comp <= 6:
        comp_score = 60
    else:
        comp_score = max(0, 60 - (avg_comp - 6) * 8)

    # 超长函数扣分
    overlong_penalty = len(big_funcs) * 5
    score = 0.6 * line_score + 0.4 * comp_score - min(overlong_penalty, 30)
    return max(0, min(100, round(score, 1))), {
        "total_lines": total_lines, "code_lines": code_lines,
        "functions": nfunc, "avg_complexity": round(avg_comp, 2),
        "big_functions": [f["name"] for f in big_funcs],
        "per_file": per_file,
    }


# --------------------------------------------------------------------------
# 维度 2:测试充分性
# --------------------------------------------------------------------------

def run_blackbox_tests(use_go):
    """在 dymsgxtest 目录运行 go test -json,解析事件流统计测试正确性。"""
    data = {"pass_count": 0, "fail_count": 0, "skip_count": 0,
            "test_total": 0, "failed_tests": [], "fail_output": {},
            "build_failed": False, "test_rc": None}
    if not use_go:
        return data
    out, _, rc = run(["go", "test", "-json", "./..."], cwd=XTEST_DIR)
    data["test_rc"] = rc
    passed = failed = skipped = 0
    failed_tests = []
    fail_output = {}
    buf = {}
    for ln in out.splitlines():
        try:
            ev = json.loads(ln)
        except Exception:
            continue
        act = ev.get("Action")
        tname = ev.get("Test") or ""
        if act == "run" and tname:
            buf.setdefault(tname, [])
        elif act == "output" and tname:
            buf.setdefault(tname, []).append(ev.get("Output", ""))
            if len(buf[tname]) > 200:
                buf[tname] = buf[tname][-200:]
        elif act == "pass" and tname:
            passed += 1
        elif act == "fail" and tname:
            failed += 1
            failed_tests.append(tname)
            fail_output[tname] = "".join(buf.get(tname, []))[-800:]
        elif act == "skip" and tname:
            skipped += 1
    data["pass_count"] = passed
    data["fail_count"] = failed
    data["skip_count"] = skipped
    data["test_total"] = passed + failed + skipped
    data["failed_tests"] = failed_tests
    data["fail_output"] = fail_output
    data["build_failed"] = rc != 0 and (passed + failed + skipped) == 0
    return data


def score_testing(use_go):
    """包内测试的实现情况:覆盖率 + 测试数量 + 行数比(不含黑盒正确性)。"""
    data = {"coverage": None, "test_count": 0, "test_lines": 0, "src_lines": 0}
    if use_go:
        # 包内测试覆盖率:在 dymsg 目录跑 go test -cover
        out, _, rc = run(["go", "test", "-cover", "./..."], cwd=DYMSG_DIR)
        cov = None
        m = re.search(r"coverage:\s*([\d.]+)%", out)
        if m:
            cov = float(m.group(1))
        data["coverage"] = cov

    # 包内测试文件统计(仅 dymsg 包内的 _test.go)
    test_files = []
    for name in os.listdir(DYMSG_DIR):
        if name.endswith("_test.go"):
            test_files.append(name)
    test_src = ""
    for name in test_files:
        with open(os.path.join(DYMSG_DIR, name), encoding="utf-8") as f:
            test_src += f.read()
    data["test_count"] = len(re.findall(r"^func\s+Test\w+", test_src, re.M))
    data["test_lines"] = len(test_src.splitlines())
    for name, src in files_cache.items():
        data["src_lines"] += len(src.splitlines())

    cov_score = data["coverage"] if data["coverage"] is not None else 0
    ratio = data["test_lines"] / data["src_lines"] if data["src_lines"] else 0
    ratio_score = min(100, ratio * 120)
    count_score = min(100, data["test_count"] * 1.5)
    score = 0.7 * cov_score + 0.2 * ratio_score + 0.1 * count_score
    data["test_files"] = test_files
    data["test_to_src_ratio"] = round(ratio, 2)
    return max(0, min(100, round(score, 1))), data


def score_correctness(use_go):
    """包对功能的实现正确性:黑盒测试(go test -json)全过为满分。"""
    data = run_blackbox_tests(use_go)
    total = data["test_total"]
    if data["build_failed"] or total == 0:
        score = 0
    else:
        score = 100.0 * (data["pass_count"] + data["skip_count"]) / total
    data["pass_rate"] = round(score, 1)
    return max(0, min(100, round(score, 1))), data


# --------------------------------------------------------------------------
# 维度 3:代码健康度
# --------------------------------------------------------------------------

def score_health(files, use_go):
    data = {}
    penalty = 0.0

    if use_go:
        out, _, rc = run(["gofmt", "-l", "."], cwd=DYMSG_DIR)
        unformatted = [ln for ln in out.splitlines() if ln.strip()]
        data["gofmt_unformatted"] = unformatted
        if unformatted:
            penalty += 20 * len(unformatted)

        vet_out, _, rc = run(["go", "vet", "./..."], cwd=DYMSG_DIR)
        vet_warns = [ln for ln in vet_out.splitlines() if ln.strip()]
        data["vet_warnings"] = vet_warns
        if vet_warns:
            penalty += 15 * len(vet_warns)

    # TODO/FIXME
    todo = 0
    for src in files.values():
        todo += len(re.findall(r"\b(TODO|FIXME)\b", src))
    data["todo_count"] = todo
    penalty += todo * 10

    # panic("todo")
    ptd = 0
    for src in files.values():
        ptd += len(re.findall(r'panic\s*\(\s*"todo"', src))
    data["panic_todo"] = ptd
    penalty += ptd * 10

    # 导出符号无文档注释
    missing_doc = []
    for name, src in files.items():
        clean = src.splitlines()
        for i, ln in enumerate(clean):
            m = re.match(r"^\s*func\s+(\(\w+\s+\*?[\w.]+\)\s*)?([A-Z]\w*)\s*\(", ln)
            m2 = re.match(r"^\s*type\s+([A-Z]\w*)", ln)
            m3 = re.match(r"^\s*var\s+([A-Z]\w*)", ln)
            sym = None
            if m:
                sym = m.group(2)
            elif m2:
                sym = m2.group(1)
            elif m3:
                sym = m3.group(1)
            if sym:
                # 向前找注释(允许空行)
                prev = i - 1
                has_doc = False
                while prev >= 0 and (not clean[prev].strip() or clean[prev].strip().startswith("//")):
                    if clean[prev].strip().startswith("//"):
                        has_doc = True
                    prev -= 1
                if not has_doc:
                    missing_doc.append(f"{name}:{i+1}:{sym}")
    data["undocumented_exported"] = missing_doc
    penalty += len(missing_doc) * 3

    score = 100 - min(penalty, 100)
    return max(0, round(score, 1)), data


# --------------------------------------------------------------------------
# 维度 4:契约一致性(SPEC 与源码)
# --------------------------------------------------------------------------

def _norm_sig(sig):
    """把函数签名规整为可比较形式(去参数、接收者)。"""
    sig = re.sub(r"\s+", " ", sig.strip())
    sig = re.sub(r"^func\s+", "", sig)
    sig = re.sub(r"\([^)]*\)\s*", "", sig, count=1)  # 接收者
    sig = sig.split("(")[0].strip()
    return sig


def score_contract(files):
    spec_path = os.path.join(DYMSG_DIR, "SPEC.md")
    spec = ""
    if os.path.exists(spec_path):
        with open(spec_path, encoding="utf-8") as f:
            spec = f.read()

    # 从 SPEC 提取函数签名(代码块内的 func 行)
    spec_funcs = set()
    for m in re.finditer(r"^func\s+\([^)]*\)\s*([A-Z]\w*)\s*\([^)]*\)", spec, re.M):
        spec_funcs.add(m.group(1))
    for m in re.finditer(r"^func\s+([A-Z]\w*)\s*\([^)]*\)", spec, re.M):
        spec_funcs.add(m.group(1))

    # 源码导出函数
    src_funcs = set()
    for src in files.values():
        for m in re.finditer(r"^func\s+(?:\([^)]*\)\s*)?([A-Z]\w*)\s*\(", src, re.M):
            src_funcs.add(m.group(1))

    missing = sorted(spec_funcs - src_funcs)
    # 源码中 SPEC 未声明的新导出符号(不算扣分,仅记录)
    extra = sorted(src_funcs - spec_funcs)

    # Message 方法(可能分散在多个文件)
    spec_methods = set()
    for m in re.finditer(r"^func\s+\(m \*Message\)\s*([A-Z]\w*)\s*\(", spec, re.M):
        spec_methods.add(m.group(1))
    src_methods = set()
    for src in files.values():
        for m in re.finditer(r"^func\s+\(m \*Message\)\s*([A-Z]\w*)\s*\(", src, re.M):
            src_methods.add(m.group(1))
    method_missing = sorted(spec_methods - src_methods)

    # 哨兵错误
    spec_errors = set()
    for m in re.finditer(r"\b(Err[A-Z]\w*)", spec):
        spec_errors.add(m.group(1))
    src_errors = set()
    for src in files.values():
        for m in re.finditer(r"\b(Err[A-Z]\w*)\s*=\s*errors\.New", src):
            src_errors.add(m.group(1))
    error_missing = sorted(spec_errors - src_errors)

    total_declared = max(len(spec_funcs), 1)
    matched = len(spec_funcs) - len(missing)
    func_score = 100 * matched / total_declared
    method_score = 100 if not method_missing else 50
    err_score = 100 if not error_missing else max(0, 100 - 20 * len(error_missing))
    score = 0.6 * func_score + 0.2 * method_score + 0.2 * err_score

    return max(0, min(100, round(score, 1))), {
        "spec_funcs": sorted(spec_funcs),
        "src_funcs": sorted(src_funcs),
        "missing_funcs": missing,
        "extra_funcs": extra,
        "missing_methods": method_missing,
        "spec_errors": sorted(spec_errors),
        "src_errors": sorted(src_errors),
        "missing_errors": error_missing,
    }


# --------------------------------------------------------------------------
# 维度 5:并发与安全
# --------------------------------------------------------------------------

def score_concurrency(files):
    all_src = "\n".join(files.values())
    data = {}

    data["mutex"] = len(re.findall(r"sync\.(Mutex|RWMutex)", all_src))
    data["atomic"] = len(re.findall(r"sync/atomic|atomic\.", all_src))

    # 全局可变状态(var xxx = map[] ...)
    globals_maps = re.findall(r"var\s+(\w+)\s*=\s*map\[", all_src)
    data["global_maps"] = globals_maps

    # 全局 map 是否被锁保护(近似:检查含 mutex 的文件同时含该 map 的锁)
    # 简化:检查 registry 相关代码里 RWMutex 的存在(已知设计)
    reg_has_lock = bool(re.search(r"registryMu\.(RLock|Lock)", files.get("dymsg.go", "")))
    data["registry_locked"] = reg_has_lock

    # 并发测试
    test_src = ""
    for name in os.listdir(XTEST_DIR):
        if name.endswith("_test.go"):
            with open(os.path.join(XTEST_DIR, name), encoding="utf-8") as f:
                test_src += f.read()
    conc_tests = re.findall(r"^func\s+(TestConcurrent\w*|TestRegisterConcurrent\w*)", test_src, re.M)
    data["concurrency_tests"] = conc_tests

    score = 40.0
    if data["mutex"] > 0:
        score += 25
    if reg_has_lock:
        score += 20
    if len(conc_tests) > 0:
        score += 15
    if data["global_maps"] and not reg_has_lock:
        score -= 20
    return max(0, min(100, round(score, 1))), data


# --------------------------------------------------------------------------
# 维度 6:错误处理
# --------------------------------------------------------------------------

def score_errors(files):
    data = {}
    exported_funcs = []
    err_funcs = 0
    # 逐行匹配(避免 finditer + 可选组在多行文本上的兼容问题)
    sig_re = re.compile(r"^func\s+(?:\([^)]*\)\s*)?([A-Z]\w*)\s*\([^)]*\)\s*([^{]*)")
    for src in files.values():
        for ln in src.splitlines():
            m = sig_re.match(ln)
            if m:
                exported_funcs.append(m.group(1))
                if "error" in m.group(2):
                    err_funcs += 1
    data["exported_functions"] = len(exported_funcs)
    data["error_returning"] = err_funcs
    err_ratio = err_funcs / len(exported_funcs) if exported_funcs else 0

    # 吞错(_ = xxx() / _, _ =)
    swallow = 0
    for src in files.values():
        swallow += len(re.findall(r"(?:_,\s*_|_)\s*=\s*\w+\(", src))
    data["swallowed_errors"] = swallow

    panics = 0
    for src in files.values():
        panics += len(re.findall(r"\bpanic\(", src))
    data["panics"] = panics

    score = err_ratio * 60
    score += max(0, 40 - swallow * 4 - panics * 5)
    return max(0, min(100, round(score, 1))), data


# --------------------------------------------------------------------------
# 维度 7:性能(benchmark)
# --------------------------------------------------------------------------

def score_performance(use_go):
    data = {}
    if not use_go:
        return 0.0, {"skipped": True}

    out, _, rc = run(["go", "test", "-bench", ".", "-benchmem",
                      "-benchtime=1s", "-run", "^$", "./..."], cwd=XTEST_DIR)
    results = {}
    for m in re.finditer(
        r"^(Benchmark\w+)-?\d*\s+\d+\s+([\d.]+)\s*(ns/op|µs/op|ms/op)\s+([\d.]+)\s+B/op\s+([\d.]+)\s+allocs/op",
        out, re.M):
        name, val, unit, bytes_alloc, allocs = m.groups()
        ns = float(val) * {"ns/op": 1, "µs/op": 1000, "ms/op": 1e6}[unit]
        results[name] = {"ns_op": round(ns, 1), "B_op": float(bytes_alloc),
                         "allocs_op": int(float(allocs))}
    data["benchmarks"] = results

    # ---- 机器无关度量 ----
    # 时间以同机 BenchmarkReference 归一化(相对倍率);内存 allocs/B 本身确定性。
    # 连续评分:每项 score = 100 - (测量值/阈值)*80,裁剪到 [0,100]。
    # 阈值取当前实现的 ~2.67x,使当前实现每项约 70 分。
    ref_ns = results.get("BenchmarkReference", {}).get("ns_op")

    # 相对时间阈值(相对参照倍率)
    time_thr = {
        "BenchmarkNew": 0.85, "BenchmarkSetScalar": 0.96, "BenchmarkGetScalar": 0.83,
        "BenchmarkSetNested": 1.36, "BenchmarkGetNested": 1.28,
        "BenchmarkEncodeProto": 2.80, "BenchmarkDecodeProto": 10.0,
        "BenchmarkEncodeJSON": 28.7, "BenchmarkDecodeJSON": 54.7,
        "BenchmarkRoundTripProto": 13.6, "BenchmarkCopyMessage": 4.9,
    }
    # allocs/op 阈值
    alloc_thr = {
        "BenchmarkNew": 5.33, "BenchmarkSetScalar": 8, "BenchmarkGetScalar": 8,
        "BenchmarkSetNested": 8, "BenchmarkGetNested": 8,
        "BenchmarkEncodeProto": 21.3, "BenchmarkDecodeProto": 72,
        "BenchmarkEncodeJSON": 125.3, "BenchmarkDecodeJSON": 138.7,
        "BenchmarkRoundTripProto": 93.3, "BenchmarkCopyMessage": 21.3,
    }
    # B/op 阈值
    bytes_thr = {
        "BenchmarkNew": 768, "BenchmarkSetScalar": 149.3, "BenchmarkGetScalar": 277.3,
        "BenchmarkSetNested": 256, "BenchmarkGetNested": 384,
        "BenchmarkEncodeProto": 490.7, "BenchmarkDecodeProto": 2282.7,
        "BenchmarkEncodeJSON": 2858.7, "BenchmarkDecodeJSON": 5672,
        "BenchmarkRoundTripProto": 2773.3, "BenchmarkCopyMessage": 1365.3,
    }

    time_scores, mem_scores = [], []
    over_alloc, over_bytes = [], []
    relative = {}
    for name, thr in time_thr.items():
        if name not in results:
            continue
        r = results[name]
        ratio = r["ns_op"] / ref_ns if ref_ns else 0.0
        relative[name] = round(ratio, 2)
        time_scores.append(max(0.0, min(100.0, 100 - (ratio / thr) * 80)))

        a_ratio = r["allocs_op"] / alloc_thr[name]
        b_ratio = r["B_op"] / bytes_thr[name]
        mem_scores.append(max(0.0, min(100.0, 100 - max(a_ratio, b_ratio) * 80)))
        if r["allocs_op"] > alloc_thr[name]:
            over_alloc.append(f"{name}({r['allocs_op']}>{alloc_thr[name]:.1f})")
        if r["B_op"] > bytes_thr[name]:
            over_bytes.append(f"{name}({r['B_op']:.0f}>{bytes_thr[name]:.1f})")

    time_score = sum(time_scores) / len(time_scores) if time_scores else 0.0
    mem_score = sum(mem_scores) / len(mem_scores) if mem_scores else 0.0

    data["relative_time"] = relative
    data["time_score"] = round(time_score, 1)
    data["mem_score"] = round(mem_score, 1)
    data["over_alloc"] = over_alloc
    data["over_bytes"] = over_bytes
    data["ref_ns_op"] = ref_ns

    score = 0.5 * time_score + 0.5 * mem_score
    return max(0, min(100, round(score, 1))), data


# --------------------------------------------------------------------------
# 主流程
# --------------------------------------------------------------------------

files_cache = None


# --------------------------------------------------------------------------
# 评分与输出
# --------------------------------------------------------------------------

# 各维度达标阈值(score 为 0.0-1.0)
THRESHOLDS = {
    "size": 0.60,
    "testing": 0.70,
    "correctness": 1.0,
    "health": 0.90,
    "contract": 0.80,
    "concurrency": 0.80,
    "errors": 0.80,
    "performance": 0.70,
}

DIM_ORDER = ["size", "testing", "correctness", "health", "contract", "concurrency", "errors", "performance"]


def build_reason(name, data):
    """根据维度数据生成描述文本。"""
    if name == "size":
        return (f"total_lines={data.get('total_lines')}, code_lines={data.get('code_lines')}, "
                f"functions={data.get('functions')}, avg_complexity={data.get('avg_complexity')}")
    if name == "testing":
        cov = data.get("coverage")
        return (f"coverage={cov}% (in-package go test -cover), tests={data.get('test_count')}, "
                f"test/src_ratio={data.get('test_to_src_ratio')}")
    if name == "correctness":
        return (f"blackbox pass={data.get('pass_count')}/{data.get('test_total')} "
                f"(fail={data.get('fail_count')}, skip={data.get('skip_count')}), "
                f"pass_rate={data.get('pass_rate')}%, "
                f"failed={data.get('failed_tests')}")
    if name == "health":
        return (f"gofmt_unformatted={len(data.get('gofmt_unformatted') or [])}, "
                f"vet_warnings={len(data.get('vet_warnings') or [])}, "
                f"todo={data.get('todo_count')}, panic_todo={data.get('panic_todo')}, "
                f"undocumented_exported={len(data.get('undocumented_exported') or [])}")
    if name == "contract":
        return (f"missing_funcs={data.get('missing_funcs')}, "
                f"missing_methods={data.get('missing_methods')}, "
                f"missing_errors={data.get('missing_errors')}")
    if name == "concurrency":
        return (f"mutex={data.get('mutex')}, atomic={data.get('atomic')}, "
                f"global_maps={data.get('global_maps')}, "
                f"registry_locked={data.get('registry_locked')}, "
                f"concurrency_tests={len(data.get('concurrency_tests') or [])}")
    if name == "errors":
        return (f"exported={data.get('exported_functions')}, "
                f"error_returning={data.get('error_returning')}, "
                f"swallowed_errors={data.get('swallowed_errors')}, panics={data.get('panics')}")
    if name == "performance":
        return (f"time_score={data.get('time_score')}, mem_score={data.get('mem_score')}, "
                f"over_alloc={data.get('over_alloc')}, over_bytes={data.get('over_bytes')}, "
                f"ref_ns={data.get('ref_ns_op')}")
    return ""


def main():
    global files_cache
    files_cache = read_go_files()

    use_go = USE_GO
    quiet = QUIET
    output = OUTPUT

    results = {}

    s, d = score_size(files_cache)
    results["size"] = (s, d)
    if not quiet:
        print(f"size        : {s}")

    s, d = score_testing(use_go)
    results["testing"] = (s, d)
    if not quiet:
        print(f"testing     : {s}  (coverage={d.get('coverage')}%)")

    s, d = score_correctness(use_go)
    results["correctness"] = (s, d)
    if not quiet:
        print(f"correctness : {s}  (pass={d.get('pass_count')}/{d.get('test_total')})")

    s, d = score_health(files_cache, use_go)
    results["health"] = (s, d)
    if not quiet:
        print(f"health      : {s}")

    s, d = score_contract(files_cache)
    results["contract"] = (s, d)
    if not quiet:
        print(f"contract    : {s}  (missing={d['missing_funcs']})")

    s, d = score_concurrency(files_cache)
    results["concurrency"] = (s, d)
    if not quiet:
        print(f"concurrency : {s}")

    s, d = score_errors(files_cache)
    results["errors"] = (s, d)
    if not quiet:
        print(f"errors      : {s}")

    s, d = score_performance(use_go)
    results["performance"] = (s, d)
    if not quiet:
        print(f"performance : {s}")

    lines = []
    for name in DIM_ORDER:
        s0, data = results[name]
        score01 = round(s0 / 100.0, 2)
        resolved = score01 >= THRESHOLDS[name]
        lines.append({
            "resolved": resolved,
            "score": score01,
            "reason": build_reason(name, data),
        })

    # 每维度一行 JSON(JSONL)
    text = "\n".join(json.dumps(ln, ensure_ascii=False) for ln in lines) + "\n"
    if output:
        with open(output, "w", encoding="utf-8") as f:
            f.write(text)
        if not quiet:
            print(f"written to {output}")
    else:
        sys.stdout.write(text)


if __name__ == "__main__":
    main()
