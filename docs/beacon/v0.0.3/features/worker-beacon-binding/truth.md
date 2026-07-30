---
slug: worker-beacon-binding
version: v0.0.3
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
git_canonical_branch: main
program: adb-nl-stable-ops
promotion_ref: user-ack-三节-2026-07-30
canonical_refs:
  prd: docs/beacon/v0.0.3/features/worker-beacon-binding/truth.md
  user_story: docs/beacon/v0.0.3/features/worker-beacon-binding/truth.md
  test_case: docs/beacon/v0.0.3/features/worker-beacon-binding/tests.md
---

# Requirement Truth: worker-beacon-binding

## 人话

把 Hermes Kanban 任务体绑到本机 agent runner（如 Codex/coding profile），按 stage 调用 Beacon skill（至少 plan）；implement/freeze 仍走既有 approve。goal 一等 stage 默认延后。

## User Intent

> Hermes 调度本机 agent 跑 Beacon skill（plan 等）；goal 长程默认延后。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: worker-beacon-binding
program: adb-nl-stable-ops
depends_on: nl-intent-envelope
goal_stage: deferred
runner: coding_profile_or_codex
auto_release: false
```

## Alignment Surface

stage→skill/runner 映射、task body 合同、workspace admission 失败 fail-closed、plan 绑定。

## Phased Backlog

goal 一等 stage；cron digest；知识库梳理；kanban 运维扩展。

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
| EnvelopeOrRequest | project_slug, stage, feature, action | 需要项目时必须已解析 |
| ResultEnvelope | status, blocked, reason_code, resume_action, data | 所有公共命令统一外壳 |
| Gate | preflight/approval refs | 不得削弱既有 approve |

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-WRK-001 | covered |
| INT-002 | user | must | AC-WRK-002 | covered |
| INT-003 | user | must | AC-WRK-003 | covered |
| INT-004 | user | must | AC-WRK-004 | covered |
| INT-005 | user | must | AC-WRK-005 | covered |

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-WRK-001 | dispatch 创建的 Hermes task body 含 stage→Beacon skill/命令绑定字段（至少 plan） |
| AC-WRK-002 | runner 约定使用 Hermes coding profile 或显式 Codex/等价本机 agent，不得假定云端集群调度器 |
| AC-WRK-003 | workspace admission/预检失败时不得创建成功派工；返回稳定 reason_code |
| AC-WRK-004 | implement/freeze 仍要求既有 approve token；绑定层不得绕过 |
| AC-WRK-005 | goal-stage-binding 默认 defer：未升格前 ENABLED_STAGES 不含 goal，或显式 blocked reason |

## Domain FSM — WRKFlow

| State | From | Guard |
|-------|------|-------|
| idle | — | ready |
| running | idle | request_valid |
| done | running | success |
| blocked | running | gate_fail |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| idle | bind_request | envelope_or_dispatch_ok | binding | build_task_body |
| binding | admission_pass | runner_resolved | queued | create_hermes_task |
| binding | admission_fail | always | blocked | emit_admission_failed |
| queued | worker_terminal | always | reconciling | hand_to_reconcile |

## Illegal transitions

- binding → completed skipping Hermes/reconcile · TC-WRK-ILL-001
- binding → implement_dispatch without approve · TC-WRK-ILL-001
- goal_stage_enabled without P4 promote · TC-WRK-ILL-002

## Public CLI Contract

- 扩展既有 `adb dispatch` task body 合同（文档+测试）
- 不新增自动 release CLI

## Non-goals

- Orca 主调度器替换 Hermes
- 自动 release
- 默认启用 goal 一等 stage（除非后续 change 升格）

## Freeze readiness

- [x] Alignment / Phased / Deferral 无未决项
- [x] FSM + Illegal 已具体化
- [x] AC↔TC Command+Assertion 齐全
