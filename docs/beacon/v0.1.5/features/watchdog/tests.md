---
slug: watchdog
version: v0.1.5
materials_status: current
---

# Tests: watchdog

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-WD-001 | behavior | `go test ./... -run StalledByProgressNotLiveness` | exit_code_0；20 分钟无动作输出 stalled；有空闲进程但不影响判定 |
| AC-002 | TC-WD-002 | behavior | `go test ./... -run RouterHeartbeatExpiry` | exit_code_0；心跳过期或缺失输出 router_stalled 并写 trace |
| AC-003 | TC-WD-003 | behavior | `go test ./... -run SilentWhenClean` | exit_code_0；输出仅 NOTHING；watchdog trace 文件不存在或未变 |
| AC-004 | TC-WD-004 | behavior | `go test ./... -run InterventionTraceShape` | exit_code_0；trace 六字段齐全；无 --apply 时 action=observed |
| AC-005 | TC-WD-005 | behavior | `go test ./... -run RepeatStallEscalation` | exit_code_0；连续三次 stalled 转 repeat_stall；6 小时内只写一次 |
| AC-006 | TC-WD-006 | behavior | `go test ./... -run AgentPlistGeneration` | exit_code_0；plist 含 StartInterval/WorkingDirectory；不写文件、不调用 launchctl |
| AC-007 | TC-WD-007 | behavior | `go test ./... -run AgentStatusReadOnly` | exit_code_0；plist 缺失时报告 not_installed；launchctl 不可用时 reason_code=launchctl_unavailable；不改系统 |

## State machine slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-WD-008 | behavior | `go test ./... -run WatchStateWalks` | exit_code_0；idle→scanning→clean 与 idle→scanning→alerted→acted 均走通 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-WD-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含 M1/M2 的 26 个用例） |
| AC-001 | TC-WD-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0（Python 侧零回归） |
