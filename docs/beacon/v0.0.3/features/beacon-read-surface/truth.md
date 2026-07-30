---
slug: beacon-read-surface
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
  prd: docs/beacon/v0.0.3/features/beacon-read-surface/truth.md
  user_story: docs/beacon/v0.0.3/features/beacon-read-surface/truth.md
  test_case: docs/beacon/v0.0.3/features/beacon-read-surface/tests.md
---

# Requirement Truth: beacon-read-surface

## 人话

按项目只读查询 Beacon 版本与最新需求摘要，经 Beacon 公开 CLI；不改写 truth/freeze。

## User Intent

> 配置 Hermes/ADB 可查看 Beacon 管理版本与最新需求。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: beacon-read-surface
program: adb-nl-stable-ops
depends_on: nl-intent-envelope
write_truth: false
```

## Alignment Surface

version 列表/当前 docs version、最新 feature 需求摘要、公开 CLI only。

## Phased Backlog

cron digest 推送；知识库梳理；改写 truth。

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
| VersionDigest | project, docs_version | read-only |
| RequirementDigest | project, feature_summaries | read-only |
| ResultEnvelope | status, blocked, reason_code | 统一外壳 |


## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-BCR-001 | covered |
| INT-002 | user | must | AC-BCR-002 | covered |
| INT-003 | user | must | AC-BCR-003 | covered |
| INT-004 | user | must | AC-BCR-004 | covered |

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-BCR-001 | VersionDigest：按 project 只读摘要 docs version，合法路径 idle→running→done |
| AC-BCR-002 | RequirementDigest：最新需求/feature 摘要只读；失败进入 blocked |
| AC-BCR-003 | Intent action=beacon_status 路由到 VersionDigest/RequirementDigest 只读面，成功态 done |
| AC-BCR-004 | 不允许 write_truth/freeze（illegal/拒绝）；本湖命令保持 read-only，违规 blocked |


## Domain FSM — BeaconRead

| State | From | Guard |
|-------|------|-------|
| idle | — | project resolved |
| running | idle | read request valid |
| done | running | beacon public CLI ok |
| blocked | running | cli fail or write attempt |


## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| idle | version_read | project_ok | running | fetch_version_digest |
| idle | requirement_read | project_ok | running | fetch_requirement_digest |
| running | ok | always | done | emit_digest |
| running | fail | always | blocked | emit_reason |
| running | write_attempt | always | blocked | reject_illegal |


## Illegal transitions

- running → write_truth · TC-BCR-ILL-001
- running → freeze_feature · TC-BCR-ILL-001

## Public CLI Contract

- `adb beacon status --project`（或等价只读子命令）
- 底层仅 Beacon 公开 CLI

## Non-goals

- 改写 requirement truth
- 替代 Beacon freeze/QA

## Freeze readiness

- [x] Alignment / Phased / Deferral 无 pending
- [x] FSM + Illegal 已具体化
- [x] AC↔TC Command+Assertion 齐全
