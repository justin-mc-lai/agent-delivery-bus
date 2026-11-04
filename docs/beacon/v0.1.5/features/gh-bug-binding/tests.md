---
slug: gh-bug-binding
version: v0.1.5
materials_status: current
---

# Tests: gh-bug-binding

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-GB-001 | behavior | go -C cmd/adbd test -run TestParseBugBindingLabels | exit_code_0；合法标签解出 slug 与 AC；no-ac 识别为豁免；非法、缺失、重复判未绑定 |
| AC-002 | TC-GB-002 | behavior | go -C cmd/adbd test -run TestBoundBugDispatches | exit_code_0；sdd=gated；trace.ac_ref 正确；派活文本含 AC 引用与回归句 |
| AC-003 | TC-GB-003 | behavior | go -C cmd/adbd test -run TestUnboundBugIsRefused | exit_code_0；reason=issue_unbound；零 paseo 调用；条目留队列 |
| AC-004 | TC-GB-004 | behavior | go -C cmd/adbd test -run TestNoAcExemptionDispatches | exit_code_0；sdd=guard_only；reason=issue_exempt_no_ac |
| AC-005 | TC-GB-005 | behavior | go -C cmd/adbd test -run TestBoundBugNeedsFrozenTruth | exit_code_0；未冻结得 truth_not_frozen |
| AC-006 | TC-GB-006 | behavior | go -C cmd/adbd test -run TestBoundBugMissingACMismatch | exit_code_0；AC 不存在得 ac_not_found |
| AC-007 | TC-GB-007 | behavior | go -C cmd/adbd test -run TestBindingGateCanBeDisabled | exit_code_0；binding=off 时回落 guard_only 并记 binding_off |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-007 | TC-GB-EXEC-001 | behavior | go -C cmd/adbd test ./... | exit_code_0（含既有 134 个用例） |
| AC-007 | TC-GB-EXEC-002 | behavior | python3 -m pytest -q tests/ | exit_code_0 |
