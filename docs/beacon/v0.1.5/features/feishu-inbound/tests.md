---
slug: feishu-inbound
version: v0.1.5
materials_status: current
---

# Tests: feishu-inbound

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-FI-001 | behavior | `go test ./... -run FeishuFeedMapsText` | exit_code_0；accepted=1；intake 收到 feishu:oc_x 与文本 |
| AC-002 | TC-FI-002 | behavior | `go test ./... -run FeishuFeedSkipsNonText` | exit_code_0；skipped 计数 + unsupported_message_type |
| AC-003 | TC-FI-003 | behavior | `go test ./... -run FeishuFeedSkipsBot` | exit_code_0；skipped + from_bot |
| AC-004 | TC-FI-004 | behavior | `go test ./... -run FeishuFeedBadLines` | exit_code_0；invalid_json / missing_field 逐行跳过，后续行仍处理 |
| AC-005 | TC-FI-005 | behavior | `go test ./... -run FeishuFeedDedupAndQueue` | exit_code_0；重复 message_id 只处理一次；队列/直通结果符合预期 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-FI-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含此前 84 个用例） |
| AC-001 | TC-FI-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |
