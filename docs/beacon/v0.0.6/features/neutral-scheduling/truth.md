---
slug: neutral-scheduling
version: v0.0.6
status: frozen
language: zh
domain_required: true
ux_required: false
package_maturity: filled
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
materials_status: current
canonical_refs:
  prd: docs/beacon/v0.0.6/features/neutral-scheduling/truth.md
  user_story: docs/beacon/v0.0.6/features/neutral-scheduling/truth.md
  test_case: docs/beacon/v0.0.6/features/neutral-scheduling/tests.md
---

# Requirement Truth: neutral-scheduling

## 人话

把 agent-delivery-bus 从「Beacon 专属调度器」改成**中立的多 agent 通信调度组件**：它通过强规则接口（派发信封 + binding manifest + evidence spec）接收任务，不规定交付任务的 truth gate 必须是 Beacon。Beacon 保留为内置参考 profile（现有 `### Beacon worker binding` 契约继续可用），但每个注册项目都可以声明自己的 truth gate、执行器和 worker binding profile；派工单里写清楚"证据放哪、要哪些文件、验收查什么"，并且证据必须绑定本次派发，旧证据不能冒充完成。

- 能做：项目级声明 truth_gate/executor/binding_profile；派工单带 evidence spec；非 Beacon profile 产出不含 Beacon 字段；closure 校验证据归属；文档按"通用调度层 + Beacon 参考实现"表述。
- 不能做：要求所有项目必须用 Beacon；派工单缺 evidence spec；用无 dispatch_id 归属的证据判完成；自动 release。
- 怎样算完：profile 化 + evidence spec + per-project 路由 + 证据归属校验全部实现；现有 beacon profile 行为兼容；pytest 全绿；本机 hermes 对注册项目可真实 dispatch/reconcile。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: neutral-scheduling
revision: R1
program: neutral-scheduling
depends_on: []
adapter_spi: TruthGateAdapter / ExecutorAdapter / MemoryAdapter
default_profile: beacon (reference, backward compatible)
custom_profile: project-registry-declared skill/command/evidence_spec
evidence_ownership: dispatch_id bound manifest, fail-closed on mismatch
release_gate: human always
```

## 用户旅程

1. 触发：项目注册时声明 `truth_gate` / `executor` / `binding_profile`（可省略，回落全局默认）。
1. 关键操作：`adb dispatch` 生成派工单，内含 binding manifest（profile、skill/command、runner）与 evidence spec（证据目录、glob、dispatch 绑定）。
1. 结果：worker 按 profile 执行并把证据写入声明路径；`adb reconcile` 用该项目 truth gate 的 closure 校验，证据绑定本次 dispatch 后 completed。
1. 异常：未知 profile / 未知适配器 → fail-closed（blocked，不派发）；证据缺失或 dispatch_id 不匹配 → 保持 reconciling；beacon 未安装时 beacon profile preflight 阻断并给出修复动作。

## First principles

- 系统边界：ADB 核心只依赖 SPI 与派发信封，不依赖任何 truth gate / executor 的具体实现。
- 不可变：派工单必须有 evidence spec；closure 必须绑定 dispatch_id；release 永远人工门；旧 beacon profile 契约不得静默破坏。
- 可推翻假设：非 beacon profile 的具体字段形态可由项目声明扩展，但信封 schema 必须版本化。

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-NS-001 | covered |
| INT-002 | user | must | AC-NS-002 | covered |
| INT-003 | user | must | AC-NS-003 | covered |
| INT-004 | user | must | AC-NS-004 | covered |
| INT-005 | user | must | AC-NS-005 | covered |
| INT-006 | user | must | AC-NS-006 | covered |
| INT-007 | user | must | AC-NS-007 | covered |
| INT-008 | user | must | AC-NS-008 | covered |

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-NS-001 | worker binding 改为 profile 制：内置 `beacon` profile 保留现有 `### Beacon worker binding` 段与字段；自定义 profile（如 `generic`）由项目声明 skill/command/runner，产出派工单不得包含 `beacon_skill`/`beacon_command` 字段 |
| AC-NS-002 | 派工单 body 必含 evidence spec（evidence_dir、glob、dispatch_id 绑定说明），envelope schema_version 升级（1.0→1.1）且旧 schema 解析兼容 |
| AC-NS-003 | Project 支持 per-project `truth_gate` / `executor` / `binding_profile`，缺失时回落全局配置；未知适配器/profile fail-closed，CLI 按项目解析 |
| AC-NS-004 | closure 校验证据归属：evidence manifest 中 dispatch_id 必须等于当前 dispatch；缺失、陈旧或无绑定证据 → closure 不通过并保持 reconciling |
| AC-NS-005 | 文档中立化：SKILL.md / usage-playbook / README / role-division 研究文档不再宣称 ADB 依赖 Beacon，统一表述为"通用调度层 + Beacon 参考实现" |
| AC-NS-006 | 兼容性：现有 beacon profile 消费者（worker-beacon-binding v0.0.3 契约）关键字段与行为保持；仅新增字段/版本，不破坏既有派工单解析 |
| AC-NS-007 | 本机交付验收：hermes executor + 注册项目可完成 dispatch（plan 或等价非受限 stage）并 reconcile；受限 stage 仍要求 approval token |
| AC-NS-008 | illegal：未知 profile/适配器静默回落、派工单缺 evidence_spec、closure 用无 dispatch_id 归属证据判完成 → fail-closed |

## Domain Model

| Entity | Key fields | Notes |
|--------|------------|-------|
| BindingProfile | slug, schema_version, stage_map(beacon_skill/command), runner, evidence_spec | beacon 为内置参考 profile；自定义 profile 由项目 registry 声明 |
| DispatchEnvelope | project_slug, stage, feature, idempotency_key, approval state, binding manifest, evidence spec | 派工单即强规则接口 |
| EvidenceSpec | evidence_dir, glob, required_files, dispatch_id_binding | 写入任务 body，closure 按此校验 |
| ProjectRouting | truth_gate, executor, binding_profile, global fallback | 每项目可路由到不同适配器 |
| EvidenceManifest | dispatch_id, stage, feature, files[] | closure 通过 manifest 确认归属 |

## Entity Precedence

| Entity | Order | Requires |
|--------|-------|----------|
| BindingProfile | 1 | 内置表或项目声明 |
| ProjectRouting | 2 | 已解析适配器与 profile |
| DispatchEnvelope | 3 | ProjectRouting 解析成功 |
| EvidenceManifest | 4 | worker 按 EvidenceSpec 产出证据 |
| ClosureVerdict | 5 | EvidenceManifest.dispatch_id == dispatch_id |

## Domain FSM — WorkerBindingClosure

| State | From | Guard |
|-------|------|-------|
| profile_requested | — | stage/feature 已归一化 |
| profile_resolved | profile_requested | profile 存在于内置表或项目/全局声明 |
| binding_emitted | profile_resolved | envelope schema 有效 + evidence_spec 完整 |
| evidence_pending | binding_emitted | 证据文件按 spec 存在 |
| closure_verified | evidence_pending | manifest.dispatch_id == dispatch_id 且 glob 命中 |
| closure_failed | evidence_pending | 缺证据 / dispatch_id 不匹配 / glob 不命中 |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| profile_requested | resolve(profile_slug) | profile 存在于内置表或项目声明 | profile_resolved | 记录 profile 来源（builtin/project/global） |
| profile_resolved | emit(stage, feature) | envelope schema 有效且 evidence_spec 完整 | binding_emitted | 生成 binding manifest + evidence spec |
| binding_emitted | worker_reports(evidence) | 证据文件按 spec 存在 | evidence_pending | 标记待 closure |
| evidence_pending | closure(manifest) | manifest.dispatch_id == dispatch_id 且文件命中 glob | closure_verified | transition completed |
| evidence_pending | closure(manifest) | 缺证据 / dispatch_id 不匹配 / glob 不命中 | closure_failed | 保持 reconciling，reason=truth_evidence_incomplete |

**终态：** closure_verified（任务级）；release 仍为独立人工门。

### Legal walks

1. **W-NS-01** profile_requested → profile_resolved → binding_emitted（profile 解析 + evidence_spec 完整）· TC-NS-001 / TC-NS-002
2. **W-NS-02** binding_emitted → evidence_pending → closure_verified（worker 按 spec 产证据 + dispatch_id 匹配）· TC-NS-004
3. **W-NS-03** project 路由：project 显式 profile → 按项目解析；未声明 → 全局回落 · TC-NS-003

## Illegal transitions

- profile_requested → binding_emitted without profile_resolved（未知 profile 静默回落默认）· TC-NS-ILL-001
- profile_resolved → binding_emitted without evidence_spec（派工单缺 evidence spec）· TC-NS-ILL-002
- evidence_pending → closure_verified with dispatch_id mismatch（旧证据/无 manifest 冒充完成）· TC-NS-ILL-003
- binding_emitted → closure_verified without evidence（无证据直接判完成）· TC-NS-ILL-004

## Non-goals

- 不新增 pi/其他执行器后端实现（只保证 SPI 可插拔）。
- 不改变 dispatch/approval/schedule 既有核心 FSM 语义。
- 不做自动 release、自动 merge、自动部署。
- 不删除或破坏 beacon profile 对旧消费者的兼容。

## Freeze readiness

- [x] Intent Matrix must 无 missing
- [x] 用户旅程无待补文案
- [x] First principles / Domain Model 已具体化
- [x] 业务 FSM 五元组 + illegal（非 draft/in_review 元状态）
- [x] 每 AC ≥ 1 TC（Command+Assertion）+ exec-layer TC
- [x] tasks 均有 ac= + evidence= 绑定
- [x] package_maturity: filled
- [ ] `beacon truth review` pass 后再 freeze
