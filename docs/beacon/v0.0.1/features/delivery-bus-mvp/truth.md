---
slug: delivery-bus-mvp
version: v0.0.1
status: frozen
revision_id: R1
language: zh
domain_required: true
domain_kind: business
ux_required: false
package_maturity: filled
parser_contract: beacon-feature-package-v2
materials_status: current
truth_source_model: intent_first
program_ref: docs/beacon/v0.0.1/programs/agent-delivery-bus-v001/program-manifest.md
git_canonical_branch: main
canonical_refs:
  prd: docs/beacon/v0.0.1/features/delivery-bus-mvp/truth.md
  user_story: docs/beacon/v0.0.1/features/delivery-bus-mvp/truth.md
  test_case: docs/beacon/v0.0.1/features/delivery-bus-mvp/tests.md
provenance:
  - ac_id: AC-ADB-001
    source_type: user_intent
    source_ref: "从单一项目注册表解析 Beacon 源码仓和 managed 项目"
  - ac_id: AC-ADB-002
    source_type: user_intent
    source_ref: "派工前执行仓库、docs 版本和 beacon doctor strict 预检"
  - ac_id: AC-ADB-003
    source_type: user_intent
    source_ref: "implement/freeze/release 使用一次性、带 actor/expiry/scope 的审批令牌"
  - ac_id: AC-ADB-004
    source_type: user_intent
    source_ref: "派工请求和事件持久化到 SQLite"
  - ac_id: AC-ADB-005
    source_type: user_intent
    source_ref: "使用稳定幂等键"
  - ac_id: AC-ADB-006
    source_type: user_intent
    source_ref: "Hermes Kanban 为任务执行后端，创建项目 board 和受约束 task"
  - ac_id: AC-ADB-007
    source_type: derived_contract
    source_ref: "稳定调度需要显式、可审计的 dispatch FSM"
  - ac_id: AC-ADB-008
    source_type: user_intent
    source_ref: "能够查询任务并对账；worker success 不等于 Beacon completion"
  - ac_id: AC-ADB-009
    source_type: user_intent
    source_ref: "提供符合 skill-creator 规范的 agent-delivery-bus skill 和 openai.yaml"
  - ac_id: AC-ADB-010
    source_type: user_intent
    source_ref: "不自动 release、不直接修目标项目、不读取 Hermes 内部 SQLite"
---

# Requirement Truth: Agent Delivery Bus MVP

## 人话

把“给哪个项目、做哪个阶段、由谁执行”变成一个可查、可审批、可重试但不会重复派工的本地控制面。

- 能做：解析 Beacon 源码仓和 managed 项目，先做只读预检，再经必要审批投递到
  Hermes Kanban，并将 worker 结果与 Beacon 交付证据对账。
- 不能做：自动修项目、绕过 Beacon gate、读取 Hermes 内部数据库、把 worker
  自报成功当成交付完成、自动 release。
- 怎样算完：相同请求只产生一个任务；受限阶段无有效审批必阻断；异常均有稳定
  reason code 与恢复路径；测试覆盖成功、失败、重放和非法转换。

## User Intent

> 实现 Agent Delivery Bus MVP。必须：从单一项目注册表解析 Beacon 源码仓和所有 managed 项目；派工前执行仓库、docs 版本和 beacon doctor strict 预检；implement/freeze/release 使用一次性、带 actor/expiry/scope 的审批令牌；派工请求和事件持久化到 SQLite，使用稳定幂等键；Hermes Kanban 为任务执行后端，创建项目 board 和带 worktree/skill/重试限制的任务；能够查询任务并对账；提供符合 Codex skill-creator 规范的 agent-delivery-bus skill 和 openai.yaml；不自动 release，不直接修改目标项目修复其 Beacon context，不读取 Hermes 内部 SQLite。

## User Intent Snapshot

```yaml
program: agent-delivery-bus-v001
lake_or_ocean: 海
scope_mode: full_parity
registry_authority: config/projects.json
scheduler_owner: Hermes Kanban
delivery_gate_owner: Beacon
knowledge_source: Personal Brain
approval_stages: [implement, freeze, release]
auto_release: false
target_repo_mutation_during_preflight: false
scope_ack_ref: user-confirmation-2026-07-27
```

## 用户旅程

1. 操作者用 slug、alias 或 repo path 指定目标项目和 stage/feature。
2. 系统解析唯一项目记录，展示 class、repo 和 docs version。
3. 系统运行只读 strict preflight；失败时写 blocked 事件并给出人工修复 route。
4. implement/freeze/release 请求必须绑定未过期、scope 完全匹配的一次性审批令牌。
5. 系统以稳定幂等键写入 dispatch，并通过 Hermes JSON CLI 创建或复用 board/task。
6. 操作者查询本地 dispatch 与 Hermes task；worker 完成后系统进入 reconciling。
7. 系统核对阶段对应的 Beacon 输出和证据；满足才标 completed，否则保持 blocked 或
   reconciling。

### 失败旅程

- alias 同时命中多个项目：拒绝猜测，返回 `project_alias_ambiguous`。
- 目标项目 Beacon context 不合格：不执行修复，返回 `beacon_context_invalid` 和
  `beacon doctor setup-context/verify-context` 的人工 route。
- 审批过期、scope 不匹配或已经消费：不创建 Hermes task。
- Hermes create 超时且结果未知：审批保持 reserved，dispatch 进入 reconciling，
  先按 idempotency key 查询，禁止盲重试。
- Hermes worker completed 但 Beacon 证据不完整：返回 `beacon_evidence_incomplete`，
  不得标记 completed。

## First principles

- 调度权、执行权、交付判定权必须分离。
- 任何可能写目标项目的阶段都必须先证明“目标、版本、分支和授权”正确。
- 重试是常态，因此每个外部副作用都必须可幂等恢复。
- 未知结果比明确失败更危险；未知结果进入 reconcile，不直接重试。
- 单一真值优先于同步便利：registry、dispatch ledger、Beacon truth 各自只有一个
  canonical owner。

## Domain Model

| Entity | Key fields | Invariant |
|--------|------------|-----------|
| Project | slug, class, repo, docs_version, aliases, dispatchable | slug 唯一；alias 不得跨项目冲突；repo 使用 canonical absolute path |
| PreflightResult | project, stage, status, checks, reason_code, resume_action | 只读；任一 required check fail 则整体 blocked |
| Approval | id, token_hash, actor, project, stage, feature, expiry, state | token 明文不落库；scope 精确匹配；一次性消费 |
| Dispatch | id, idempotency_key, normalized_request, state, approval_id, hermes refs | key 唯一；同 key 不同 payload 必须 conflict |
| DispatchEvent | dispatch_id, sequence, from, event, to, reason_code, payload | 只追加；同 dispatch sequence 单调递增 |
| HermesBinding | project, board_slug, task_id, idempotency_key | 只能由 Hermes 公开 JSON 接口获得 |
| Reconciliation | dispatch_id, hermes_status, beacon_status, evidence_refs | Hermes success 不可单独产生 completed |

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-ADB-001 | covered |
| INT-002 | user | must | AC-ADB-002 | covered |
| INT-003 | user | must | AC-ADB-003 | covered |
| INT-004 | user | must | AC-ADB-004 | covered |
| INT-005 | user | must | AC-ADB-005 | covered |
| INT-006 | user | must | AC-ADB-006 | covered |
| INT-007 | user | must | AC-ADB-007 | covered |
| INT-008 | user | must | AC-ADB-008 | covered |
| INT-009 | derived-safety | must | AC-ADB-009 | covered |
| INT-010 | derived-operations | must | AC-ADB-010 | covered |

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-ADB-001 | `config/projects.json` 是唯一项目注册真值；CLI 能 list，并按 slug、唯一 alias 或 canonical repo path resolve Beacon 源码仓及 managed 项目；缺失、重复 slug、冲突 alias、非法 class 或不存在 repo 必须 fail-closed |
| AC-ADB-002 | `doctor` 和每次 dispatch 前执行只读 strict preflight：repo/git、docs root、声明 docs version、Beacon CLI、`beacon doctor verify-context --strict`、Hermes CLI/gateway/profile；输出逐项 status、稳定 reason_code 和人工 `resume_action`，不得自动修复 |
| AC-ADB-003 | implement/freeze/release scope 使用带 actor、project、stage、feature、expiry 的随机一次性 token；仅存 hash；支持 issue→reserve→finalize，明确失败可 release reservation，过期/错 scope/重放/in-flight 均阻断 |
| AC-ADB-004 | SQLite 原子持久化 projects snapshot、approvals、dispatches、dispatch_events；事件只追加且 sequence 单调，CLI 重启后仍可查询完整状态与最后失败原因 |
| AC-ADB-005 | dispatch 使用规范化请求生成稳定 SHA-256 idempotency key；相同请求返回同一 dispatch/Hermes binding，不重复建 task；同 key 不同 payload 返回 `idempotency_conflict` |
| AC-ADB-006 | Hermes adapter 仅通过公开 JSON CLI 创建项目 board 和 task；task 必须绑定 coding assignee、`agent-delivery-bus` skill、2h runtime、2 retries、稳定 idempotency key；implement 使用 `worktree:<repo>` |
| AC-ADB-007 | dispatch 状态机覆盖 draft、awaiting_approval、queued、dispatched、reconciling、completed、blocked、failed、cancelled；所有转换写事件，非法转换 fail-closed |
| AC-ADB-008 | `task show/list` 与 `reconcile` 合并本地 ledger、Hermes JSON 状态和 Beacon 阶段证据；Hermes/worker success 只进入 reconciling，只有阶段 closure 条件满足才 completed |
| AC-ADB-009 | 提供 `skills/agent-delivery-bus`，SKILL frontmatter 仅含 name/description，包含 `agents/openai.yaml` 和最小契约参考；通过 skill-creator `quick_validate.py`，安装时安全创建 Codex/Hermes symlink 且不覆盖已有目标 |
| AC-ADB-010 | MVP 不读取 Hermes SQLite、不直接修改目标 repo、不自动执行 Beacon 修复、不自动 release；release dispatch 即使有审批也返回 `stage_not_enabled` 和人工 release route |

## Domain FSM — Dispatch

| State | From | Guard |
|-------|------|-------|
| draft | — | normalized request persisted |
| awaiting_approval | draft | restricted stage and strict preflight passed |
| queued | draft, awaiting_approval | open stage or matching approval reserved |
| dispatched | queued, reconciling | Hermes task receipt recovered or created |
| reconciling | queued, dispatched | external result unknown or worker succeeded |
| blocked | draft, awaiting_approval, reconciling | required gate or closure evidence missing |
| failed | queued, dispatched | terminal external failure |
| completed | reconciling | stage closure verified |
| cancelled | draft, awaiting_approval, queued, dispatched | cancellation completed before delivery closure |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| draft | preflight_failed | required_check_failed | blocked | persist checks + reason + resume_action |
| draft | submit_restricted | preflight_pass AND stage in implement/freeze/release | awaiting_approval | persist normalized request |
| draft | submit_open | preflight_pass AND stage in plan/qa | queued | create idempotent dispatch |
| awaiting_approval | approve | token_scope_valid AND not_expired AND available | queued | reserve token atomically |
| awaiting_approval | approval_rejected | invalid_or_expired_or_consumed | blocked | persist approval reason |
| queued | hermes_created | task_id_present OR idempotent_existing_task | dispatched | persist board/task binding; finalize token |
| queued | hermes_failed | result_definitively_failed | failed | release token reservation; persist stderr summary |
| queued | hermes_unknown | timeout_or_unknown_result | reconciling | retain reservation; query by idempotency key |
| dispatched | worker_running | hermes_claimed_or_running | dispatched | refresh remote snapshot |
| dispatched | worker_succeeded | hermes_completed | reconciling | collect Beacon/evidence refs |
| dispatched | worker_failed | hermes_terminal_failure | failed | persist remote failure |
| reconciling | external_task_found | matching_idempotency_key | dispatched | bind recovered Hermes task; finalize token |
| reconciling | closure_verified | stage_closure_pass | completed | persist verification evidence |
| reconciling | closure_incomplete | hermes_done_but_gate_incomplete | blocked | reason=`beacon_evidence_incomplete` |
| blocked | retry | blocker_resolved AND request_still_current | draft | append retry event; rerun preflight |
| failed | retry | retry_authorized AND idempotency_safe | draft | append retry event |
| draft | cancel | no_external_side_effect | cancelled | append cancellation |
| awaiting_approval | cancel | token_not_reserved | cancelled | append cancellation |
| queued | cancel | hermes_not_created | cancelled | release reservation |
| dispatched | cancel_confirmed | hermes_cancel_ack | cancelled | persist remote cancellation |

终态：`completed`、`cancelled`。`blocked` 与 `failed` 必须由显式 retry 或新的外部事实恢复。

## Illegal transitions

- awaiting_approval → queued without an atomically reserved matching token
- completed/cancelled → any non-terminal state
- queued → dispatched without a Hermes task id or idempotent existing-task receipt
- Hermes completed → completed without stage closure verification
- approval consumed twice or reserved by two dispatches
- same idempotency key → two Hermes task bindings
- preflight blocked → Hermes create
- release stage → Hermes create in MVP
- any preflight path → write target project
- Hermes internal SQLite → read/write by Delivery Bus

## Stage Closure Contract

| Stage | Approval | Workspace | completed condition |
|-------|----------|-----------|---------------------|
| plan | no | repo workdir | Hermes success + produced plan/truth artifact refs are readable |
| implement | yes | `worktree:<repo>` | Hermes success + Beacon implementation evidence/admission refs present |
| qa | no | repo/worktree selected by Beacon | Hermes success + Beacon QA verdict pass |
| freeze | yes | truth canonical branch | Hermes success + Beacon freeze artifact reports frozen revision |
| release | yes | n/a | disabled in MVP; always `stage_not_enabled` |

## Public CLI Contract

- `adb projects list`
- `adb projects resolve (--slug SLUG | --alias ALIAS | --path PATH)`
- `adb doctor [--project SLUG]`
- `adb boards sync [--project SLUG]`
- `adb approve --actor ACTOR --project SLUG --stage STAGE --feature FEATURE --ttl SECONDS`
- `adb dispatch --project SLUG --stage STAGE --feature FEATURE [--approval-token TOKEN] [--dry-run]`
- `adb task list [--project SLUG]`
- `adb task show DISPATCH_ID`
- `adb reconcile [DISPATCH_ID]`
- `adb install-skills [--dry-run]`

JSON 输出至少包含：`schema_version`、`status`、`blocked`、`reason_code`、
`resume_action`、`data`。秘密 token 只在 approve 成功响应中出现一次。

## Non-goals

- 自动 release、自动 merge、自动 push 或部署。
- 替代 Hermes worker supervisor、retry engine 或 Kanban 数据库。
- 替代 Beacon truth、freeze、QA 或 release verdict。
- 修改 Personal Brain 的知识模型或实现飞书机器人。
- 首版提供 Web UI、远程多机调度、定时任务或 Orca worker adapter。
- 为 Beacon context 不合格的项目提供静默降级派工。

## Adversarial risks

1. Hermes create 成功但 CLI 超时：必须 reconcile by idempotency key。
2. token 在外部调用前消费，调用失败后无法恢复：使用 reserve/finalize。
3. alias 冲突把任务派到错误仓：registry load 时全局校验。
4. worker 声称完成但没有 Beacon 证据：保持 reconciling/blocked。
5. “doctor”顺手修复目标项目：adapter 只允许 verify/read-only 命令。

## Provenance

每个 AC 均追溯到已确认的用户意图或其必需安全约束；详见 frontmatter provenance
与 Intent Coverage Matrix。

## Freeze readiness

- [x] P4 feature graph ack 已记录（`user-confirmation-2026-07-27`）
- [x] registry / approval / dispatch / reconcile 领域对象已具体化
- [x] 业务 FSM、非法转换、失败旅程已具体化
- [x] 每个 AC 均绑定可执行 TC
- [x] Truth Review Gate pass

## Source Excerpt

```markdown
实现 Agent Delivery Bus MVP。必须：从单一项目注册表解析 Beacon 源码仓和所有 managed 项目；派工前执行仓库、docs 版本和 beacon doctor strict 预检；implement/freeze/release 使用一次性、带 actor/expiry/scope 的审批令牌；派工请求和事件持久化到 SQLite，使用稳定幂等键；Hermes Kanban 为任务执行后端，创建项目 board 和带 worktree/skill/重试限制的任务；能够查询任务并对账；提供符合 Codex skill-creator 规范的 agent-delivery-bus skill 和 openai.yaml；不自动 release，不直接修改目标项目修复其 Beacon context，不读取 Hermes 内部 SQLite。
```
