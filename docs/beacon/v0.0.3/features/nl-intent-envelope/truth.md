---
slug: nl-intent-envelope
version: v0.0.3
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
program: adb-nl-stable-ops
promotion_ref: user-ack-三节-2026-07-30
canonical_refs:
  prd: docs/beacon/v0.0.3/features/nl-intent-envelope/truth.md
  user_story: docs/beacon/v0.0.3/features/nl-intent-envelope/truth.md
  test_case: docs/beacon/v0.0.3/features/nl-intent-envelope/tests.md
provenance:
  - ac_id: AC-INT-001
    source_type: user_intent
    source_ref: "adb + 自然语言稳定触发；意图分析"
  - ac_id: AC-INT-002
    source_type: user_intent
    source_ref: "准确进行项目分配调度"
  - ac_id: AC-INT-003
    source_type: program_ack
    source_ref: "user-ack-三节-2026-07-30 / feature-graph first lake"
---

# Requirement Truth: nl-intent-envelope

## 人话

给 Agent Delivery Bus 加一层可测的自然语言意图信封：飞书/本机话术先解析成结构化 IntentEnvelope（项目、阶段、feature、动作），项目歧义就拒绝并说明怎么补；Hermes skill 必须先展示信封给人确认，才能去走现有 approve/dispatch。本湖不负责真正派工、不嵌 NLU 服务、不直连飞书 OpenAPI。

- 能做：`adb intent parse`、歧义 fail-closed、确认门、接到 assign candidates。
- 不能做：解析完自动派工、绕过 approve、ADB 内嵌模型服务。
- 怎样算完：信封 schema 稳定；歧义有 reason_code；确认前不能进 dispatch 路径；pytest 契约绿。

## User Intent

> 自然语言经 Hermes 稳定触发 ADB：意图解析 → 准确项目路由 → 人工确认后再进入现有审批/派工；调度足够稳定、可测。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: nl-intent-envelope
program: adb-nl-stable-ops
approach: B-intent-cli-thin-skill
alignment_surface: IntentEnvelope + confirm gate + fail-closed resolve
phased_backlog:
  - worker-beacon-binding
  - kanban-ops-nl
  - beacon-read-surface
  - ops-digest-cron
  - knowledge-curation-digest
auto_dispatch: false
embedded_nlu: false
feishu_openapi_in_adb: false
```

## Alignment Surface

本 revision 必须煮干：IntentEnvelope schema、`adb intent parse`、registry alias 唯一解析、歧义 reason_code、Hermes skill 确认门、与 assign/approve/dispatch 的结构化衔接（不替代它们）。

## Phased Backlog

不在本包 closure：worker↔Beacon skill 绑定、kanban 运维扩展、Beacon 只读摘要、cron digest、知识库梳理、`goal` 一等 stage。

## Deferral Ledger

| ID | Item | user_decision | note |
|----|------|---------------|------|
| D1 | goal-stage-binding | accepted_defer | P4 ack；放入 worker-beacon-binding 后续 |
| D2 | knowledge-curation-digest | accepted_defer | support lake；ADB 外 |
| D3 | embedded-nlu-service | rejected | Approach C |
| D4 | auto-release | rejected | global boundary |
| D5 | feishu-openapi-in-adb | rejected | 仅 Hermes 通道/载荷 |

## 用户旅程

1. 自然语言解析 → 确认 → 结构化 CLI
2. 项目歧义 fail-closed
3. 信封驱动 assign candidates（不派工）

### 失败旅程

- 歧义项目：`intent_project_ambiguous`，列出 candidates，不选默认项目。
- 动作未知：`intent_action_unknown`，要求澄清。
- 未确认就 dispatch：非法，fail-closed。
- parse 成功但直接创建 executor task：非法。

## Domain Model

| Entity | Key fields | Invariant / requires |
|--------|------------|----------------------|
| ProjectRegistry | slug, aliases, path | loaded before parse |
| IntentEnvelope | schema_version, utterance_hash, action, project_slug, project_candidates, stage, feature, confidence, ambiguity_codes, requires_confirmation, requires_approval | requires ProjectRegistry；action 需项目时 project_slug 唯一或 blocked |
| IntentParser | parse(utterance, registry) | requires ProjectRegistry；只读 registry；不写 dispatch/approval |
| AmbiguityReport | reason_code, resume_action, candidates | requires IntentParser multi-hit；fail-closed |
| ConfirmGate | envelope, actor_ack | requires IntentEnvelope resolved；ack 前不得调用 dispatch |

## Entity Precedence

| Entity | Order | Requires |
|--------|-------|----------|
| ProjectRegistry | 1 | — |
| IntentParser | 2 | ProjectRegistry |
| IntentEnvelope | 3 | IntentParser |
| AmbiguityReport | 3 | IntentParser |
| ConfirmGate | 4 | IntentEnvelope |

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-INT-001 | covered |
| INT-002 | user | must | AC-INT-002 | covered |
| INT-003 | user | must | AC-INT-003 | covered |
| INT-004 | program | must | AC-INT-004 | covered |
| INT-005 | program | must | AC-INT-005 | covered |
| INT-006 | program | must | AC-INT-006 | covered |
| INT-007 | program | must | AC-INT-007 | covered |

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-INT-001 | 提供 `adb intent parse`（及等价 Python API），输出 IntentEnvelope JSON，含 schema_version/status/blocked/reason_code/resume_action/data |
| AC-INT-002 | 使用 registry slug/alias/path 解析项目；唯一命中写入 project_slug；零命中 `intent_project_unresolved` |
| AC-INT-003 | 多候选歧义时 fail-closed：`intent_project_ambiguous`，data.project_candidates 非空，不得静默挑第一个 |
| AC-INT-004 | Hermes skill 合同：展示信封并取得确认前，不得调用 `adb dispatch`；可用文档/fixture 断言确认门 |
| AC-INT-005 | 可将 envelope 的 project/stage/feature 传入既有 `adb assign candidates`，仍只产出候选 |
| AC-INT-006 | parse/confirm 路径不得创建 executor task、不得消费 approve token |
| AC-INT-007 | 提供可执行 pytest 契约覆盖唯一解析、歧义拒绝、非法跳过确认 |

## Domain FSM — IntentParse

| State | From | Guard |
|-------|------|-------|
| idle | — | registry loaded |
| parsing | idle | utterance received |
| resolved | parsing | unique project when required |
| ambiguous | parsing | multiple project candidates |
| blocked | parsing, resolved | missing required fields or unknown action |
| confirmed | resolved | actor_ack |
| ready_for_structured_cli | confirmed | envelope valid |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| idle | parse | registry_ok | parsing | run_parser |
| parsing | unique_hit | project_required_and_unique | resolved | emit_envelope |
| parsing | multi_hit | project_required | ambiguous | emit_ambiguous |
| parsing | no_hit | project_required | blocked | emit_unresolved |
| ambiguous | clarify | user_picks_one | resolved | set_project_slug |
| resolved | confirm | actor_ack | confirmed | mark_confirmed |
| confirmed | handoff | envelope_valid | ready_for_structured_cli | allow_assign_or_approve_or_dispatch_cli |
| resolved | dispatch_without_confirm | always | blocked | emit_confirm_required |

## Illegal transitions

- IntentParse.resolved → ready_for_structured_cli without confirm · TC-INT-ILL-001
- IntentParse.parsing → create_executor_task · TC-INT-ILL-001
- IntentParse.ambiguous → resolved by silent first-candidate pick · TC-INT-ILL-002
- IntentParser.parse → consume_approval_token · TC-INT-ILL-001

## Public CLI Contract

- `adb intent parse --utterance TEXT [--json]`
- `adb intent parse --utterance TEXT --project SLUG [--json]`（可选强制项目）
- 既有：`adb assign candidates` / `adb approve` / `adb dispatch`（本湖不改其门控语义）

## Non-goals

- ADB 内嵌 NLU/LLM 服务
- 飞书 OpenAPI 直推或关键字硬路由内核
- 自动跳过确认的 dispatch
- worker↔Beacon skill 绑定、cron digest、Beacon 需求摘要、知识库梳理（见 Phased Backlog）
- 自动 release

## Freeze readiness

- [x] Alignment Surface / Phased Backlog / Deferral Ledger 决策已闭合
- [x] Domain Model / Entity Precedence / Domain FSM / Illegal 已具体化
- [x] 每个 AC 绑定 Command+Assertion TC
- [x] Truth fill R2 projected + Review Gate A/B/C/D
