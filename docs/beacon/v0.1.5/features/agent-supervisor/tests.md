---
slug: agent-supervisor
version: v0.1.5
materials_status: current
---

# Tests: agent-supervisor

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-AS-001 | behavior | `go test ./... -run WorkerGetsOwnershipLabel` | exit_code_0；fake paseo 收到 `--label adbd.item=<thread>`；补标走 `agent update --label` |
| AC-002 | TC-AS-002 | behavior | `go test ./... -run AdoptDryRunClassifies` | exit_code_0；owned/foreign 分类正确；无写操作、无 archive 调用 |
| AC-003 | TC-AS-003 | behavior | `go test ./... -Run AdoptApplyIdempotent` | exit_code_0；首次打标，二次不打；foreign 打 `adbd.foreign=1` |
| AC-004 | TC-AS-004 | behavior | `go test ./... -run SuperviseClassifiesByFourFacts` | exit_code_0；七种状态各一条；dry-run 无动作；未知状态输出 unknown |
| AC-005 | TC-AS-005 | behavior | `go test ./... -run PermissionPolicyAllowsAndEscalates` | exit_code_0；普通工具 allow，高危工具 escalate；审计行字段齐全 |
| AC-006 | TC-AS-006 | behavior | `go test ./... -run OrphanProbesOnceThenFails` | exit_code_0；首轮一次探针，次轮转 failed，不重复探针 |
| AC-007 | TC-AS-007 | behavior | `go test ./... -run TerminalItemsArchived` | exit_code_0；done/failed 调 archive 一次；无 delete；再次 supervise 无动作 |
| AC-008 | TC-AS-008 | behavior | `go test ./... -run MaxActiveBlocksNewWorker` | exit_code_0；达到上限时 route 返回 max_active_reached 且队列保留 |
| AC-009 | TC-AS-009 | behavior | `go test ./... -run InboxOnlyNeedsHuman` | exit_code_0；只列两类并按等待时长排序；空时输出 NOTHING |
| AC-009 | TC-AS-011 | behavior | `go test ./... -run InboxShowsEscalations` | exit_code_0；24h 内 escalate 进 inbox 并标注来源；窗口外不出现；三类皆空时输出 NOTHING |

## State machine slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-006 | TC-AS-010 | behavior | `go test ./... -run SupervisorStateWalk` | exit_code_0；working→grace→orphan→failed 走通 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-AS-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含此前 94 个用例） |
| AC-001 | TC-AS-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0（Python 侧零回归） |
