---
slug: install
version: v0.1.5
materials_status: current
---

# Tests: install

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-IN-001 | behavior | `go test ./... -run InstallDryRunWritesNothing` | exit_code_0；打印四项；临时 HOME 下无新文件；未调用 launchctl |
| AC-002 | TC-IN-002 | behavior | `go test ./... -run InstallApplyIdempotent` | exit_code_0；两次 apply 后四个 plist 内容一致且无错误 |
| AC-003 | TC-IN-003 | behavior | `go test ./... -run InstallPlistShape` | exit_code_0；四个 plist 的 argv 指向同一二进制与同一 root，PATH 正确 |
| AC-004 | TC-IN-004 | behavior | `go test ./... -run InstallSecretNotPrinted` | exit_code_0；密钥写进 plist；stdout/stderr 不含密钥 |
| AC-005 | TC-IN-005 | behavior | `go test ./... -run InstallUninstallIdempotent` | exit_code_0；卸载后文件消失；再次卸载不报错 |
| AC-006 | TC-IN-006 | behavior | `go test ./... -run InstallStatusReadOnly` | exit_code_0；报告 installed/loaded/last_exit；不改文件 |
| AC-007 | TC-IN-007 | behavior | `go test ./... -run InstallWriteFailureIsLoud` | exit_code_0；不可写目录 → install_failed + 首个失败路径 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-IN-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含此前 87 个用例） |
| AC-001 | TC-IN-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |
