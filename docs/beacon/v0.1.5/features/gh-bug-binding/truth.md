---
slug: gh-bug-binding
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
  truth: docs/beacon/v0.1.5/features/gh-bug-binding/truth.md
  tests: docs/beacon/v0.1.5/features/gh-bug-binding/tests.md
  tasks: docs/beacon/v0.1.5/features/gh-bug-binding/tasks.md
  evidence: docs/beacon/v0.1.5/features/gh-bug-binding/evidence.md
---

# Requirement Truth: gh-bug-binding

## 人话

GitHub 上来的 issue 多数是 bug，不是新需求。现在 router 对 gh 线程只能贴一句"别乱改"就放行，
因为 thread 里只有仓库和编号，说不上它坏的是哪条约定。

这个湖要求 issue 自己声明归属：`beacon:ac=<slug>#<AC-ID>`。有声明才校验、才派活；
没有就拒绝并留在队列里。没有冻结 AC 的生产 bug 因此必须先回去补 AC、重新冻结，再修。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "issue 是 bug；有 AC 按 AC 修，没 AC 先 beacon 梳理后重新冻结，小版本修"
goal: "gh 线程必须声明它违反的已冻结 AC，否则拒绝派活"
non_goals: "不猜归属；不替 beacon 写需求；不改信任分级"
constraints: "stdlib only；标签来自现有轮询字段；拒绝留队列；非 beacon 不变"
scope_mode: "lake"
```

## 判定规则

| 情况 | 判定 | 动作 |
|------|------|------|
| 非 beacon 项目 | 无基线可挂 | 裸 worker，sdd=guard_only，行为不变 |
| 有 `beacon:no-ac` | 显式豁免（杂活） | 派活，sdd=guard_only，reason=issue_exempt_no_ac |
| 有 `beacon:ac=<slug>#<AC-ID>`，包已冻结，AC-ID 存在 | 合规 | 派活，sdd=gated，trace.ac_ref=<slug>#<AC-ID> |
| 有绑定，包未冻结 | 基线不可用 | 拒绝，reason=truth_not_frozen |
| 有绑定，AC-ID 不在包内 | 基线不存在 | 拒绝，reason=ac_not_found |
| 无任何绑定标签 | 说不清坏在哪 | 拒绝，reason=issue_unbound，条目留在队列 |
| 标签格式错、多个 ac 标签 | 歧义 | 按未绑定处理（拒绝） |

## 版本与冻结（AC-007）

无 AC 的 bug 想通过这道门，只能先补 AC：`truth change → 改 truth/tests → rebase → refreeze`，
补完再打标签。版本规则写进 change：只修行为不改约定 = patch；新增或修改 AC = minor。
大版本不因 bug 跳。

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 解析 issue 标签：`beacon:ac=<slug>#<AC-ID>` 取出 (slug, AC-ID)；`beacon:no-ac` 标记豁免；无标签、格式非法、出现多个 ac 标签时一律视为未绑定 |
| AC-002 | 已绑定、该 slug 的 truth.md 存在且 status 为 frozen、包内出现该 AC ID 时允许派活：sdd=gated，trace 写 `ac_ref=<slug>#<AC-ID>`，派活文本包含该 AC 引用与"回归必须让其它 AC 保持绿"一句 |
| AC-003 | beacon 管理项目的 gh 线程未绑定时拒绝派活：reason_code=issue_unbound，不建 worker、不写绑定、条目留在队列，进程退出码 0 |
| AC-004 | 带 `beacon:no-ac` 的 gh 线程照常派活：sdd=guard_only，reason_code=issue_exempt_no_ac，trace 留痕 |
| AC-005 | 已绑定但 truth 未冻结时拒绝派活：reason_code=truth_not_frozen，与 skill 线程同一判定 |
| AC-006 | 已绑定、已冻结但包内不存在该 AC ID 时拒绝派活：reason_code=ac_not_found |
| AC-007 | 未绑定被拒后，补 AC 必须经过 change→refreeze 才能再次通过；该 change 文件记录版本类型（只修行为=patch，增改 AC=minor）；trace 同时保留 ac_ref 与 reason_code 供审计 |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_bug_binding` | `on` | 绑定门总开关；`off` 时 gh 线程回落 guard_only 并记 `binding_off` |

## Non-goals（冻结边界）

- 不从 issue 正文推断归属（禁止猜）
- 不在 router 里创建或修改 truth
- 不改非 beacon 项目的派活行为

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | user utterance | truth.md#User-Intent-Snapshot |
| AC-002 | user utterance | truth.md#User-Intent-Snapshot |
| AC-003 | user utterance | truth.md#User-Intent-Snapshot |
| AC-004 | user utterance | truth.md#User-Intent-Snapshot |
| AC-005 | user utterance | truth.md#User-Intent-Snapshot |
| AC-006 | user utterance | truth.md#User-Intent-Snapshot |
| AC-007 | user utterance | truth.md#User-Intent-Snapshot |
