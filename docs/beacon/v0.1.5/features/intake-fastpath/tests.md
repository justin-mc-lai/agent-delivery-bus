---
slug: intake-fastpath
version: v0.1.5
materials_status: current
---

# Tests: intake-fastpath

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-IF-001 | unit | `cd cmd/adbd && go build ./...` | exit_code_0；`adbd --version` 输出非空；`go.mod` 存在 |
| AC-002 | TC-IF-002 | behavior | `go test ./... -run FastpathForwardsToBoundWorker` | exit_code_0；真实记录型脚本进程收到 argv `send <agent-id> <text>`；输出 route=fast |
| AC-003 | TC-IF-003 | unit | `go test ./... -run DuplicateMessageDeduped` | exit_code_0；第二次 route=deduped；runner 只被调用一次 |
| AC-004 | TC-IF-004 | unit | `go test ./... -run LeaseRenewAndExpiry` | exit_code_0；转发后续租；临期输出 lease=expiring；过期后 route=slow |
| AC-005 | TC-IF-005 | unit | `go test ./... -run SlowPathAndRejected` | exit_code_0；无绑定 route=slow 且 runner 未调用；未知来源 route=rejected + source_not_allowed |
| AC-006 | TC-IF-006 | unit | `go test ./... -run NoCrossThreadLeak` | exit_code_0；两个 thread 交叉投递互不串；released 复活返回 binding_released |

## State machine slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-004 | TC-IF-007 | unit | `go test ./... -run LeaseExpiryWalksToUnbound` | exit_code_0；bound→leasing→expiring→unbound 走通 |
| AC-006 | TC-IF-008 | unit | `go test ./... -run IllegalTransitionsRejected` | exit_code_0；released→bound、unbound→leasing、expiring→released 全部被拒 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-IF-EXEC-001 | behavior | `cd cmd/adbd && go vet ./... && go test ./...` | exit_code_0 |
| AC-001 | TC-IF-EXEC-002 | integration | `python3 -m pytest -q tests/ --tb=short` | exit_code_0（Python 侧零回归） |
