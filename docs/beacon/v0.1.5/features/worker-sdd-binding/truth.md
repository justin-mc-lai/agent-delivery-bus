---
slug: worker-sdd-binding
version: v0.1.5
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v0.1.5/features/worker-sdd-binding/truth.md
  tests: docs/beacon/v0.1.5/features/worker-sdd-binding/tests.md
  tasks: docs/beacon/v0.1.5/features/worker-sdd-binding/tasks.md
  evidence: docs/beacon/v0.1.5/features/worker-sdd-binding/evidence.md
---

# Requirement Truth: worker-sdd-binding

## 人话

派出去的 worker 现在只被提示词管着，想改代码就改，没有冻结需求也能动手——代码变了真相没变，就是漂移。
这个湖加一道结构门：beacon 管的项目，只有对应的需求**已经冻结**，才允许派活；
没冻结就拒绝、留在队列，等真相冻好再派。不在 beacon 管的项目（内容、调研）照旧。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "确保需求意图按照规范的beacon进行交付，不漂移"
goal: "beacon 管的项目在派活前必须有冻结 truth；没有就结构化拒绝。"
non_goals: "adbd 不代管 goal run 生命周期；不改 beacon 真相；不接飞书；不做 UI。"
constraints: "Go 放 cmd/adbd（stdlib only）；真相只读；拒绝时条目留队列；非 beacon 项目不变。"
scope_mode: "lake"
```

## 判定规则

| 情况 | 判定 | 动作 |
|------|------|------|
| 仓库没有 `docs/beacon/<v>/features/` | 非 beacon 项目 | 裸 worker，照旧，trace 记 `sdd=not_applicable` |
| beacon 项目 + thread 是 `skill:<project>:<slug>` + 该 slug 的 truth.md 存在且 `status: frozen` | 可派 | 正常派活，trace 记 `sdd=gated` |
| beacon 项目 + `skill:` thread + 该 slug 的 truth.md 存在但未冻结 | 拒绝 | `reason_code=truth_not_frozen`，不建 worker，条目留队列 |
| beacon 项目 + `skill:` thread + 该 slug 的 truth.md 不存在 | 拒绝 | `reason_code=feature_not_registered`，不建 worker，条目留队列 |
| beacon 项目 + thread 不是 `skill:`（例如 `gh:`） | 无法映射到需求 | 派活但 trace 记 `sdd=guard_only`，并在派活文本里带 SDD 守卫段 |

## 用户旅程

1. 有人把 `skill:studio:ac-003` 的活送进 router。
2. adbd 先查 studio 是不是 beacon 项目，再查 ac-003 的 truth.md。
3. 没冻结：不建 worker，输出 `truth_not_frozen`，条目留在队列——重复发送也不会绕过。
4. 冻结了：正常派活，trace 里 `sdd=gated`，worker 的提示词带"先看 spine、无冻结 AC 不改代码"。
5. 内容类项目（无 docs/beacon）照旧，记 `sdd=not_applicable`。

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | goal：beacon 项目要有门 | truth.md#判定规则 |
| AC-002 | goal：没冻结不许派 | truth.md#判定规则 |
| AC-003 | goal：没登记不许派 | truth.md#判定规则 |
| AC-004 | goal：拒绝不丢活 | truth.md#判定规则 |
| AC-005 | goal：worker 要知道规矩 | truth.md#Acceptance-Criteria |
| AC-006 | goal：非 beacon 不受影响 | truth.md#判定规则 |
| AC-007 | goal：可配置可审计 | truth.md#Acceptance-Criteria |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `adbd` 能判定一个项目根是否为 beacon 管理：存在 `docs/beacon/<version>/features/` 且该 version 目录可枚举；判定结果写入 trace 字段 `sdd`（`gated` / `guard_only` / `not_applicable`） |
| AC-002 | 对 beacon 项目且 thread 形如 `skill:<project>:<slug>`：`<repo>/docs/beacon/<version>/features/<slug>/truth.md` 的 frontmatter `status` 不是 `frozen` 时，`route` 不创建 worker，输出 `reason_code=truth_not_frozen`，条目保留在慢路径队列，退出码 0 |
| AC-003 | 同上但 `<slug>/truth.md` 不存在时，不创建 worker，输出 `reason_code=feature_not_registered`，条目保留在慢路径队列 |
| AC-004 | 拒绝路径不得修改 worker 绑定、不得调用 `paseo run`、不得丢弃队列条目；连续投递同一消息仍按幂等键去重 |
| AC-005 | 允许派活时，派给 worker 的文本以固定 SDD 守卫段开头（含 `beacon-go spine show` 与"没有冻结 AC 不得修改代码"两句）；bootstrap 提示词同步包含同一守卫段 |
| AC-006 | 非 beacon 项目（无 `docs/beacon/*/features/`）保持现状：直接建 worker，trace 记 `sdd=not_applicable`，没有任何拒绝 |
| AC-007 | 新增 `adbd_sdd_gate`（默认 `on`，可设 `off` 关闭整道门）与 `adbd_beacon_docs_root`（默认 `docs/beacon`）；关闭时 trace 记 `sdd=gated_off` 且不拒绝；配置与判定都可在 `--json` 输出中看到 |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_sdd_gate` | `on` | 总开关；`off` 时不拒绝，只记 `sdd=gated_off` |
| `adbd_beacon_docs_root` | `docs/beacon` | beacon 文档根，相对项目目录 |

## 禁止的迁移（人工维护，内核不校验）

- illegal: refused -> dispatched（必须先冻真相，不能靠重试绕过）

## Non-goals（冻结边界）

- adbd 不创建、不推进、不完成 goal run
- 不修改 beacon 真相
- 不改非 beacon 项目的派活行为
- 不做 UI

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | truth 状态从哪里读？ | 读 truth.md frontmatter 的 status（包是唯一权威），不调 beacon-go CLI | answered | 读 truth.md frontmatter status；adbd 不依赖 beacon-go CLI 也能判。 | human |
