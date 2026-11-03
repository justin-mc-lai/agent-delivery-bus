---
slug: dispatch-liveness
version: v0.1.5
materials_status: current
---

# Tests: dispatch-liveness

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-DL-001 | behavior | go -C cmd/adbd test -run TestWorkerUsesWorktreeWorkspace | exit_code_0；默认 argv 含 --new-workspace worktree；local 可配；两者都带 --cwd |
| AC-002 | TC-DL-002 | behavior | go -C cmd/adbd test -run TestDispatchDetectsLiveTurn | exit_code_0；进度增长判活；无增长超时判死 |
| AC-003 | TC-DL-003 | behavior | go -C cmd/adbd test -run TestDispatchRetriesDeadSend | exit_code_0；判死后重发；次数受 adbd_worker_dispatch_retries 限制 |
| AC-004 | TC-DL-004 | behavior | go -C cmd/adbd test -run TestDispatchDeadRequeues | exit_code_0；trace 记 dispatch_dead；条目留队列；未写绑定；退出码 0 |
| AC-005 | TC-DL-005 | behavior | go -C cmd/adbd test -run TestDispatchProbeFailureIsNotSuccess | exit_code_0；探测失败记 dispatch_unverified，不判活 |
| AC-006 | TC-DL-006 | behavior | go -C cmd/adbd test -run TestDispatchCheckCanBeDisabled | exit_code_0；off 时跳过检查并记 dispatch_unchecked |
| AC-007 | TC-DL-007 | behavior | go -C cmd/adbd test ./... | exit_code_0（含既有 126 个用例） |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-007 | TC-DL-EXEC-001 | behavior | go -C cmd/adbd test ./... | exit_code_0 |
| AC-007 | TC-DL-EXEC-002 | behavior | python3 -m pytest -q tests/ | exit_code_0 |
