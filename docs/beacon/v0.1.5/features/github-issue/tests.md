---
slug: github-issue
version: v0.1.5
materials_status: current
---

# Tests: github-issue

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-GH-001 | behavior | `go test ./... -run PollMapsConfiguredRepos` | exit_code_0；按配置的仓库调用 gh（记录型 gh 脚本收到预期 argv）；无配置时 polled=0 且 exit 0 |
| AC-002 | TC-GH-002 | behavior | `go test ./... -run ThreadAndDedup` | exit_code_0；thread 形如 `gh:o/r#12`；message-id `gh:12:<updatedAt>`；updatedAt 未变时 new=0 deduped=1 |
| AC-003 | TC-GH-003 | behavior | `go test ./... -run OpenOnlyAndLabelFilter` | exit_code_0；label 白名单生效；源码扫描断言无写 GitHub 的调用 |
| AC-004 | TC-GH-004 | behavior | `go test ./... -run ProxyInjectedIntoGhEnv` | exit_code_0；配置代理时子进程 env 含 HTTPS_PROXY/HTTP_PROXY；未配置时继承 |
| AC-005 | TC-GH-005 | behavior | `go test ./... -run GhErrorCodes` | exit_code_0；gh_not_found / gh_failed / gh_output_invalid 三种码与退出码 2 正确 |
| AC-006 | TC-GH-006 | behavior | `go test ./... -run PollEndToEnd` | exit_code_0；输出 polled/new/deduped；runner 收到 UNTRUSTED_INPUT 文本 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-GH-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含此前 42 个用例） |
| AC-001 | TC-GH-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0（Python 侧零回归） |
