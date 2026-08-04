---
slug: search-boundary-curation
version: v0.0.5
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
program: personal-production-flywheel
promotion_ref: user-ack-search-boundary-2026-08-04
depends_on: vision-flywheel
canonical_refs:
  prd: docs/beacon/v0.0.5/features/search-boundary-curation/truth.md
  user_story: docs/beacon/v0.0.5/features/search-boundary-curation/truth.md
  test_case: docs/beacon/v0.0.5/features/search-boundary-curation/tests.md
---

# Requirement Truth: search-boundary-curation（网络搜索边界整理 · 待审拍板）

## 人话

定时把「新的网络搜索边界」提案收进 ADB **待审队列**，你人工拍板后才生效。ADB 不自己上网搜，也不自动启用边界；Hermes cron 到点触发整理脚本 → ADB `ingest` → `pending` → 你 `decide`。未拍板的提案不得进入 active。

## User Intent

> 定时整理新的网络搜索边界 → 进待审 → 人工拍板；可验收可交付，并配置本机 Hermes。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: search-boundary-curation
program: personal-production-flywheel
depends_on: [vision-flywheel]
cron_owner: hermes
auto_approve_on_ingest: false
adb_stores_knowledge_body: false
```

## Alignment Surface（本 revision 必煮干）

1. **提案入库**：`adb boundary ingest` 接收 topic / query_hints / sources / rationale，写入 pending。
2. **待审列表**：`adb boundary pending`（并进入统一待拍板视图）。
3. **人工拍板**：`adb boundary decide <id> --decision approve|reject`；只有 approve 才 active。
4. **定时触发**：复用 schedule 登记 + Hermes cron 脚本；脚本只做整理+ingest，不 auto-approve。
5. **证据**：每次 ingest / decide 可审计；schedule spend-after-evidence 仍适用。

## Phased Backlog（显式不在本包）

| ID | 内容 | revisit |
|----|------|---------|
| PB-SBC-1 | 真实多源网络搜索 / LLM 选题（归 Hermes skill 深化） | v0.0.6 |
| PB-SBC-2 | 知识库正文写入 Personal Brain | 跨项目，不进 ADB |
| PB-SBC-3 | 飞书卡片一键拍板边界 | 体验增强 |

## Deferral Ledger

| ID | Item | user_decision | note |
|----|------|---------------|------|
| D1 | auto-approve-on-ingest | rejected | 必须人工拍板 |
| D2 | adb-embedded-web-search | rejected | 搜索执行在 Hermes/脚本侧 |
| D3 | knowledge-body-in-adb | rejected | 与 knowledge-curation-digest 一致 |
| D4 | auto-release | rejected | global |
| D5 | auto-dispatch-from-heartbeat | rejected | 延续 vision-flywheel |

## 用户旅程

1. Hermes cron 到点 → should-run → 整理脚本产出边界提案 → `adb boundary ingest`。
2. 你查看 `adb boundary pending` / `adb approvals awaiting`。
3. `adb boundary decide --decision approve|reject`。
4. 仅 approved 出现在 `adb boundary list`（active）。

### 失败旅程

- 缺字段 ingest → blocked + reason_code。
- 对已决定条目重复 decide → blocked（idempotent reject / already_decided）。
- 试图 ingest 时 auto-activate / skip pending → illegal fail-closed。

## Domain Model

| Entity | Key fields | Invariant |
|--------|------------|-----------|
| BoundaryProposal | id, topic, query_hints, sources, rationale, status | ingest → pending only |
| BoundaryDecision | proposal_id, actor, decision, note, at | append-only; approve\|reject |
| ActiveBoundary | proposal_id (=approved) | only via decide(approve) |

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

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-SBC-001 | `adb boundary ingest` 写入 pending 提案（topic 必填；query_hints/sources 可多值）；缺 topic 拒绝 |
| AC-SBC-002 | `adb boundary pending` 列出全部 pending；`show <id>` 返回单条 |
| AC-SBC-003 | `adb boundary decide <id> --actor A --decision approve\|reject`；approve→active，reject→rejected |
| AC-SBC-004 | `adb boundary list` 默认仅 active（approved）；可用 `--status` 过滤 |
| AC-SBC-005 | 待审提案进入统一待拍板视图（`approvals awaiting` 含 `kind=boundary_pending`） |
| AC-SBC-006 | 定时路径：schedule 登记 + Hermes cron 脚本只 ingest，不 auto-approve；可跑 fixture 提案 |
| AC-SBC-007 | illegal：ingest 直达 active、跳过 pending 启用、自动 approve |

## Domain FSM — BoundaryReview

| State | From | Guard |
|-------|------|-------|
| idle | — | cron/CLI ingest trigger |
| pending | idle | ingest valid |
| approved | pending | human decide approve |
| rejected | pending | human decide reject |
| blocked | * | illegal / invalid |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| idle | ingest | fields_ok | pending | persist_proposal |
| pending | decide_approve | actor_present | approved | activate |
| pending | decide_reject | actor_present | rejected | record_reject |
| pending | auto_approve | always | blocked | reject_illegal |
| idle | activate_skip_pending | always | blocked | reject_illegal |

## Illegal transitions

- ingest → approved · TC-SBC-ILL-001
- pending → active without decide · TC-SBC-ILL-002
- idle → approved · TC-SBC-ILL-001

## Public CLI Contract

- `adb boundary ingest --topic <t> [--query <q>]... [--source <s>]... [--rationale <r>] [--json]`
- `adb boundary pending [--json]`
- `adb boundary show <id> [--json]`
- `adb boundary decide <id> --actor <a> --decision approve|reject [--note <n>] [--json]`
- `adb boundary list [--status pending|approved|rejected|all] [--json]`
- Hermes: `~/.hermes/scripts/adb-search-boundary-tick.sh` + cron 登记

## Non-goals

- ADB 内嵌搜索引擎 / 爬虫
- 知识正文入库
- 自动 approve / 跳过待审
- 飞书一键拍板（PB-SBC-3）

## Freeze readiness

- [x] Alignment / Phased / Deferral 已拍板
- [x] FSM + Illegal 已具体化
- [x] AC↔TC Command+Assertion 齐全
- [x] 湖级：仅待审拍板闭环；搜网执行外置
