# AGENTS.md

This repo is an evaluation workspace for the `dymsg` Go library (dynamic message
encode/decode driven by runtime schemas instead of compiled structs). The
implementation is expected to satisfy a written spec and a black-box test suite.

## Layout

- `workspace/` — the `dymsg` module (module name `dymsg`, go 1.24). This is the
  implementation to work on. Split across: `dymsg.go` (types, sentinel errors,
  registry, `ParseSchema`), `message.go` (`Message` + `Get/Set/Append/Clear/Has`),
  `value.go` (`Value` accessors), `path.go` (path parser), `slice.go` (typed-slice
  helpers), `convert.go` (type coercion), `copy.go` (deep copy), `json.go` and
  `proto.go` (codecs).
- `test_files/` — the `eval` module: an **external (black-box) test package** that
  imports `dymsg`. Contains `dymsg_test.go` (the acceptance tests + shared
  `richSchema`), `bench_test.go` (perf benchmarks), `go.mod`, `readme.md`.
- `prompt.md` — the task prompt; identical to the spec in `test_files/readme.md`.
- `test_by_code.py` — the scoring harness (see "Evaluation" below).

## Critical gotcha: broken module replace (tests won't run as-is)

`test_files/go.mod` points at `replace dymsg => /workspace`, an absolute path that
does **not** exist on this machine. `go test ./...` from `test_files/` fails with
`replacement directory /workspace does not exist`. The file already has a commented
`// replace dymsg => ../workspace` — swap the two lines to run tests locally. (The
eval harness mounts the repo at `/workspace`, so keep the absolute path working for it.)

## The spec is the only authority

`test_files/readme.md` (== `prompt.md`) is the authoritative contract: field types,
`MessageSchema`/`FieldSchema`, the exact `Message`/`Value` method surface, sentinel
errors, JSON config format, field path syntax, presence model, and JSON/proto codec
semantics. The harness extracts exported function signatures from this file and
compares against the source. Do not add/rename exported API or define the sentinel
errors any way other than `errors.New` (the contract check greps for it).

## How to verify

- `workspace/` has no tests; `go test ./...` there just builds. The acceptance tests
  run from `test_files/`: `go test ./...` (after fixing the replace line above).
- Overall score: `python3 test_by_code.py` from the repo root (writes
  `test_files/code_result.json`).
- Keep `workspace/` gofmt-clean (`gofmt -l` empty) and `go vet ./...` clean.

## Evaluation scoring (test_by_code.py) — non-obvious rules

Six equal-weight dimensions: `size`, `testing`, `correctness`, `health`,
`contract`, `performance`. "Resolved" requires `correctness` == 1.0 (all black-box
tests pass).

- **Correctness** also covers concurrency: the black-box suite runs with
  `go test -race`, so a data race in the implementation fails tests and drops this
  dimension. There is no separate concurrency score or source-lock grep.
- **Testing**: measures coverage of the `dymsg` source by `workspace/`'s **own**
  tests (`workspace/*_test.go`), not the `test_files/` black-box suite. `workspace/`
  currently has no tests, so this dimension scores 0 — improve it by adding unit
  tests inside `workspace/`.
- **Health**: penalizes unformatted files, vet warnings, `TODO`/`FIXME` comments,
  and **exported symbols without doc comments**.
- **Performance**: benchmarks live in `test_files/bench_test.go` (registered once via
  `init()`); thresholds (`time_thr`/`alloc_thr`/`bytes_thr`) are hard-coded in
  `test_by_code.py`, normalized against `BenchmarkReference`.

## Semantics that are easy to get wrong (spec details in readme.md)

- `Value` distinguishes not-found / exists-but-unset / set (incl. zero value) via
  `Err()`/`Exists()`/`IsSet()`; getters return zero values and never error.
- All `Set`/`Append` writes deep-copy (nested messages, repeated slices, `[]byte`).
- `Register`/`New` are concurrency-safe; individual `Message` instances are **not**
  (caller must lock). The tests include concurrent `Register` cases.
- Protobuf is a self-implemented subset — no `google.golang.org/protobuf` dependency.
