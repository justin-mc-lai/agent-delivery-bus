---
slug: ops-digest-cron
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
  prd: docs/beacon/v0.0.3/features/ops-digest-cron/truth.md
  user_story: docs/beacon/v0.0.3/features/ops-digest-cron/truth.md
  test_case: docs/beacon/v0.0.3/features/ops-digest-cron/tests.md
---

# Requirement Truth: ops-digest-cron

## 人话

用 Hermes cron 定期跑 digest：汇总 fleet + 待拍板 +（可选）Beacon 摘要，产出飞书可发送载荷；ADB 不内嵌 cron daemon，也不根据 digest 自动派工。

## User Intent

> 定期反馈总结项目进度看板与待拍板。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: ops-digest-cron
program: adb-nl-stable-ops
depends_on: [kanban-ops-nl, beacon-read-surface]
cron_owner: hermes
auto_dispatch_from_digest: false
```

## Alignment Surface

cron 模板、digest 渲染、飞书载荷 handoff、幂等。

## Phased Backlog

知识库梳理正文；ADB 进程内 cron。

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
| DigestPayload | fleet, awaiting, optional beacon | no dispatch side effects |
| CronTemplate | hermes cron job | outside ADB daemon |
| ResultEnvelope | status, blocked, reason_code | 统一外壳 |


## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-DIG-001 | covered |
| INT-002 | user | must | AC-DIG-002 | covered |
| INT-003 | user | must | AC-DIG-003 | covered |
| INT-004 | user | must | AC-DIG-004 | covered |
| INT-005 | user | must | AC-DIG-005 | covered |

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-DIG-001 | DigestPayload：`adb digest render` 沿 idle→running→done 汇总 fleet+awaiting（可选 beacon） |
| AC-DIG-002 | CronTemplate：文档化 Hermes cron 触发 digest；不在 ADB 内嵌调度器；路径仍到 done |
| AC-DIG-003 | 飞书通道仅产出 DigestPayload 载荷；不强制 OpenAPI；成功态 done |
| AC-DIG-004 | digest 幂等：相同输入不 create 派工副作用，保持 done 且无 dispatch |
| AC-DIG-005 | digest 不得自动 approve/dispatch（illegal/拒绝）；违规进入 blocked |


## Domain FSM — OpsDigest

| State | From | Guard |
|-------|------|-------|
| idle | — | cron or CLI trigger |
| running | idle | collect surfaces |
| done | running | payload rendered |
| blocked | running | collect fail or illegal dispatch |


## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| idle | tick | cron_or_cli | running | collect_fleet_awaiting |
| running | ok | always | done | emit_digest_payload |
| running | fail | always | blocked | emit_reason |
| running | auto_dispatch_attempt | always | blocked | reject_illegal |


## Illegal transitions

- rendering → dispatch · TC-DIG-ILL-001
- rendering → approve · TC-DIG-ILL-001

## Public CLI Contract

- `adb digest render [--channel feishu|text] [--json]`
- Hermes `cron create` 模板（文档+fixture）

## Non-goals

- ADB 内嵌 cron daemon
- 根据 digest 自动派工
- 飞书 OpenAPI 客户端进核心

## Freeze readiness

- [x] Alignment / Phased / Deferral 无 pending
- [x] FSM + Illegal 已具体化
- [x] AC↔TC Command+Assertion 齐全
