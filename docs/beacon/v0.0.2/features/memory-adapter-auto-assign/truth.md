---
slug: memory-adapter-auto-assign
version: v0.0.2
status: frozen
revision_id: R2
language: zh
domain_required: true
domain_kind: business
ux_required: false
package_maturity: filled
parser_contract: beacon-feature-package-v2
materials_status: current
truth_source_model: intent_first
git_canonical_branch: main
canonical_refs:
  prd: docs/beacon/v0.0.2/features/memory-adapter-auto-assign/truth.md
  user_story: docs/beacon/v0.0.2/features/memory-adapter-auto-assign/truth.md
  test_case: docs/beacon/v0.0.2/features/memory-adapter-auto-assign/tests.md
provenance:
  - ac_id: AC-MEM-001
    source_type: user_intent
    source_ref: "ADB 外薄 MemoryAdapter；dispatch 前 scoped recall"
  - ac_id: AC-MEM-002
    source_type: user_intent
    source_ref: "reconcile 后写回证据"
  - ac_id: AC-MEM-003
    source_type: user_intent
    source_ref: "验证跨项目 ACL 召回"
  - ac_id: AC-MEM-004
    source_type: user_intent
    source_ref: "规则/评分器产出自动分配候选"
  - ac_id: AC-MEM-005
    source_type: user_intent
    source_ref: "人工审批仍走现有 approve"
  - ac_id: AC-MEM-006
    source_type: user_intent
    source_ref: "Hermes 飞书通道列出待人工拍板事项"
  - ac_id: AC-MEM-007
    source_type: user_intent
    source_ref: "拍板通过后允许项目内 agent 调度"
---

# Requirement Truth: memory-adapter-auto-assign

## 人话

在 Agent Delivery Bus 外加一层薄 MemoryAdapter：派工前按项目作用域召回记忆，对账后把证据写回；自动分配只产出候选，真正派工仍走现有 approve；Hermes 飞书通道列出待拍板事项，人拍板后才能在项目内调度 agent。

- 能做：scoped recall / writeback、跨项目 ACL 拒召回、规则评分产出候选、飞书列出待审、批准后 dispatch。
- 不能做：把 agentmemory 嵌进 ADB 核心、绕过 approve 自动派受限阶段、读取 Hermes 内部库、自动 release。
- 怎样算完：跨项目召回失败关闭；候选不等于派工；拍板令牌可用后可 dispatch；相关 pytest 绿。

## User Intent

> ADB 外薄 MemoryAdapter：dispatch 前 scoped recall、reconcile 后写回；验证跨项目 ACL；规则/评分器产出自动分配候选仍走现有 approve；Hermes 飞书通道列出待人工拍板事项，拍板后允许项目内 agent 调度运行。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: memory-adapter-auto-assign
program: agent-delivery-bus-v002
memory_backend: agentmemory REST
adapter_placement: outside ADB core thin SPI
recall_hook: before dispatch
writeback_hook: after reconcile
acl: project_slug scoped fail-closed
auto_assign: rules+scorer -> candidates only
approval_authority: existing adb approve
feishu_surface: Hermes channel list awaiting_approval items
post_approve: allow in-project agent dispatch
auto_release: false
```

## 用户旅程

1. 操作者或评分器提出 project/stage/feature 候选。
2. 系统列出待拍板事项（CLI/JSON，并可经 Hermes 飞书通道渲染）。
3. 人对受限阶段签发一次性 approve 令牌。
4. dispatch 前 MemoryAdapter 按 project_slug 做 scoped recall，注入 task 上下文。
5. 既有 preflight + token reserve 后创建 executor task。
6. worker 终态后 reconcile；MemoryAdapter 写回证据记忆（失败不抹主结果）。
7. 跨项目召回断言持续成立：B 不得读到 A。

### 失败旅程

- 无有效 approve：`approval_required`，不得创建 executor task。
- 跨项目记忆命中：`memory_acl_denied`，fail-closed。
- agentmemory 不可用：稳定 `memory_unavailable` + resume_action；不得静默跨项目降级。
- 写回失败：保留 reconcile status，记录 writeback error 可重试。
- 评分器直接派工：非法；候选不得消费 token。

## Domain Model

| Entity | Key fields | Invariant / requires |
|--------|------------|----------------------|
| MemoryScope | project_slug, optional agent_id | 召回与写回必须绑定同一 project_slug；跨 slug 不可见 |
| MemoryAdapter | recall, writeback, health | 位于 adapters 层；核心 registry/storage/approvals 不得硬编码 agentmemory |
| MemoryRecord | scope, kind, payload, dispatch_id | writeback 至少含 project/stage/feature/dispatch_id/reason_code |
| DispatchCandidate | project, stage, feature, score, reasons | 仅候选；不得持有或消费 approve token；不得含 executor task_id |
| AssignmentScorer | rules, weights | 只产出 candidates 列表 |
| PendingApprovalView | project, stage, feature, expires_at, actor_hint | 展示待拍板；不等于已授权 |
| Approval | token_hash, actor, project, stage, feature, expiry, state | 沿用现有 ApprovalService；一次性 |
| Dispatch | id, state, approval_id, memory_injection_ref | 受限阶段必须 reserved token；recall 摘要可注入 body |

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-MEM-001 | covered |
| INT-002 | user | must | AC-MEM-002 | covered |
| INT-003 | user | must | AC-MEM-003 | covered |
| INT-004 | user | must | AC-MEM-004 | covered |
| INT-005 | user | must | AC-MEM-005 | covered |
| INT-006 | user | must | AC-MEM-006 | covered |
| INT-007 | user | must | AC-MEM-007 | covered |

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-MEM-001 | 提供位于 ADB 核心外的 MemoryAdapter SPI；dispatch 前按 project_slug 做 scoped recall，并将摘要注入 task body/上下文，不把 agentmemory 客户端嵌进 registry/storage/approvals 核心模块 |
| AC-MEM-002 | reconcile 进入 completed/blocked 终态后调用 MemoryAdapter.writeback，写入含 project_slug/stage/feature/dispatch_id/reason_code 的证据记忆；写回失败不得抹掉 reconcile 主结果 |
| AC-MEM-003 | 跨项目 ACL：以项目 B scope 召回不得返回项目 A 写入的记忆；越权命中必须 fail-closed 并有可执行测试断言 |
| AC-MEM-004 | 自动分配：规则+评分器仅产出 dispatch candidates（含 score/reason），不得直接创建 executor task 或消费 approve 令牌 |
| AC-MEM-005 | 受限阶段派工仍必须经现有 ApprovalService.issue/reserve/finalize；候选转派工路径只能消费有效 approve token |
| AC-MEM-006 | 提供 awaiting_approval / 待拍板列表（CLI/JSON），并可经 Hermes 飞书通道渲染待拍板事项（项目/阶段/feature/过期时间/actor 需求） |
| AC-MEM-007 | 拍板签发有效令牌后，允许对该 project/stage/feature 执行 in-project agent dispatch；无令牌时保持 approval_required 阻断 |

## Domain FSM — MemoryDispatch

| State | From | Guard |
|-------|------|-------|
| idle | — | registry loaded |
| candidates_ready | idle | scorer produced zero-or-more candidates |
| awaiting_human_approve | candidates_ready, idle | restricted stage or candidate selected |
| dispatch_authorized | awaiting_human_approve | matching approve token issued |
| recalling | dispatch_authorized | preflight pass and token reserved |
| dispatched | recalling | acl_scope_ok and executor task created |
| writing_back | dispatched | executor terminal and reconcile evaluated |
| completed | writing_back | final state persisted |
| blocked | awaiting_human_approve, recalling, writing_back | approval_required or memory_acl_denied or gate fail |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| idle | score_request | registry_ok | candidates_ready | run_scorer |
| candidates_ready | list_awaiting | has_restricted_or_candidate | awaiting_human_approve | emit_feishu_payload |
| awaiting_human_approve | approve_issued | token_valid_scope | dispatch_authorized | store_approval |
| dispatch_authorized | dispatch | preflight_pass_and_token_reserved | recalling | memory_recall |
| recalling | recall_done | acl_scope_ok | dispatched | create_executor_task |
| dispatched | reconcile | executor_terminal | writing_back | truth_gate_closure |
| writing_back | writeback_done | always | completed | persist_final_state |
| recalling | acl_violation | cross_project_hit | blocked | emit_memory_acl_denied |
| awaiting_human_approve | dispatch_without_token | stage_restricted | blocked | emit_approval_required |

## Illegal transitions

- MemoryDispatch.candidates_ready → dispatched without approve (跳过 approve) · TC-MEM-ILL-001
- MemoryDispatch.idle → dispatched without recall/preflight · TC-MEM-ILL-001
- MemoryDispatch.awaiting_human_approve → completed · TC-MEM-ILL-001
- MemoryDispatch.recalling → completed skipping executor/reconcile · TC-MEM-ILL-001
- MemoryScope.cross_project → recall_success · TC-MEM-ILL-002
- AssignmentScorer.score → consume_approval_token · TC-MEM-ILL-001
- MemoryAdapter.writeback_failure → erase_reconcile_result · TC-MEM-ILL-002

## Public CLI Contract

- `adb assign candidates [--project SLUG] [--json]`
- `adb approvals awaiting [--project SLUG] [--channel feishu|text] [--json]`
- `adb approve ...`（既有）
- `adb dispatch ...`（既有；前插 recall，后接 writeback）

JSON 至少含 `schema_version`、`status`、`blocked`、`reason_code`、`resume_action`、`data`。

## Non-goals

- 将 agentmemory SDK/客户端嵌入 ADB registry/storage/approvals 核心
- 自动分配直接创建 Hermes task 或自动消费 approve 令牌
- 读取或写入 Hermes 内部 SQLite
- 自动 release / 自动 merge / 自动修复目标仓 Beacon context
- 在本湖内实现完整飞书机器人协议栈（仅复用 Hermes 通道列出/通知）
- 多租户云端记忆托管或跨机器同步

## Freeze readiness

- [x] 湖范围与 non-goals 已确认
- [x] Domain Model / Domain FSM / Illegal transitions 已具体化
- [x] 每个 AC 绑定可执行 TC（含 illegal 覆盖）
- [x] Truth Review Gate A/C/D pass
