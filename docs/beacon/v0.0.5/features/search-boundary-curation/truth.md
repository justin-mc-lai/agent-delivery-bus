---
slug: search-boundary-curation
version: v0.0.5
status: frozen
language: zh
package_maturity: filled
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
materials_status: current
canonical_refs:
  prd: docs/beacon/v0.0.5/features/search-boundary-curation/truth.md
  user_story: docs/beacon/v0.0.5/features/search-boundary-curation/truth.md
  test_case: docs/beacon/v0.0.5/features/search-boundary-curation/tests.md
---

# Requirement Truth: search-boundary-curation

## 人话

把「定时整理网络搜索/选题边界 → 待审 → 你拍板」升级成带双层垂直画像的能力。库拾账号只做 GitHub 开源 AI 库 / AI Spec 向的微信公众号贴图（图文信息图，不是表情包）。系统按项目生成可审计 ProjectVerticalProfile，自媒体再生成 AccountVerticalProfile；选题可用 intel-radar/agent-reach 编排发挥，但须有意义有价值；完整偏离项目/账号垂直则 VerticalGate fail-closed，不得进待审。每日 cron 产出 5 条待审选题推飞书；仍须人工 approve/reject。ADB 不内嵌爬虫，不 auto-approve。

- 能做：画像 ref 绑定、VerticalGate、库拾垂直日批 5 条、ingest→待审→decide、Hermes 日更。
- 不能做：表情包离题题库、无画像硬编码池、自动 approve、ADB 内嵌搜网引擎。
- 怎样算完：离题题被拒；日批 5 条均在 oss-picks/AI Spec；pytest 绿；人工拍板后才 active。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖（R2 change 覆盖 L0–L4）
language: zh
scope_mode: lake
feature: search-boundary-curation
revision: R2
program: personal-production-flywheel
depends_on: [vision-flywheel]
account_example: kushi-gzh / wechat-gzh
default_vertical: oss-picks
draft_tab_meaning: 微信草稿箱「贴图」= image_post 信息图，非表情包
cron: daily 5 topics → awaiting_review → human decide
status_token: awaiting_review
cli_list_awaiting: adb boundary pending
auto_approve_on_ingest: false
adb_embedded_web_search: false
vertical_gate: fail_closed_on_complete_drift
intel_radar: orchestration_preferred_with_in_vertical_fallback
```

## 用户旅程

1. L0 止血：移除离题题库 / 错语义待审项
1. L1 提案绑定项目/账号垂直画像 ref
1. L2 生成/加载可审计 ProjectVerticalProfile
1. L3 自媒体 AccountVerticalProfile（库拾）
1. L4 VerticalGate + intel-radar 编排 → 待审拍板
1. 验收与发布门

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-SBC-001 | covered |
| INT-002 | user | must | AC-SBC-002 | covered |
| INT-003 | user | must | AC-SBC-003 | covered |
| INT-004 | user | must | AC-SBC-004 | covered |
| INT-005 | user | must | AC-SBC-005 | covered |
| INT-006 | user | must | AC-SBC-006 | covered |
| INT-007 | user | must | AC-SBC-007 | covered |
| INT-008 | user | must | AC-SBC-008 | covered |
| INT-009 | user | must | AC-SBC-009 | covered |
| INT-010 | user | must | AC-SBC-010 | covered |
| INT-011 | user | must | AC-SBC-011 | covered |

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-SBC-001 | adb boundary ingest 写入 awaiting_review（topic 必填；query_hints/sources 可多值；须带 project_profile_ref；自媒体路径另带 account_profile_ref）；缺 topic/画像 ref 拒绝 |
| AC-SBC-002 | CLI 列出全部 awaiting_review；show 返回单条（含画像 ref） |
| AC-SBC-003 | adb boundary decide 指定 id --actor A --decision approve\|reject；approve→active，reject→rejected |
| AC-SBC-004 | adb boundary list 默认仅 active；可用 --status 过滤 |
| AC-SBC-005 | 待审提案进入 approvals awaiting（kind 绑定 boundary 待审） |
| AC-SBC-006 | 定时路径：schedule + Hermes cron 日更；脚本只 ingest 不 auto-approve；默认产出 5 条垂直约束提案 |
| AC-SBC-007 | illegal：ingest 直达 active、跳过待审启用、自动 approve → fail-closed |
| AC-SBC-008 | L1：无 project_profile_ref（及自媒体缺 account_profile_ref）不得 ingest；禁止无画像静态离题池 |
| AC-SBC-009 | L2/L3：可加载审计 ProjectVerticalProfile 与 AccountVerticalProfile；库拾样例 vertical=oss-picks，贴图=image_post 非表情包，out_of_scope 含表情包/情感漫等 |
| AC-SBC-010 | L4 VerticalGate：完整偏离项目/账号垂直（如表情包题）→ reason_code=vertical_gate_rejected，不得进入 awaiting_review；有意义价值陈述可通过 |
| AC-SBC-011 | 日批选题均绑定库拾画像且主题落在 GitHub 开源 AI 库 / AI Spec；题库与脚本不得含表情包/情侣/宠物等离题词；provenance 标明 intel-radar 或 in-vertical-fixture |

## Domain Model

| Entity | Key fields | Invariant / requires |
|--------|------------|----------------------|
| ProjectVerticalProfile | id, themes, must_include, must_exclude, value_tests | auditable; loaded before VerticalGate |
| AccountVerticalProfile | id, account_id, platform, vertical, brand, draft_tab_meaning, out_of_scope | requires ProjectVerticalProfile for 自媒体; 库拾 vertical=oss-picks；贴图=image_post 非表情包 |
| VerticalGate | proposal, project_profile, account_profile? | requires profiles; complete drift → reject (reason_code=vertical_gate_rejected) |
| BoundaryProposal | id, topic, query_hints, sources, rationale, project_profile_ref, account_profile_ref, provenance, status | requires project_profile_ref；自媒体另需 account_profile_ref；VerticalGate allow 才进 awaiting_review |
| BoundaryDecision | proposal_id, actor, decision, note, at | append-only；approve→active，reject→rejected |
| ActiveBoundary | proposal_id | requires BoundaryDecision(approve)；禁止 ingest 直达 / 跳过待审 |

## Entity Precedence

| Entity | Order | Requires |
|--------|-------|----------|
| ProjectVerticalProfile | 1 | — |
| AccountVerticalProfile | 2 | ProjectVerticalProfile |
| VerticalGate | 3 | ProjectVerticalProfile (+ AccountVerticalProfile when 自媒体) |
| BoundaryProposal | 4 | VerticalGate allow |
| BoundaryDecision | 5 | BoundaryProposal awaiting_review |
| ActiveBoundary | 6 | BoundaryDecision approve |

## Domain FSM — BoundaryProposal

| State | From | Guard |
|-------|------|-------|
| idle | — | profiles available |
| validating | idle | ingest received |
| awaiting_review | validating | fields_ok + profile_refs + VerticalGate allow |
| blocked | validating | missing_profile_ref / vertical_gate_reject / illegal |
| approved | awaiting_review | decide approve + actor |
| rejected | awaiting_review | decide reject + actor |
| active | approved | decide(approve) recorded |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| idle | ingest | fields_ok_and_profiles_present_and_gate_allow | awaiting_review | persist_proposal |
| idle | ingest | missing_profile_ref | blocked | reject_missing_profile |
| idle | ingest | vertical_gate_reject | blocked | reject_off_vertical |
| awaiting_review | decide_approve | actor_present | approved | activate |
| awaiting_review | decide_reject | actor_present | rejected | record_reject |
| awaiting_review | auto_approve | always | blocked | reject_illegal |
| idle | activate_skip_awaiting | always | blocked | reject_illegal |

### Legal walks

1. **W-SBC-01** idle → validating → awaiting_review（profiles + VerticalGate allow）· TC-SBC-001
2. **W-SBC-02** awaiting_review → approved → active（decide approve + actor）· TC-SBC-003
3. **W-SBC-03** awaiting_review → rejected（decide reject + actor）· TC-SBC-003

## Illegal transitions

- BoundaryProposal.idle → active without awaiting_review · TC-SBC-ILL-001
- BoundaryProposal.awaiting_review → active without decide · TC-SBC-007
- BoundaryProposal.validating → awaiting_review when VerticalGate reject · TC-SBC-ILL-002
- BoundaryProposal.validating → awaiting_review when missing_profile_ref · TC-SBC-008
- BoundaryProposal.idle → scheduled_tick with sticker_emoji_bank · TC-SBC-011

## Public CLI Contract

```text
adb boundary ingest ...
adb boundary pending [--json]          # lists awaiting_review
adb boundary show <id> [--json]
adb boundary decide <id> --actor A --decision approve|reject
adb boundary list [--status ...] [--json]
# wire status may still serialize as pending for CLI compat; truth token = awaiting_review
```

## Non-goals

- ADB 内嵌搜索引擎/爬虫内核
- 知识正文写入 Personal Brain
- 自动 approve / 跳过待审
- 飞书一键拍板（仍 PB）
- 跨账号串垂类创作（库拾出情感漫/航司）
- 把微信「贴图」做成表情包垂类

## Freeze readiness

- [x] Alignment / Phased / Deferral 已拍板（R2 L0–L4 湖；R3 release coverage）
- [x] Domain Model / Entity Precedence / Domain FSM / Legal walks / Illegal 已具体化
- [x] AC↔TC Command+Assertion 齐全；Domain FSM QA Matrix 含 legal/illegal
- [x] ui-state-matrix.v1 绑定 BoundaryProposal → CLI 表面
- [x] 库拾样例垂直=oss-picks；贴图≠表情包
