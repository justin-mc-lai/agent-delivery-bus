---
slug: workflow-lifecycle
version: v0.0.7
status: frozen
language: zh
domain_required: true
ux_required: false
package_maturity: filled
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
materials_status: current
canonical_refs:
  prd: docs/beacon/v0.0.7/features/workflow-lifecycle/truth.md
  user_story: docs/beacon/v0.0.7/features/workflow-lifecycle/truth.md
  test_case: docs/beacon/v0.0.7/features/workflow-lifecycle/tests.md
---

# Requirement Truth: workflow-lifecycle (v0.0.7)

## 人话

把 ADB 做成真正的通用调度内核：它能加载 beacon 生命周期 skill 工作流（plan/truth/implement/qa/freeze/goal，分别对应 beacon-plan/beacon-truth/beacon-implement/beacon-qa/beacon-goal），作为第一方能力稳定可用；飞书、微信、Line 等渠道用同一张关键词表，同一句话得到同一个解析结果；内置 superpowers、openspec 两个开源对标 skill 工作流预设；用户丢任意开源库时，adb 只做确定性盘点并出"分析题"，由承载它的宿主 agent（Codex/Claude/Hermes 会话）回填答案，adb 校验后给草案，人工确认后才安装；整个分析/校验/安装/派发过程写 JSONL trace，可排查可回放，接入后能通过验收探针再用 adb 调度，坏库永远 fail-closed。

- 能做：beacon 六阶段派发与 skill 强制加载；渠道无关关键词表；superpowers/openspec 预设；开源库通用适配（宿主回填）；trace/debug/replay/verify。
- 不能做：adb 自己调用外部 LLM；无人工确认安装工作流；安装危险命令或无证据字段的工作流；伪造 trace。
- 怎样算完：六阶段 force-load + 同句话三渠道同 envelope + 预设可装 + 任意本地库 ingest→宿主回填→确认→verify→dispatch 闭环 + trace 可查 + pytest/QA 全绿。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: workflow-lifecycle
revision: R1
program: workflow-lifecycle
depends_on: []
first_party: beacon lifecycle (plan/truth/implement/qa/freeze/goal)
presets: superpowers, openspec (skill workflows)
channel_keywords: single canonical map (feishu/weixin/line agnostic)
ingestion_mode: host-agent fill (adb never calls an external LLM)
trace: workflow-trace.v1 JSONL
release_gate: human always
```

## 用户旅程

1. 触发：用户在任意渠道（飞书/微信/Line/Codex/Claude）对承载 adb 的 agent 说调度或工作流管理意图。
2. 关键操作：adb intent 解析（规范关键词表）→ 派发六阶段或管理工作流（install/ingest/draft/confirm/verify）。
3. 结果：任务派给 hermes worker 并 force-load 绑定 skill；证据按 evidence_spec 产出；reconcile 完成；工作流接入先 verify 再正式派发。
4. 异常：缺 skill → binding_skill_missing；坏库/无锚点字段 → 校验 fail-closed；宿主未回填 → draft 缺失；确认前不安装；release 永远人工门。

## First principles

- 系统边界：adb 是宿主 agent 内的 skill；语义分析由宿主 agent 完成，adb 只做盘点/出题/校验/绑定/派发/验收。
- 不可变：安装需人工确认；字段需文件证据；危险命令拒绝；trace 必须可回放；release 人工门。
- 可推翻假设：预设列表可扩展，渠道别名可扩展，但规范关键词表版本化后不静默改义。

## Domain Model

| Entity | Key fields | Invariant / requires |
|--------|------------|----------------------|
| WorkflowBinding | name, source, commit, stages, runner, evidence_spec, skills | stages 非空；evidence_spec 必填；skills 可校验 |
| StageBinding | stage, skill, command, public_harness | 命令模板变量可解析；skill 存在于绑定中 |
| KeywordMap | stage, aliases(zh/en/channel) | 单真值；渠道不各自硬编码 |
| AnalysisRequest | anchors[], schema, prompt, source_commit | 只读盘点产物 |
| HostFillResponse | fields with evidence refs | 每字段必须引用 anchors 内文件 |
| WorkflowDraft | response, validation, status | 未确认不可安装 |
| WorkflowTrace | events[] (inventory/request/host_fill/validation/install/dispatch/reconcile) | JSONL 追加式；可回放 |

## Entity Precedence

| Entity | Order | Requires |
|--------|-------|----------|
| KeywordMap | 1 | 版本化规范表 |
| WorkflowBinding | 2 | 预设或本地配置 |
| AnalysisRequest | 3 | 源码盘点 |
| HostFillResponse | 4 | AnalysisRequest + 锚点文件 |
| WorkflowDraft | 5 | 校验通过 |
| WorkflowTrace | 6 | 全程事件 |

## Domain FSM — WorkflowLifecycle

| State | From | Guard |
|-------|------|-------|
| requested | — | source 可读、盘点完成 |
| filled | requested | host_fill_response schema 合法 |
| validated | filled | 字段有证据 + 命令安全 + 模板可解析 |
| draft | validated | 校验通过 |
| confirmed | draft | 人工确认 |
| installed | confirmed | 写入 config workflows，记录 commit+trace_id |
| verified | installed | verify 探针全过 |
| bound | verified | 项目 binding_profile=workflow |
| dispatched | bound | skill 就绪 + preflight pass |
| reconciled | dispatched | closure 证据通过 |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| requested | host_fill(response) | schema 合法 | filled | 记录响应与证据 |
| filled | validate() | 字段有证据 + 无危险命令 | validated | 写校验结果 |
| validated | confirm(actor) | 人工确认 | confirmed | 标记确认 |
| confirmed | install() | commit 记录 | installed | 写入 workflows |
| installed | verify() | skill/命令/evidence/dry-run 全过 | verified | 写 verify 报告 |
| verified | bind(project) | 项目存在 | bound | 设置 binding_profile |
| bound | dispatch(stage) | skill 就绪 + preflight | dispatched | force-load skill |
| dispatched | reconcile() | closure pass | reconciled | transition completed |

**终态：** reconciled；release 仍为独立人工门。

### Legal walks

1. **W-WF-01** requested→filled→validated→draft→confirmed→installed→verified→bound→dispatched→reconciled · TC-WF-004/007
2. **W-WF-02** 六阶段派发（plan/truth/implement/qa/freeze/goal）→ force-load 对应 skill · TC-WF-001
3. **W-WF-03** 三渠道同句话 → 同一 envelope · TC-WF-002

## Illegal transitions

- filled → confirmed without validate（无证据字段直接安装）· TC-WF-ILL-001
- draft → installed without human confirm · TC-WF-ILL-002
- validated → installed with dangerous command（rm -rf 等）· TC-WF-ILL-003
- installed → dispatched without verify · TC-WF-ILL-004
- 任何阶段伪造/缺失 trace 事件 · TC-WF-ILL-005

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-WF-001 | beacon 生命周期六阶段（plan/truth/implement/qa/freeze/goal）可经 adb 派发，worker 任务 force-load 对应 beacon skill；设备缺 skill → binding_skill_missing fail-closed |
| AC-WF-002 | 规范关键词表唯一真值；`adb intent keywords --json` 输出机器可读；飞书/微信/Line 同一句话得到同一 envelope |
| AC-WF-003 | `adb workflow install --preset superpowers|openspec` 可安装并可被项目绑定；预设为 skill 工作流形状（stages→skill/command/evidence） |
| AC-WF-004 | `adb workflow ingest <本地路径|URL>` 只读盘点 → 产出 analysis request（anchors+commit）→ 宿主 agent 回填 response → 校验 → draft → 人工确认 → install（记录 commit+trace_id） |
| AC-WF-005 | 全程 JSONL trace（inventory/analysis_request/host_fill/validation/install/dispatch/reconcile）；`adb workflow trace/debug/replay` 可用 |
| AC-WF-006 | `adb workflow verify <name>` 验收探针：skill 存在、命令模板可解析、evidence_spec 合法、各 stage dry-run preflight pass |
| AC-WF-007 | 工作流绑定后 dispatch→reconcile 闭环；坏库/无锚点字段/危险命令 → fail-closed 不安装 |
| AC-WF-008 | illegal：无确认安装、危险命令、无证据字段、伪造 trace → fail-closed |

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-WF-001 | covered |
| INT-002 | user | must | AC-WF-002 | covered |
| INT-003 | user | must | AC-WF-003 | covered |
| INT-004 | user | must | AC-WF-004 | covered |
| INT-005 | user | must | AC-WF-005 | covered |
| INT-006 | user | must | AC-WF-006 | covered |
| INT-007 | user | must | AC-WF-007 | covered |
| INT-008 | user | must | AC-WF-008 | covered |

## Non-goals

- adb 不内置/不调用外部 LLM（分析由宿主 agent 完成）。
- 不自动安装工作流；不执行仓库代码或构建。
- 不替换 beacon 的判定与 release 门。
- 预设只含开源第三方 skill 工作流；beacon 作为第一方能力，不进预设列表。

## Freeze readiness

- [x] Intent Matrix must 无 missing
- [x] 用户旅程无待补文案
- [x] First principles / Domain Model 已具体化
- [x] 业务 FSM 五元组 + illegal（非 draft/in_review 元状态）
- [x] 每 AC ≥ 1 TC（Command+Assertion）+ exec-layer TC
- [x] tasks 均有 ac= + evidence= 绑定
- [x] package_maturity: filled
- [ ] `beacon truth review` pass 后再 freeze
