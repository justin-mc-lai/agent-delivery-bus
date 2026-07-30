---
slug: kanban-ops-nl
version: v0.0.3
status: draft
revision_id: R1
language: zh
domain_required: true
domain_kind: business
ux_required: false
package_maturity: filled
parser_contract: beacon-feature-package-v2
materials_status: current
truth_source_model: intent_first
git_canonical_branch: main
program: adb-nl-stable-ops
promotion_ref: user-ack-三节-2026-07-30
canonical_refs:
  prd: docs/beacon/v0.0.3/features/kanban-ops-nl/truth.md
  user_story: docs/beacon/v0.0.3/features/kanban-ops-nl/truth.md
  test_case: docs/beacon/v0.0.3/features/kanban-ops-nl/tests.md
---

# Requirement Truth: kanban-ops-nl

## 人话

自然语言/CLI 稳定查询多项目 fleet 与看板状态、待拍板表格；只走 Hermes 公开 CLI，不读私有 SQLite。

## User Intent

> Kanban 看板管理与表格反馈，经 ADB+Hermes 公开面。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: kanban-ops-nl
program: adb-nl-stable-ops
depends_on: nl-intent-envelope
hermes_db_access: public_cli_only
```

## Alignment Surface

fleet/boards/awaiting 表格契约；NL action 映射到只读运维命令。

## Phased Backlog

cron 定期推送；Beacon 需求摘要正文；知识库梳理。

## Deferral Ledger

| ID | Item | user_decision | note |
|----|------|---------------|------|
| D1 | goal-stage-binding | accepted_defer | 仅 worker 湖处理；默认 defer 除非本湖显式纳入 |
| D2 | knowledge-curation-in-adb-core | rejected | 知识正文不进 ADB |
| D3 | auto-release | rejected | global |
| D4 | hermes-private-db | rejected | public CLI only |

## 用户旅程

1. 操作者经 NL/Intent 或 CLI 触发本湖动作。
2. 系统按本湖契约执行只读或受控写路径。
3. 返回稳定 JSON（schema_version/status/blocked/reason_code/resume_action/data）。
4. 失败时 fail-closed，给出 resume_action。

### 失败旅程

- 前置湖未满足 / 预检失败：blocked + 稳定 reason_code。
- 试图越权（私库、自动 release、跳过 approve）：illegal fail-closed。

## Domain Model

| Entity | Key fields | Invariant |
|--------|------------|-----------|
| FleetTable | projects, health, board | idle→done 只读 |
| BoardView | columns, counts | project scoped |
| PendingApprovalView | project, stage, feature | awaiting list |
| ResultEnvelope | status, blocked, reason_code | 统一外壳 |


## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-KAN-001 | covered |
| INT-002 | user | must | AC-KAN-002 | covered |
| INT-003 | user | must | AC-KAN-003 | covered |
| INT-004 | user | must | AC-KAN-004 | covered |
| INT-005 | user | must | AC-KAN-005 | covered |

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-KAN-001 | FleetTable 查询：`adb fleet [--json]` 沿 Domain FSM 合法路径 idle→running→done 输出多项目健康/看板摘要表字段稳定 |
| AC-KAN-002 | BoardView 查询：`adb boards status --project` 沿 idle→running→done 展开列/计数；失败进入 blocked 并给 reason_code |
| AC-KAN-003 | Intent action=fleet\|boards\|awaiting 可路由到对应只读命令（经 intent 湖），成功态为 done |
| AC-KAN-004 | PendingApprovalView 待拍板表格复用 approvals awaiting，可与 fleet 一并作为反馈面（合法路径 done） |
| AC-KAN-005 | 禁止读取 Hermes 私有 SQLite（illegal）；仅公开 CLI/API；违规必须 reject/拒绝并 blocked |


## Domain FSM — KanbanOps

| State | From | Guard |
|-------|------|-------|
| idle | — | registry/fleet ready |
| running | idle | query args valid |
| done | running | public CLI success |
| blocked | running | cli fail or illegal db access |


## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| idle | fleet_query | args_ok | running | render_fleet_table |
| idle | boards_query | project_resolved | running | expand_board_view |
| idle | awaiting_query | always | running | render_pending_table |
| running | ok | always | done | emit_table |
| running | cli_fail | always | blocked | emit_reason |
| running | private_db_attempt | always | blocked | reject_illegal |


## Illegal transitions

- running → open_hermes_sqlite · TC-KAN-ILL-001
- query → mutate_without_public_cli · TC-KAN-ILL-001

## Public CLI Contract

- `adb fleet` / `adb boards status` / `adb approvals awaiting`

## Non-goals

- Web UI
- Hermes DB 直读
- 自动根据看板状态派工

## Freeze readiness

- [x] Alignment / Phased / Deferral 无 pending
- [x] FSM + Illegal 已具体化
- [x] AC↔TC Command+Assertion 齐全
