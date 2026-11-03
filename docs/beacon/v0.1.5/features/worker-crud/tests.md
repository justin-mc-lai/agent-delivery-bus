---
slug: worker-crud
version: v0.1.5
materials_status: current
---

# Tests: worker-crud

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-WC-001 | behavior | `go test ./... -run WorkerListFilters` | exit_code_0；project 由最长前缀匹配；三种过滤器可叠加；未匹配输出 `-` |
| AC-002 | TC-WC-002 | behavior | `go test ./... -run WorkerShowDetail` | exit_code_0；九个字段齐全；不存在时 agent_not_found 且退出码 2 |
| AC-003 | TC-WC-003 | behavior | `go test ./... -run WorkerCreateInProject` | exit_code_0；fake paseo 收到 `--cwd <项目绝对路径>` 与 `--label adbd.item=<item>`；未登记项目 exit 2 且零调用 |
| AC-004 | TC-WC-004 | behavior | `go test ./... -run WorkerUpdateSupportedFields` | exit_code_0；name/label/mode 各自映射正确；cwd/provider/model 返回 field_not_updatable |
| AC-005 | TC-WC-005 | behavior | `go test ./... -run WorkerArchiveOwnershipRules` | exit_code_0；无归属拒绝；重复归档 exit 0；审计行写入 |
| AC-005 | TC-WC-009 | behavior | `go test ./... -run ArchiveReleasesBindings` | exit_code_0；归档后状态文件里指向该 agent 的绑定变为 released；再次直通不再命中该 agent |
| AC-006 | TC-WC-006 | behavior | `go test ./... -run NoHardDelete` | exit_code_0；`worker delete` 返回 delete_not_supported；源码扫描无硬删调用 |
| AC-007 | TC-WC-007 | behavior | `go test ./... -run UnknownOwnershipBlocked` | exit_code_0；update/archive 均拒绝且无 paseo 写调用；list 仍展示 |
| AC-008 | TC-WC-008 | behavior | `go test ./... -run WorkspacesLongestPrefix` | exit_code_0；最长前缀生效；重复 slug、缺失路径、非法 JSON 返回 workspaces_invalid |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-WC-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含此前 104 个用例） |
| AC-001 | TC-WC-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0（Python 侧零回归） |
