# Architecture Blueprint

- status: accepted
- project_mode: greenfield
- product: Agent Delivery Bus
- runtime: Python 3 standard library + SQLite
- delivery_version: v0.0.1
- beacon_runtime: v1.6.10

## 1. 系统定位

Agent Delivery Bus 是本机开发项目的控制面，不是新的任务执行器，也不复制
Hermes 或 Beacon 的内部状态机。

```text
Personal Brain / human command
            |
            v
   Agent Delivery Bus
 registry -> preflight -> approval -> idempotent dispatch -> receipt/reconcile
       |          |                         |                    |
       |          v                         v                    v
       |       Beacon CLI             Hermes Kanban         Beacon evidence
       |
       +-> Beacon source repo + Beacon-managed project repos
```

## 2. 权威边界

| Surface | Canonical owner | Delivery Bus responsibility |
|---------|-----------------|-----------------------------|
| 项目路由 | `config/projects.json` | 解析 slug、alias、repo、docs version；拒绝歧义 |
| 灵感与知识正文 | Personal Brain | 只登记来源和路由，不把灵感直接当软件 truth |
| requirement truth / freeze / QA / release gate | Beacon | 调用公开 CLI，保存结果摘要和证据引用 |
| 持久任务、claim、heartbeat、retry、worker 生命周期 | Hermes Kanban | 只调用 JSON CLI/API，不读取 Hermes SQLite |
| 派工意图、审批、幂等映射、审计事件 | Delivery Bus SQLite | 原子写入，支持查询和对账 |
| 目标仓代码与 worktree | 目标项目 + Beacon admission | 不直接修复目标仓 context；预检失败即阻断 |

## 3. 模块图

| Module | Responsibility |
|--------|----------------|
| `registry.py` | 加载并校验单一 JSON registry；slug/alias/path 解析 |
| `preflight.py` | repo、git、Beacon docs/version/context、Hermes/profile 检查 |
| `approvals.py` | issue/reserve/finalize/release 一次性审批令牌 |
| `storage.py` | SQLite schema、事务、dispatch/event/approval 持久化 |
| `service.py` | 状态机、幂等策略、派工与对账编排 |
| `adapters/beacon.py` | Beacon 公开 CLI JSON 契约 |
| `adapters/hermes.py` | Hermes Kanban 公开 CLI JSON 契约 |
| `cli.py` | `adb projects/doctor/boards/approve/dispatch/task/reconcile` |

## 4. 数据模型

### Project

`slug`、`title`、`class`、`repo`、`beacon_docs_root`、
`current_docs_version`、`aliases`、`dispatchable`。

### Dispatch

`dispatch_id`、`idempotency_key`、`project_slug`、`stage`、`feature`、
`state`、`approval_id`、`hermes_board`、`hermes_task_id`、`created_at`、
`updated_at`、`last_reason_code`。

### Approval

`approval_id`、`token_hash`、`actor`、`project_slug`、`stage`、`feature`、
`expires_at`、`state`、`reserved_by`、`reserved_at`、`consumed_at`。

### Event

只追加的 `dispatch_id`、`sequence`、`event_type`、`from_state`、`to_state`、
`reason_code`、`payload_json`、`created_at`。

## 5. 派工事务

1. registry 解析项目和 docs version。
2. 运行只读 strict preflight；任一硬门失败则写 `blocked` 事件并停止。
3. 对 implement/freeze/release 校验审批令牌。MVP 允许签发 release scope，
   但不提供自动 release 派工。
4. 用稳定输入计算 idempotency key，并在 SQLite 事务中创建或复用 dispatch。
5. 对需审批的请求先 reserve token，再调用 Hermes JSON CLI。
6. Hermes 创建成功后保存 board/task id 并 finalize token；若调用明确失败则释放
   reservation，未知结果保留 reservation 并要求 reconcile，避免双派。
7. worker 成功只进入 `reconciling`。只有阶段要求的 Beacon/证据检查通过才进入
   `completed`。

## 6. 稳定幂等键

```text
sha256(
  schema_version
  + project_slug
  + canonical_repo
  + docs_version
  + stage
  + feature
  + normalized_request
)
```

相同 key 只返回原 dispatch；若相同 key 对应的规范化 payload 不同，返回
`idempotency_conflict`，不得创建第二个 Hermes task。

## 7. Workspace 策略

- `plan`、只读 `qa`：使用目标 repo 作为 default workdir，但仍由 Beacon gate
  判定可执行性。
- `implement`：Hermes task workspace 使用 `worktree:<repo>`，worker 必须运行
  Beacon workspace admission，不得直接写 main。
- `freeze`：只允许目标项目 governance 声明的 truth canonical branch；Delivery
  Bus 不替代 Beacon 的 canonical-branch gate。
- `release`：MVP 始终返回 `stage_not_enabled`，仅输出人工 release route。

## 8. 失败关闭与 reason codes

稳定 reason codes 至少包括：

- `project_not_found`
- `project_alias_ambiguous`
- `repo_missing`
- `repo_not_git`
- `beacon_docs_missing`
- `beacon_version_mismatch`
- `beacon_context_invalid`
- `beacon_cli_unavailable`
- `hermes_cli_unavailable`
- `hermes_gateway_unavailable`
- `hermes_profile_missing`
- `approval_required`
- `approval_invalid`
- `approval_expired`
- `approval_scope_mismatch`
- `approval_already_consumed`
- `approval_in_flight`
- `idempotency_conflict`
- `hermes_dispatch_failed`
- `reconciliation_required`
- `beacon_evidence_incomplete`
- `stage_not_enabled`

每个 blocked/failed 结果必须同时给出 `resume_action`，但不得自动执行修复。

## 9. 安全边界

- 不读取或写入 Hermes 内部数据库。
- 不自动运行 `beacon doctor` 的修复命令。
- 不直接修改 registry 中的目标项目。
- 不自动 freeze、implement 或 release；受限阶段必须有明确审批。
- 不把 worker 文本、自报 success 或 Hermes completed 直接映射成 Beacon delivery
  complete。
- 不把 Personal Brain 灵感直接写成目标仓的 frozen truth。

## 10. 首版仓库结构

```text
src/agent_delivery_bus/
  __init__.py
  cli.py
  registry.py
  storage.py
  approvals.py
  preflight.py
  service.py
  adapters/
    beacon.py
    hermes.py
config/projects.json
bin/adb
tests/
skills/agent-delivery-bus/
  SKILL.md
  agents/openai.yaml
  references/contracts.md
```

## 11. 推荐落地顺序

1. registry + read-only doctor
2. SQLite/event ledger + approval token
3. strict preflight + reason-code contract
4. Hermes board/task adapter + idempotent dispatch
5. query/reconcile + Beacon completion semantics
6. Codex/Hermes skill packaging and install
7. 在 Beacon 源码仓 dry-run
8. 在一个健康 managed 项目 canary
9. 修复其余项目的 Beacon context 后逐个开放
10. 最后才考虑 release adapter、定时任务和 UI
