---
slug: knowledge-curation-digest
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
  prd: docs/beacon/v0.0.3/features/knowledge-curation-digest/truth.md
  user_story: docs/beacon/v0.0.3/features/knowledge-curation-digest/truth.md
  test_case: docs/beacon/v0.0.3/features/knowledge-curation-digest/tests.md
---

# Requirement Truth: knowledge-curation-digest

## 人话

在 Personal Brain + Hermes cron/skill 侧定期梳理知识库并产出摘要反馈；不把知识正文写入 ADB SQLite，灵感不得直接变 software truth。

## User Intent

> 定期对知识库梳理后反馈。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: knowledge-curation-digest
program: adb-nl-stable-ops
depends_on: []
soft_depends_on: ops-digest-cron
support_surface: true
adb_core: false
```

## Alignment Surface

Brain curation job、摘要载荷、outside-adb-core 边界。

## Phased Backlog

把灵感直接 freeze 成 feature truth；ADB 核心存知识正文。

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
| CurationSummary | summary_text, created_at | outside ADB sqlite |
| BrainCurationJob | schedule, entrypoint | support surface |
| ResultEnvelope | status, blocked, reason_code | 统一外壳 |


## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-KNW-001 | covered |
| INT-002 | user | must | AC-KNW-002 | covered |
| INT-003 | user | must | AC-KNW-003 | covered |
| INT-004 | user | must | AC-KNW-004 | covered |

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-KNW-001 | CurationSummary：Brain 侧 curation 入口产出摘要，合法路径 idle→running→done |
| AC-KNW-002 | Hermes cron 可定期触发 curation，成功进入 done |
| AC-KNW-003 | 摘要可经消息通道反馈，但拒绝写入 ADB SQLite 正文（illegal → blocked） |
| AC-KNW-004 | 禁止将灵感直接 freeze 为 feature truth（illegal/拒绝）；保持 support 边界 |


## Domain FSM — KnowledgeCuration

| State | From | Guard |
|-------|------|-------|
| idle | — | brain available |
| running | idle | curate request |
| done | running | summary emitted |
| blocked | running | fail or illegal persist/freeze |


## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| idle | curate | brain_ok | running | run_curation |
| running | ok | always | done | emit_curation_summary |
| running | fail | always | blocked | emit_reason |
| running | persist_adb_or_freeze | always | blocked | reject_illegal |


## Illegal transitions

- running → write_adb_sqlite_knowledge · TC-KNW-ILL-001
- running → freeze_as_feature_truth · TC-KNW-ILL-001

## Public CLI Contract

- Personal Brain / qi_dev personal-brain 命令（文档化）
- Hermes cron 模板（ADB 外）

## Non-goals

- 知识正文进入 ADB 核心存储
- 灵感自动升格为 requirement truth

## Freeze readiness

- [x] Alignment / Phased / Deferral 无 pending
- [x] FSM + Illegal 已具体化
- [x] AC↔TC Command+Assertion 齐全
