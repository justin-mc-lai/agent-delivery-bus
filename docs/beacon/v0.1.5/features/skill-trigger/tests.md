---
slug: skill-trigger
version: v0.1.5
materials_status: current
---

# Tests: skill-trigger

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-ST-001 | behavior | `go test ./... -run SkillDocContract` | exit_code_0；`skills/agent-delivery-bus/SKILL.md` 含固定标题与推荐命令形态，且不再教人拼 `adbd intake` |
| AC-002 | TC-ST-002 | behavior | `go test ./... -run FeedDefaultsAndIdempotency` | exit_code_0；未给 message-id 时按键生成；两次相同输入第二次 route=deduped |
| AC-003 | TC-ST-003 | behavior | `go test ./... -run ThreadShapeAndIsolation` | exit_code_0；thread 形如 `skill:<project>:<slug>`；非法字符返回 thread_invalid 且不写队列；不同 project 互不影响 |
| AC-004 | TC-ST-004 | behavior | `go test ./... -run FeedEndToEndTrace` | exit_code_0；decisions.jsonl 恰好一行；runner 收到以 UNTRUSTED_INPUT 开头的文本；输出 route=fast/slow/deduped |
| AC-005 | TC-ST-005 | behavior | `go test ./... -Run FeedErrorCodes` | exit_code_0；text_empty / thread_invalid / dispatch_failed 三种码与退出码 2 均正确 |
| AC-006 | TC-ST-006 | behavior | `go test ./... -run FeedNeedsNoDaemon` | exit_code_0；无托管环境下直接执行两次，结果一致 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-ST-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含 M1/M2/M3 的 36 个用例） |
| AC-001 | TC-ST-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0（Python 侧零回归） |
