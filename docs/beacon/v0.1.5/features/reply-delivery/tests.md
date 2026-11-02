---
slug: reply-delivery
version: v0.1.5
materials_status: current
---

# Tests: reply-delivery

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-RD-001 | behavior | `go test ./... -run ReplyDeliversOnFeishu` | exit_code_0；fake 发送器收到 `im +messages-send --chat-id --text --as bot --idempotency-key` |
| AC-002 | TC-RD-002 | behavior | `go test ./... -run ReplyTraceFields` | exit_code_0；trace 含 delivered 与 delivery_reason，其余字段不变 |
| AC-003 | TC-RD-003 | behavior | `go test ./... -run ReplySendFailureKeepsDecision` | exit_code_0；发送失败仍 applied，reason=send_failed，退出码 0 |
| AC-004 | TC-RD-004 | behavior | `go test ./... -run ReplyIdempotencyKeyStable` | exit_code_0；同一 message_id 两次调用携带相同键 |
| AC-005 | TC-RD-005 | behavior | `go test ./... -run ReplySkipsNonFeishu` | exit_code_0；skill/gh 线程不调用发送器，reason=unsupported_channel |
| AC-006 | TC-RD-006 | behavior | `go test ./... -run ReplyDisabledByDefault` | exit_code_0；未配置时不调用发送器，reason=not_configured |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-RD-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含此前 68 个用例） |
| AC-001 | TC-RD-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |
