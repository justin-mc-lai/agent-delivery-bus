---
slug: thread-workdir
version: v0.1.5
materials_status: current
---

# Tests: thread-workdir

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-TW-001 | behavior | `go test ./... -run ThreadWorkdirKey` | exit_code_0；skill → project；gh → owner/repo；其它 → 空 |
| AC-002 | TC-TW-002 | behavior | `go test ./... -run ProjectsRegistryTolerant` | exit_code_0；缺失/非法 JSON/非数组均返回空映射且不 panic |
| AC-003 | TC-TW-003 | behavior | `go test ./... -run CwdFromRegistry` | exit_code_0；fake paseo 收到 `--cwd <项目绝对路径>`；相对 repo 以 root 解析 |
| AC-004 | TC-TW-004 | behavior | `go test ./... -run WorkdirFallbackRecorded` | exit_code_0；未登记线程 `--cwd` 为默认值，并写一条 `workdir_fallback` |
| AC-005 | TC-TW-005 | behavior | `go test ./... -run BoundThreadSkipsResolution` | exit_code_0；复用绑定不产生新的 fallback 记录 |
| AC-006 | TC-TW-006 | behavior | `go test ./... -run WorkdirEndToEnd` | exit_code_0；skill 与 gh 两条线程各自落到正确目录 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-TW-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含此前 60 个用例） |
| AC-001 | TC-TW-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |
