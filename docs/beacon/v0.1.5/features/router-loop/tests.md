---
slug: router-loop
version: v0.1.5
materials_status: current
---

# Tests: router-loop

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-RL-001 | behavior | `go test ./... -run BatchDrainsQueueOnce` | exit_code_0；3 条队列条目使决策器被调用 1 次；输出 `batch=3 decided=3`；空队列 `batch=0` 且调用 0 次 |
| AC-002 | TC-RL-002 | behavior | `go test ./... -run FourWayDecisionOnly` | exit_code_0；四种合法决策各自落 trace；非法决策记 decision_invalid 且条目标 failed |
| AC-003 | TC-RL-003 | behavior | `go test ./... -run RouterToolsetIsEmpty` | exit_code_0；`--list-tools` 输出 `[]`；源码扫描断言 route 实现不引用 os/exec |
| AC-004 | TC-RL-004 | behavior | `go test ./... -run DecisionTraceIsIdempotent` | exit_code_0；每条决定字段齐全；重复 drain 不产生重复 trace 行 |
| AC-005 | TC-RL-005 | behavior | `go test ./... -run DispatchRetryBackoffEscalates` | exit_code_0；失败 3 次后转 escalate 且 reason_code=dispatch_attempts_exhausted |
| AC-006 | TC-RL-006 | behavior | `go test ./... -run RestartReplayNoDoubleDispatch` | exit_code_0；中途失败后队列保留未处理项；已 applied 的不再派活 |
| AC-007 | TC-RL-007 | behavior | `go test ./... -run UntrustedGuardAndLimits` | exit_code_0；trace 标 untrusted=true；派活文本以 UNTRUSTED_INPUT 开头；高危动词转 escalate；超长 reply 记 reply_too_long |
| AC-008 | TC-RL-009 | behavior | `go test ./... -run LLMDeciderContract` | exit_code_0；httptest 服务收到**一个**请求且 body 含整批；正常 JSON → 决策生效；坏 JSON/未知决策 → decision_invalid + failed；非 2xx → 回退规则且 trace 记 llm_fallback_rules；LLM 把高危文本判成 route 时被代码翻回 escalate |

## State machine slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-006 | TC-RL-008 | behavior | `go test ./... -run QueueStateWalks` | exit_code_0；queued→claimed→decided→applied 走通；failed→escalated 走通 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-RL-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含 M1 的 8 个用例） |
| AC-001 | TC-RL-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0（Python 侧零回归） |
