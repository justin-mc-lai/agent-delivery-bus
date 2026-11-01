---
slug: requirement-index
version: v0.1.5
status: draft
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: draft
intent_confirmed: false
canonical_refs:
  truth: docs/beacon/v0.1.5/features/requirement-index/truth.md
  tests: docs/beacon/v0.1.5/features/requirement-index/tests.md
  tasks: docs/beacon/v0.1.5/features/requirement-index/tasks.md
  evidence: docs/beacon/v0.1.5/features/requirement-index/evidence.md
---

# Requirement Truth: requirement-index

## 人话

（一句话说清这个需求解决什么问题、给谁用、做到什么算完。冻结前由作者补齐。）

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: ""
goal: ""
non_goals: ""
constraints: ""
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | open | |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | (fill: 用户原话/目标/约束) | (fill: 证据路径、conversation:) |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 作者写下第一条可验证的完成标准（可被 tests.md 的 TC-001 直接验证） |

## FSM 五元组

Only required when frontmatter domain_required: true. Fill the
five-tuple, declare terminal states (终态： ...), and add an
Illegal Transitions section below listing forbidden moves
(- illegal: from -> to). The graph is machine-validated at
freeze: a missing target, a missing event, or a missing illegal
section blocks.

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|

## Parity Matrix

Only required when scope_mode: full_parity. List the source capability,
whether Beacon covers it, and the evidence.

| Capability | Covered | Evidence |
|------------|---------|----------|
