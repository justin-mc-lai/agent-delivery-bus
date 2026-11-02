---
slug: worker-report
version: v0.1.5
materials_status: current
---

# Tests: worker-report

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-WR-001 | behavior | `go test ./... -run ReconcileCountsBindings` | exit_code_0；checked/done/running 计数正确；无绑定 checked=0 |
| AC-002 | TC-WR-002 | behavior | `go test ./... -run RunningToIdleRecordsReport` | exit_code_0；running→idle 写一行回执；同 agent 只写一次 |
| AC-003 | TC-WR-003 | behavior | `go test ./... -run ExecStateFileMapping` | exit_code_0；skill 线程映射为 req:<project>:<slug>；非 skill 不写；原子写入 |
| AC-004 | TC-WR-004 | behavior | `python3 -m pytest -q tests/test_requirement_index.py -k exec_state_source` | exit_code_0；索引 exec_state 取文件值；缺失仍 unknown；truth_stage 不变 |
| AC-005 | TC-WR-005 | behavior | `go test ./... -run ReconcileIdempotent` | exit_code_0；第二次 reconcile done=0 且回执行数不变 |
| AC-006 | TC-WR-006 | behavior | `go test ./... -run PaseoUnavailableNoMutation` | exit_code_0；paseo_unavailable + 退出码 2；绑定/回执/exec 均未变 |
| AC-007 | TC-WR-007 | behavior | `go test ./... -run DoneBindingNotStalled` | exit_code_0；done 绑定不再产出 stalled |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-WR-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含此前 51 个用例） |
| AC-001 | TC-WR-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0（Python 侧零回归） |
