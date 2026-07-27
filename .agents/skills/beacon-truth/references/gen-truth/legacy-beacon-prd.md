# Legacy beacon-prd Skill Content

This reference preserves the full pre-v1.6.0 `beacon-prd` skill body. It is not a host-visible skill directory in v1.6.0; its authority is absorbed by `beacon-gen-truth` and package-authoritative truth.

# Beacon PRD

## Overview

This skill routes requirement-definition work through Beacon's `prd` surface.

In v1.5.2+, `beacon-prd` is a compatibility alias for `beacon-truth` mode `truth.define`.
Use `truth.define` when explaining the preferred lifecycle, and keep `beacon-prd` available as the stable invocation surface.
Compatibility reference: v1.4.7 docs line `docs/beacon/v1.4.7/`; old chain `research -> prd -> user-story -> test-case -> requirement_clarity -> freeze`.

The human-facing contract is aligned to the `v1.5.2` package-authoritative docs line. For v1.5.2 features, `features/<slug>/truth.md` is the normal requirement authority for problem, scope, goals, boundaries, acceptance criteria, and promised surface.
Legacy `prd / user-story / qa/test-cases` files may be rebuilt by `doctor`, absorbed by `change`, or retained as archive context, but they do not replace the feature package truth.

## When to Use

- PRD drafting
- requirement clarification
- open questions or ambiguous promised surface
- business ambiguity closure
- freeze-readiness before story generation

## Boundary

- `prd` does not silently rewrite unrelated frozen truth
- `brainstorm` may help convergence, but it does not replace `prd`
- `prd` does not make implementation or release decisions

## Workflow / Decision Loop

- continue in `prd` when problem, scope, goals, boundaries, or promised surface are still open
- route to `brainstorm` only for read-only convergence when truth is still open
- route to `change` when the request modifies frozen truth
- require richer research, business flow, state, or semantic modeling when the feature is complexity-triggered

## Common Rationalizations

- “先写个模板版 PRD，后面再补真实 scope。” -> 不允许；scope 需要先收敛
- “brainstorm 已经够清楚了，可以直接当正式承诺。” -> 不允许；正式 truth 仍需回 `prd`
- “先把实现想法写进 PRD，省得后面再补。” -> 不允许；`prd` 不预写实现剧本

## Red Flags

- feature 明显需要状态机/业务流/研究闭环，却只打算靠几段文字 freeze
- 把临时 runtime notes、goal 清单或 checkboxes 当成 requirement truth
- 在问题定义仍不稳定时，已经试图进入 `user-story` 或 `implement`

## Verification

- PRD 明确覆盖问题、目标、边界和 promised surface
- 官方术语、状态和页面名优先复用，而不是新造第二层命名
- 复杂需求缺失 research / flow / state truth 时会被显式补料，而不是假装 freeze-ready

## Evidence Produced

- requirement framing
- scope and boundary definition
- promised surface and freeze-readiness rationale

## State Updated

- PRD truth package
- requirement readiness for `user-story`

## Gate Impact

- unlocks `user-story` only when problem/scope/promise are stable enough
- blocks downstream stages when requirement framing is still ambiguous

## Inputs Contract

- `project-root`
- `version`
- `feature`
- resolver-selected feature package truth, normally `features/<slug>/truth.md`
- related `user-story`, `test-case`, `global-boundaries`, and architecture refs when present
- host bridge or local context that explains the current requirement gap

## Decision Protocol

- Continue in `prd` when the user is still defining problem, scope, goals, boundaries, or promised surfaces.
- Route to `brainstorm` only for read-only convergence when truth is still open and the user needs help resolving ambiguity.
- Route to `change` when the request modifies already frozen truth.
- Block shallow template fill when the feature clearly needs state, transition, route, or semantic modeling.
- Prefer reusing official wording from current version truth over inventing parallel terms.
- For complexity-triggered features, require a human-readable closure package that covers research, architecture, business flow, and the relevant diagram truth before freeze.
- Treat that closure package as part of the formal requirement chain, not optional commentary; freeze should only happen after the chain reaches `test-case -> requirement_clarity`.

## Anti-Inertia Notes

- `prd` is not a place to freeform rewrite unrelated frozen truth.
- `brainstorm` is not a new lifecycle stage and does not replace `prd`.
- `goal`, checkboxes, or temporary runtime notes do not define requirement truth.
- `prd` cannot rewrite frozen truth or override gate truth by itself.
- If UX/UI handoff is real, `prototype` may be triggered later; do not front-load visual polish into PRD text.

## Learning Notes

- Read learning as hint, not truth.
- Prefer `feature -> host -> runner -> domain` disclosure order when prior learning exists.
- Downgrade stale or superseded learning instead of forcing it into current requirement language.
- If learning conflicts with frozen truth, frozen truth wins.

## Backstop CLI

- `beacon prd ...`
- `beacon brainstorm ...` for read-only convergence
- `beacon change ...` for frozen-truth revision

## v1.5.2 Package-Authoritative Read Order

For v1.5.2+ features, read the resolver output before consuming requirement material:

```bash
beacon truth-map resolve <feature> --project . --version v1.5.2 --json
```

Then read `features/<slug>/{truth,tests,tasks,evidence}.md` from the resolved paths. The Markdown feature package is the requirement authority; resolver output, `.machine` JSON, manifests, QA status, and release reports are generated state, not requirement truth.

## v1.5.2 Feature Truth Quick Template

When writing `features/<slug>/truth.md`, keep it parser-friendly and explicit that the feature package is authoritative:

```markdown
---
slug: example-feature
version: v1.5.2
status: draft
language: en
canonical_refs:
  prd: docs/beacon/v1.5.2/features/example-feature/truth.md
  user_story: docs/beacon/v1.5.2/features/example-feature/truth.md
  test_case: docs/beacon/v1.5.2/features/example-feature/tests.md
parser_contract: beacon-feature-package-v1
truth_source_model: feature_package_authoritative
truth_map_contract: beacon-truth-map-v1
---

# Feature: Example Feature

## Acceptance Criteria

| AC ID | Theme | Description |
|-------|-------|-------------|
| AC-001 | example | The feature package is the authoritative requirement source. |
```

This template is authoring support only. CLI validation decides whether the package is accepted as authoritative feature truth.

## Supporting References

Use these lightweight references when they reduce ambiguity faster than free-form prompting:

- `../references/requirement-closure-checklist.md`
- `../references/research-closure-checklist.md`
- `../references/business-flow-checklist.md`
- `../references/state-machine-checklist.md`
- `../references/freeze-readiness-checklist.md`

These are support material only. They help close requirement reasoning, but they do not replace resolver-selected feature package truth.

## Examples

- 适用例：
  - “我还在定义这个 feature 到底解决什么问题、边界是什么、承诺面是什么。”
  - “需求里还有很多模糊点，需要先把问题 framing 和 scope 收口。”
- 不适用例：
  - “PRD 已经很清楚了，现在要把 acceptance criteria 编译出来。” -> 应去 `beacon-user-story`
  - “需求已经冻结，我要正式改写它。” -> 应去 `beacon-change`

## Cold-start anchors

- `prd` 解决的是问题定义、目标、边界和 promised surface。
- `brainstorm` 可以帮忙 challenge 和收敛，但不能代替 `prd` 冻结正式 truth。
- 不要在 `prd` 里提前做实现、视觉或 release 层判断。
- 复杂需求不能只靠文字段落冻结；缺少 research / business flow / state-machine / decision-priority 闭环时，应回补而不是假装 freeze-ready。
- v1.5.2 的最小可执行 requirement contract 要落在 feature package：`research/support -> truth.md -> tests.md -> requirement_clarity -> freeze`。
