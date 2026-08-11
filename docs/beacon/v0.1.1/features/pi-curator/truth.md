---
slug: pi-curator
version: v0.1.1
status: frozen
language: zh
domain_required: true
ux_required: false
package_maturity: filled
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
materials_status: current
canonical_refs:
  prd: docs/beacon/v0.1.1/features/pi-curator/truth.md
  user_story: docs/beacon/v0.1.1/features/pi-curator/truth.md
  test_case: docs/beacon/v0.1.1/features/pi-curator/tests.md
---

# Requirement Truth: pi-curator (v0.1.1)

## 人话

把 pi agent 变成 ADB 的**选题策展器**：boundary 已批准的选题池 → 确定性构建知识检索请求（personal-brain 文件锚点 + agentmemory 可选）→ 宿主 agent（pi/承载 adb 的 LLM）回填证据化选题卡 → 校验后写回 personal-brain（ideas/）→ 复用 ScheduleService 定时 tick。同时并入 **pi-beacon 扩展包**（让 pi 原生加载 beacon/adb/prism 技能并注册 adb/prism 工具）与 **driver_pi 有界任务正文**（避免 pi 长循环）。

- 能做：approved 池读取；知识检索请求与文件锚点；选题卡 schema 校验；personal-brain 写回；CLI list/request/apply/tick；pi-beacon 扩展与安装器；driver_pi 有界正文；定时整合。
- 不能做：adb 自己调外部 LLM（选题卡由宿主回填）；curator 自动审批/自动 apply；越权写 knowledge_root 之外；release 自动放行。
- 怎样算完：CuratorService + CLI + 扩展包 + 有界正文全部有行为测试；knowledge_root 越界 fail-closed；host fill 缺锚点拒绝；既有 157 测试保持全绿；QA 通过后 release 仍人工门。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖（Lake B）
language: zh
scope_mode: lake
feature: pi-curator
revision: R1
program: pi-agent (v0.1.1)
depends_on:
  - pi-executor (v0.1.0)
  - workflow-lifecycle (v0.0.7)
first_party: beacon lifecycle（宿主回填选题卡）
executor: pi (driver_pi) + hermes（兼容并存）
knowledge_base: personal-brain（ideas/）+ agentmemory（可选）
release_gate: human always
```

## 用户旅程

1. 触发：定时 tick（hermes cron）或用户说"整理已批准的选题"。
2. 关键操作：`adb curator tick` → 对 approved 提案构建 curation request（含 knowledge anchors）→ 宿主回填 response → `adb curator apply` 校验 → 写 personal-brain ideas/ 选题卡。
3. 结果：选题卡带证据字段（topic/sources/knowledge_refs/status），可被创作链（prism）检索使用；curator_cards 账本记录。
4. 异常：无 approved 提案 → 空跑 pass；knowledge_root 缺失 → fail-closed；host fill 字段无锚点 → 拒绝；agentmemory 不可用 → 降级文件检索。

## First principles

- 系统边界：ADB 只做盘点/出题/校验/写回，语义分析由宿主 agent（pi）完成；pi 是执行面。
- 不可变：选题卡必须带证据字段；写回仅限声明 knowledge_root；不自动审批/apply；release 人工门。
- 可推翻假设：知识检索形态（文件扫描/agentmemory/gbrain）可扩展，但选题卡 schema 版本化后不静默改义。

## Domain Model

| Entity | Key fields | Invariant / requires |
|--------|------------|----------------------|
| CuratorProposal | proposal_id, topic, query_hints, sources, rationale | 来自 boundary approved 池 |
| CurationRequest | schema, proposal_id, topic, anchors[], prompt | 只读盘点产物 |
| TopicCard | topic, sources[], knowledge_refs[], market_signals[], status, created_at | 字段必须引用 anchors；dispatch_id 绑定 |
| KnowledgeAnchor | path, kind, excerpt | 来自 knowledge_root 扫描 |
| CuratorCardLedger | card_id, proposal_id, path, status | 追加式 SQLite 记录 |

## Entity Precedence

| Entity | Order | Requires |
|--------|-------|----------|
| CuratorProposal | 1 | boundary approved |
| KnowledgeAnchor | 2 | knowledge_root 可读 |
| CurationRequest | 3 | proposal + anchors |
| TopicCard | 4 | host fill 校验通过 |
| CuratorCardLedger | 5 | 写回成功 |

## Domain FSM — CuratorCardLifecycle

| State | From | Guard |
|-------|------|-------|
| requested | — | approved 提案存在 |
| anchored | requested | knowledge 扫描完成 |
| filled | anchored | host response schema 合法 |
| validated | filled | 字段有锚点引用 + schema 版本正确 |
| written | validated | 写入 knowledge_root/ideas/ 成功 |
| recorded | written | ledger 落账 |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| requested | knowledge_scan() | knowledge_root 可读 | anchored | 写 anchors |
| anchored | host_fill(response) | schema 合法 | filled | 暂存 response |
| filled | validate() | 字段引用 anchors | validated | 写校验结果 |
| validated | write_card() | 目标在 knowledge_root 内 | written | 写 markdown |
| written | record() | 文件已落盘 | recorded | 更新 ledger |

**终态：** recorded；release 仍为独立人工门。

### Legal walks

1. **W-PC-01** requested → anchored → filled → validated → written → recorded · TC-PC-004/TC-PC-005
2. **W-PC-02** tick 空 approved 池 → 空跑 pass · TC-PC-001
3. **W-PC-03** pi-beacon 扩展安装幂等（--dry-run）· TC-PC-006

## Illegal transitions

- filled → written without validate（无锚点字段直接写卡）· TC-PC-ILL-001
- validated → written outside knowledge_root · TC-PC-ILL-002
- curator 自动 apply / 自动审批 · TC-PC-ILL-003
- 伪造/缺失 dispatch_id 绑定 · TC-PC-ILL-004
- release 自动放行 · TC-PC-ILL-005

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-PC-001 | `pi_curator.py` CuratorService：读取 boundary approved 池；`tick` 对无提案空跑 pass；构建 curation request（proposal_id/topic/anchors/prompt） |
| AC-PC-002 | knowledge_scan 扫描 knowledge_root（ideas/daily/bases）按 topic token 命中文件生成 anchors；agentmemory 不可用时降级文件检索不抛错 |
| AC-PC-003 | 选题卡 schema（curator-card.v1）校验：topic/sources/knowledge_refs/market_signals/status 必填且引用 anchors；缺锚点 fail-closed |
| AC-PC-004 | 写回 personal-brain：`<knowledge_root>/ideas/<slug>-<date>.md` + YAML frontmatter + dispatch_id 绑定；越界写 fail-closed |
| AC-PC-005 | CLI：`adb curator list --status approved` / `request --proposal <id>` / `apply --proposal <id> --response-json <file>` / `tick --project <slug> --limit N`；curator_cards ledger 落账 |
| AC-PC-006 | pi-beacon 扩展包：`skills/pi-beacon/extension.ts`（registerTool adb_dispatch + registerCommand /prism + session_start 知识提示）+ `install.sh`（幂等合并 settings.skills、拷贝扩展，支持 --dry-run）；driver_pi create_task 追加有界任务正文（Bounded task 段） |
| AC-PC-007 | 定时整合：复用 ScheduleService should_run/quota 与 boundary cron 形态；curator 不自动审批/apply；hermes 兼容回归（既有 157 测试全绿） |

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-PC-001 | covered |
| INT-002 | user | must | AC-PC-002 | covered |
| INT-003 | user | must | AC-PC-003 | covered |
| INT-004 | user | must | AC-PC-004 | covered |
| INT-005 | user | must | AC-PC-005 | covered |
| INT-006 | user | must | AC-PC-006 | covered |
| INT-007 | user | must | AC-PC-007 | covered |

## Non-goals

- 不做创作生产（prism 承接）；不做发布；不做反馈指标采集。
- 不实现渠道入站桥与审批 actor 身份。
- 不引入 adb 调用外部 LLM（host fill 模式）。

## Freeze readiness

- [x] Intent Matrix must 无 missing
- [x] 用户旅程无待补文案
- [x] First principles / Domain Model 已具体化
- [x] 业务 FSM 五元组 + illegal
- [x] 每 AC ≥ 1 TC（Command+Assertion）+ exec-layer TC
- [x] tasks 均有 ac= + evidence= 绑定
- [x] package_maturity: filled
- [ ] `beacon truth review` pass 后再 freeze
