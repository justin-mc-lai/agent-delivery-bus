---
slug: dispatch-liveness
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
  truth: docs/beacon/v0.1.5/features/dispatch-liveness/truth.md
  tests: docs/beacon/v0.1.5/features/dispatch-liveness/tests.md
  tasks: docs/beacon/v0.1.5/features/dispatch-liveness/tasks.md
  evidence: docs/beacon/v0.1.5/features/dispatch-liveness/evidence.md
---

# Requirement Truth: dispatch-liveness

## 人话

派出去的 worker 有时候一声不响就 `idle` 了：消息收到了，模型一次都没被调用（0 token，被 abort），
但 adbd 的 send 返回成功、退出码 0，看起来"派活成功"。这次抓到两个这样的死 worker。

两件事要修：新建 worker 不再整仓拷贝（大仓库会把首轮 abort），以及**派活后必须自检**——
没真开跑就重试，仍然没跑就记 `dispatch_dead` 放回队列，绝不把"没确认"当成"成功"。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "修复 worker 派活后静默死亡"
goal: "worktree 工作区 + 派活存活自检 + 有界重试；判死回队列"
non_goals: "不改 paseo/opencode；不加 UI；不引依赖"
constraints: "Go stdlib only；paseo 不透明；检查有超时上界"
scope_mode: "lake"
```

## 根因（已由真机 A/B 证据确认）

| 建 worker 方式 | 结果 |
|----------------|------|
| `--new-workspace local` + 53MB 仓库 | 两轮全 `MessageAbortedError`，0 token |
| `--new-workspace worktree` + 53MB 仓库 | 正常，有输出 |
| 不加 `--new-workspace` | 正常，但忽略 `--cwd`，落到 CLI 当前目录 |

`local` 是整仓拷贝，工作区没就绪时首轮在 ~90ms 内被中止；`--no-wait` 紧接着的 `send` 再被中止，
agent 直接 idle，paseo 不报错。

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 新建 worker 默认使用 `--new-workspace worktree`；`adbd_worker_workspace` 可取 `worktree`/`local`，默认 `worktree`；两种模式都必须仍传 `--cwd` 与项目映射结果 |
| AC-002 | `paseo send` 之后做存活检查：send 前记录该 agent 的 paseo 时间线进度，send 后在 `adbd_worker_dispatch_timeout_sec`（默认 10 秒）内轮询；进度增长即判活，超时未增长即判死 |
| AC-003 | 判死时重试 `paseo send`，最多 `adbd_worker_dispatch_retries`（默认 1）次；每次重试后重新做存活检查 |
| AC-004 | 重试耗尽仍判死时，返回可识别错误；`route` 写 `decision=dispatch_dead`、`reason_code=dispatch_dead` 的 trace，条目回到队列，不写绑定，进程退出码 0 |
| AC-005 | 存活检查有硬超时上界；探测命令失败、输出不可解析时判 `dispatch_unverified` 并写同名 reason_code，绝不当作成功 |
| AC-006 | `adbd_worker_dispatch_check=off` 时跳过检查，trace 记 `reason_code=dispatch_unchecked`，其余行为不变 |
| AC-007 | 既有 126 个 Go 测试保持绿；检查只增加只读的 paseo 调用（日志/时间线），不改变 `paseo send` 的语义 |

## 判定表

| 情况 | 结果 |
|------|------|
| send 后进度增长 | 派活成功，trace 正常 |
| 超时无进度 + 还有重试 | 再 send 一次，重新计时 |
| 超时无进度 + 重试耗尽 | `dispatch_dead`，回队列 |
| 探测命令失败 | `dispatch_unverified`，回队列（不装作成功） |
| `adbd_worker_dispatch_check=off` | 不检查，记 `dispatch_unchecked` |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_worker_workspace` | `worktree` | 新建 worker 的工作区模式（`worktree`/`local`） |
| `adbd_worker_dispatch_check` | `on` | 存活检查总开关 |
| `adbd_worker_dispatch_timeout_sec` | `10` | 单次检查等待上限 |
| `adbd_worker_dispatch_retries` | `1` | 判死后额外重试次数 |

## Non-goals（冻结边界）

- 不解析 provider 内部状态，只读 paseo 自身的时间线
- 不改 `paseo send` 语义，不引入 `--behavior` 变更
- 不修复 paseo 的 abort 行为本身（那是上游）

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
