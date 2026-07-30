---
schema: beacon-program-manifest-v1
program_slug: adb-nl-stable-ops
version: v0.0.3
version_status: proposed
plan_mode: interactive
plan_mode_reason: user_selected_feature_graph_then_first_lake
scope_mode: phased_full
lake_or_ocean: 海
status: p4_acked
revision_id: R1
language: zh
approach: B-intent-cli-thin-skill
first_lake: nl-intent-envelope
feature_graph_ack: true
scope_ack_ref: user-ack-三节-2026-07-30
p2_review_ref: pln-review-20260730t071531z
p2_execution_mode: single_process_multi_reviewer
updated: 2026-07-30
---

# Program Manifest — adb-nl-stable-ops

## Program Intent

在已交付的 ADB 控制面（v0.0.1 MVP + v0.0.2 Memory/Assign）之上，建立**可测的自然语言意图契约**，使 Hermes（飞书/本机）稳定触发 ADB 派工；经 Hermes Kanban 调度本机 agent（如 Codex）执行 Beacon skill（plan 等）；并以 cron 定期反馈看板、Beacon 版本/需求摘要与（外置）知识库梳理。

## Baseline (shipped, not re-delivered)

- `delivery-bus-mvp` @ v0.0.1
- `memory-adapter-auto-assign` @ v0.0.2

## Approach

**B — ADB Intent CLI + 薄 Hermes skill**（拒绝 C 内嵌 NLU；Skill-only 仅作过渡话术，不作为稳定性验收）

## First lake (user)

`nl-intent-envelope`

## Delivery Boundary

- Hermes worker complete ≠ program complete
- implement/freeze 仍须 ADB approve；禁止自动 release
- 不读 Hermes/Beacon 私有 DB；知识正文不进 ADB SQLite
- `goal` 长程：记入 parity，默认 **phase L2+**（见 deferral）；是否升为一等 stage 由 P4 ack 确认

## P4 Ack Status

- ack: accepted
- ack_ref: `user-ack-三节-2026-07-30`
- decision: 三节全部接受；首湖 `nl-intent-envelope`；`goal-stage-binding` 默认 defer；知识库梳理为 support/可延后
- next: `truth` harness for `nl-intent-envelope`（建议先登记 `v0.0.3` governance）
