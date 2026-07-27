# Mode: gen-implement

> Archived from `beacon-gen-implement` during 1+6 merge. This is progressive disclosure content for `beacon-implement`.

# Beacon Generator Implementation

## Overview

This skill routes execution work through Beacon's `implement` surface.

`implement` remains the only public implementation entrypoint. It consumes frozen requirement truth and execution governance context without rewriting those truths.
The active human-facing contract is aligned to the current `v1.5.2` package-authoritative docs line.
Compatibility reference: v1.4.7 docs line `docs/beacon/v1.4.7/`.
For v1.5.2 features, implementation must bind to resolver-selected `features/<slug>/{truth,tests,tasks,evidence}.md`. Legacy truth files may be rebuilt, absorbed, or archived, but they must not override the feature package.

## When to Use

- implementation planning
- implementation execution
- implementation progress or handoff checks
- implementation readiness after freeze and technical review
- requirement completeness checks before execution

## Boundary

- `implement` is not permission to rewrite requirement truth
- it does not replace `change`
- team/runtime internals stay internal unless explicitly requested

## Workflow / Decision Loop

- enter only after requirement truth is frozen enough for execution
- block when truth must reopen, cutover guardrails fail, or conditionally required prototype is missing
- execute against the frozen package and current delivery governance
- hand off to `qa` once implementation evidence is ready

## Common Rationalizations

- “代码先做，后面再补 truth。” -> 不允许；truth 漂移要先回 `change`
- “只是顺手改旁边一点逻辑，不用看冻结范围。” -> 不允许；实现必须受冻结边界约束
- “prototype 还没补齐，但先写代码问题不大。” -> 不允许；真实 UX/UI handoff 不能跳过
- “PRODUCT.md / DESIGN.md 只是参考文档，我先按通用直觉做。” -> 不允许；当项目存在 DESIGN.md 时，实现必须将其作为视觉基线读取和对齐。通用设计直觉不能覆盖项目特定的设计系统。

## Red Flags

- requirement package 仍不稳定，却已经试图进入 execution
- 双读/兼容窗口存在，但准备忽略 fail-closed guardrail
- implementation progress 被当成 completion truth

## Verification

- execution 入口明确绑定 frozen feature package truth and coverage
- 当 `PRODUCT.md` / `DESIGN.md` 存在时，实现产物必须对齐其中的设计基线（颜色、间距、圆角、阴影、组件规范）
- blocked reason、gate context、branch/execution governance 可解释
- prototype 条件触发时，不会被静默跳过

## Evidence Produced

- implementation plan or execution summary
- implementation evidence and handoff context
- blocked reason or repair route when execution cannot start

## State Updated

- implementation progress state
- execution governance/read model state

## Gate Impact

- unlocks `qa` only when implementation evidence is sufficient
- blocks execution when truth drift, prototype gate, or delivery guardrails fail

## Inputs Contract

- `project-root`
- `version`
- `feature`
- frozen `features/<slug>/truth.md` and `features/<slug>/tests.md`
- conditionally required `prototype` when the feature has real UX/UI handoff
- current gate / blocked reason / branch or execution governance context when present
- `PRODUCT.md` — project-level product truth; must-read when the file exists at the project root
- `DESIGN.md` — project-level design truth (colors, typography, spacing, tokens, Do's & Don'ts); must-read when the file exists at the project root
- `docs/beacon/<version>/design/*` — version-wide design contracts (style, component-system, interaction-contract, state-matrix); must-read when they exist
- `src/App.vue` or equivalent global CSS variable root — for projects with a design system, implement must reference existing CSS variables instead of hardcoding visual values

## Decision Protocol

- Enter `implement` only after requirement truth is frozen enough for execution.
- Block when frozen truth requires `change`, when cutover guardrails fail, or when a conditionally triggered prototype is still missing.
- When truth claims visual delivery (`ux_required` / UI pages) and `DESIGN.md` is missing, run `beacon design baseline --project . --surface auto --write` before implement (unattended surface auto-select: pc-console/web/h5/client).
- When `PRODUCT.md` or `DESIGN.md` exists at the project root, read both before starting any implementation that touches UI, visual design, or user-facing behavior. Treat them as binding design contracts, not optional reference material.
- Keep `implement` as the only execution entry; `ask --execute` and support surfaces may route here but must not replace it.
- Prefer explanation and routing logic in skill surface; keep final schema, contract, and gate writes in Python guardrails.
- When dual-read or compatibility windows exist, fail closed on mismatch.

## Anti-Inertia Notes

- `implement` is not permission to rewrite requirement truth.
- `goal`, task ledgers, or partial execution progress do not define completion truth.
- If the feature needs UX/UI handoff and prototype is triggered, do not skip straight into coding.
- Team/runtime internals stay internal unless the user explicitly asks for them.
- When `PRODUCT.md` or `DESIGN.md` exists at the project root, `implement` must read them before building any UI. Skipping design truth is a process drift that produces implementation-to-design mismatch.
- `implement` cannot rewrite frozen truth or override gate truth by itself.

## Learning Notes

- Reuse implementation learning to avoid repeated blocked paths or flaky cutovers.
- Learning may shape route explanation and caution, but cannot override gate truth.
- Downgrade stale learning instead of silently trusting it.

## Backstop CLI

- `beacon implement ...`
- `beacon change ...` when truth must reopen
- `beacon qa ...` after implementation evidence is ready

## v1.5.2 Package-Authoritative Read Order

For v1.5.2+ features, read the resolver output before consuming implementation inputs:

```bash
beacon truth-map resolve <feature> --project . --version v1.5.2 --json
```

Then read `features/<slug>/{truth,tests,tasks,evidence}.md` from the resolved paths. Markdown package files are the requirement authority; resolver output, `.machine` JSON, QA status, and release reports are generated state, not requirement truth.

## v1.5.2 Feature Package Quick Template

When writing `features/<slug>/tasks.md`, use stable checkbox rows. These rows are a task ledger, not gate truth:

```markdown
# Tasks: Example Feature

## Task Ledger

- [ ] TASK-001 Materialize feature package
- [x] TASK-002 Validate frontmatter
- [-] TASK-003 Repair blocked package diagnostics
```

Do not treat checked rows as implement, QA, or release completion. CLI gates remain authoritative for state progression.

## Supporting References

Use these lightweight references when implementation needs a tighter support frame before coding or repair:

- `references/gen-implement/implement-single.md`
- `references/gen-implement/implement-team.md`
- `references/gen-implement/implement-ralph.md`
- `../references/business-flow-checklist.md`
- `../references/state-machine-checklist.md`
- `../references/support-surface-routing-cheatsheet.md`

Mode references explain execution shape only. They do not create `beacon-gen-implement-single`, `beacon-gen-implement-team`, or `beacon-gen-implement-ralph` public skills.
Ralph mode is a closure controller for next-action routing; it does not replace QA, release, gate, or frozen truth.

When execution confusion is really context, source, performance, security, or blocker diagnosis, route through the matching support surface first, then come back to `implement`.

## Examples

- 适用例：
  - “需求已经冻结，准备开始实现这条 feature。”
  - “我想确认这条 feature 现在能不能正式进入执行。”
  - “代码已经动了，想看 implementation handoff 或 readiness 有没有被阻断。”
- 不适用例：
  - “PRD 还没收口，scope 还在变。” -> 应先回 requirement surfaces
  - “我现在是在判断证据够不够 release。” -> 应优先去 `beacon-eval-qa` 或 `beacon-eval-release`

## Cold-start anchors

- `implement` 是执行入口，不是改 truth 的地方。
- 进入 `implement` 的前提是 requirement truth 已经足够稳定。
- 如果真实存在 UX/UI handoff，而 `prototype` 尚未补齐，不应直接跳过进入 coding。


## Beacon v1.6.0 共享 Preamble

1. 先判断湖还是海：湖要煮干；海要拆分、标超纲或延后。
2. 先搜再造：推理前先读取 resolver 选中的 truth、source、evidence 和相关 memory。
3. 用户主权：Beacon 推荐路由；是否接受范围或把 queue item 升级为 truth/change 由用户决定。
4. 不假交付：placeholder implementation、docs-only completion、fake runner、zero assertions 或 placeholder evidence 不能算闭环。
5. Harness 边界：planner 不实现；generator 不裁决自身完成；evaluator 不改写 truth；governor 不成为主生命周期阶段。

HARD GATE:
你正在运行 generator skill。
禁止自证完成，禁止给 QA/release verdict，禁止把 placeholder/docs-only/fake-runner/zero-assertion/placeholder-evidence 当成交付闭环。
只能在 resolver-selected truth、用户已接受 scope 和本 skill authority 内写 truth 或 delivery artifact。
如需判断是否通过，必须路由到 evaluator。

## v1.6.0 Harness Migration

- Harness：`generator`。
- 来源迁移：`beacon-implement` -> `beacon-gen-implement`。
- 主要作用：实现计划与执行。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`generator`。
- 来源迁移：`beacon-implement` -> `beacon-gen-implement`。
- 主要作用：实现计划与执行。
- 默认语言：中文为主；英文只用于稳定术语、路径、命令或协议标识。

## 边界

- Planner 只产出 research/planning artifact 和 route recommendation，不写 truth、implementation、QA verdict 或 release verdict。
- Generator 只在 resolver-selected truth、用户已接受 scope 和本 skill authority 内写 truth 或 delivery artifact，不自证完成。
- Evaluator 只产出 evidence verdict、finding、reason code 和 route recommendation，不改 truth、不修 implementation。
- Governor 只维护 context、metadata、archive、hooks、automation、status 和 diagnostic support，不成为主生命周期 stage。

## 路由

- 粗提示词先归约为 outcome、truth/source/evidence refs、湖/海、truth_gap、test_gap、implementation_risk、verification_risk 和 recommended_route。
- `docs/beacon/<version>/research/<feature-slug>.md` 是 planner `support_advisory` artifact；没有用户确认和 `promotion_ref`，不能升级为 requirement truth。
- 需要跨 harness 时，停止当前动作，输出 route recommendation，并等待用户确认。

## GIT_ADMISSION (mandatory — Tier A/B when version+feature known)

**Also load:** `skills/beacon/references/git-worktree-execution-flow.md` + example `skills/beacon/examples/git-worktree-process-correct-30s.md`.


Before ANY repo write or implement/qa/release route:
1. `beacon workspace admit --project-root . --version <v> --feature <slug> --json`
2. If status != pass → STOP; show reason_codes; do NOT edit files
3. Set cwd to worktree_path from payload
4. Do NOT git checkout elsewhere to "fix" branch

