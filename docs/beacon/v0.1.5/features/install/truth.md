---
slug: install
version: v0.1.5
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v0.1.5/features/install/truth.md
  tests: docs/beacon/v0.1.5/features/install/tests.md
  tasks: docs/beacon/v0.1.5/features/install/tasks.md
  evidence: docs/beacon/v0.1.5/features/install/evidence.md
---

# Requirement Truth: install

## 人话

现在装这四个常驻任务要手工复制 plist，换台机器就废。
这个湖把它变成一条命令：装、卸、看状态，重复跑不会坏，密钥不会漏到屏幕上。
不写文件就说清楚要写什么——先 dry-run，再 apply。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "现在继续补充安装，管理缺口，确保整个superagent闭环"
goal: "一条命令安装/卸载/检查四个常驻任务；幂等、可预演、密钥不外泄、失败可诊断。"
non_goals: "不装 Python/paseo/beacon-go；不改 shell profile；不写仓库内文件。"
constraints: "Go 放 cmd/adbd（stdlib only）；只动 ~/Library/LaunchAgents；密钥不进 stdout。"
scope_mode: "lake"
```

## 任务清单（固定四个）

| label 后缀 | 子命令 | 间隔 |
|------------|--------|------|
| `adbd` | `watchdog --once` | 600s |
| `adbd-route` | `route --once` | 120s |
| `adbd-github` | `poll-github --once` | 120s |
| `adbd-reconcile` | `reconcile --once` | 120s |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `adbd install --dry-run` 打印四个任务的 label、plist 路径、子命令、间隔，以及当前是否已安装；**不写任何文件、不调用 launchctl** |
| AC-002 | `adbd install --apply` 写四个 plist 到 `~/Library/LaunchAgents` 并 `launchctl bootstrap`；重复执行幂等（已加载的先 bootout 再 bootstrap，不报错） |
| AC-003 | 四个 plist 的 `ProgramArguments` 一致指向当前可执行文件与同一个 `--root`；`PATH` 含 `/opt/homebrew/bin` 与 `~/.local/bin` |
| AC-004 | `ADBD_LLM_API_KEY` 从当前环境读取并写入 plist 的 `EnvironmentVariables`；该值不得出现在 stdout/stderr 任何一行（测试断言输出不含密钥） |
| AC-005 | `adbd install --uninstall` bootout 并删除四个 plist，幂等（不存在时不报错） |
| AC-006 | `adbd install --status` 只读报告四个任务的 `installed` / `loaded` / `last_exit`；launchctl 不可用时 `reason_code=launchctl_unavailable` |
| AC-007 | 任一 plist 写入失败时整体失败并给出 `reason_code=install_failed` 与首个失败路径；不留下"部分安装"的假成功 |

## 禁止的迁移（人工维护，内核不校验）

- illegal: apply -> dry_run（同一次调用内）

## Non-goals（冻结边界）

- 不安装 Python 包、paseo、beacon-go
- 不改 PATH 或 shell profile
- 不写仓库内任何文件

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | user utterance | truth.md#User-Intent-Snapshot |
| AC-002 | user utterance | truth.md#User-Intent-Snapshot |
| AC-003 | user utterance | truth.md#User-Intent-Snapshot |
| AC-004 | user utterance | truth.md#User-Intent-Snapshot |
| AC-005 | user utterance | truth.md#User-Intent-Snapshot |
| AC-006 | user utterance | truth.md#User-Intent-Snapshot |
| AC-007 | user utterance | truth.md#User-Intent-Snapshot |

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 安装动作用 adbd install，还是单独脚本？ | adbd install（Go 内实现，幂等，可 dry-run） | answered | adbd install --dry-run/--apply/--uninstall/--status。 | human |
