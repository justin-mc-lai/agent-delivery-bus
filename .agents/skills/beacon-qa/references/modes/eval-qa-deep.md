# Mode: eval-qa-deep

> Archived from `beacon-eval-qa-deep` during 1+6 merge. This is progressive disclosure content for `beacon-qa`.

# Beacon Evaluator Qa Deep

## Overview

This skill routes deep QA support analysis through Beacon's `qa evo-deep` surface.

`qa-evo-deep` reads frozen requirement truth, QA materials, and available evidence to produce routeable findings for deeper QA exploration. It is a support surface, not a formal QA verdict surface.
The active human-facing contract is aligned to the current `v1.5.2` package-authoritative docs line. For v1.5.2 features, read resolver-selected `features/<slug>/{truth,tests,tasks,evidence}.md`; legacy triptych files are migration, rebuild, or archive inputs only.

## When to Use

- deep QA gap analysis before or after `beacon qa run`
- state-machine and business-flow scenario exploration
- scenario-combination, permission-boundary, data-consistency, and cross-surface closure review
- repeated QA round memory review when failures or blockers recur
- release supplement input that still needs formal QA/release absorption

## Boundary

- does not replace `beacon qa run`
- does not produce a formal QA pass/fail verdict
- does not produce a release verdict
- does not rewrite PRD, user-story, test-case, freeze state, QA verdict, or release verdict
- round memory is advisory influence only and never counts as current pass evidence

## Workflow / Decision Loop

- read frozen feature package truth, package coverage, and QA evidence
- identify routeable deep QA findings
- route requirement truth drift to `beacon-gen-refreeze`
- route coverage gaps to `beacon-gen-truth`
- route implementation gaps to `beacon-gen-implement`
- route evidence gaps to `beacon-eval-qa`
- route release supplement input to `beacon-eval-release` only after formal QA evidence exists

## Command

Use the fixed entrypoint:

```bash
beacon qa evo-deep "<feature>" --project . --version <version>
```

Use JSON only when automation needs the machine projection:

```bash
beacon qa evo-deep "<feature>" --project . --version <version> --json
```

## Write Topology

- detailed human-readable findings: `docs/beacon/<version>/qa/evo-deep/<feature>.md`
- machine projection: `docs/beacon/<version>/.machine/qa/evo-deep/<feature>.json`
- research: index or design-input summary only; do not duplicate the detailed QA evo-deep report there

## Common Rationalizations

- “qa-evo-deep 已经分析很深了，可以算 QA 通过。” -> 不允许；必须回到 `beacon qa run`。
- “历史轮次都过了，这轮不用重新验。” -> 不允许；round memory 只能影响场景选择。
- “发现需求不对，直接在 qa-evo-deep 里改 PRD。” -> 不允许；truth drift 必须走 `change/refreeze`。

## Red Flags

- finding 没有 affected requirement 或 route recommendation
- 把 blocked admission 写成产品 fail
- 把 release supplement input 写成 release pass
- 详细报告写进 `research/<feature>.md` 造成重复 truth

## Verification

- output path is `docs/beacon/<version>/qa/evo-deep/<feature>.md`
- machine output is evidence/read-model only
- findings use supported families: state machine, business flow, scenario combination, permission boundary, data source consistency, cross-surface closure, repeated failure
- every finding remains routeable and does not claim forbidden authority

## Evidence Produced

- QA evo-deep report
- routeable findings
- blocked admission notes
- round-memory influence notes
- release supplement input for later formal absorption

## State Updated

- `docs/beacon/<version>/.machine/qa/evo-deep/<feature>.json`

## Gate Impact

- no direct gate verdict
- can block release indirectly by exposing QA evidence gaps, truth drift, or unresolved routeable findings

## Examples

- 适用例：
  - “QA 看起来过了，但我担心状态机和业务流程叠加场景没覆盖。”
  - “同一个 blocker 多轮出现，想让 QA 深挖一下路线和记忆。”
  - “release 前需要一份深 QA findings 作为补充输入。”
- 不适用例：
  - “我要正式验收通过。” -> 应运行 `beacon-eval-qa`
  - “我要冻结新需求。” -> 应运行 `beacon-gen-refreeze`
  - “我要直接修改 test-case truth。” -> 应运行 `beacon-gen-truth`

## Cold-start anchors

- `qa-evo-deep` 是深 QA 支持分析，不是正式 QA。
- 它只产出可路由发现，不改 truth，不下 release 结论。
- 详细结果固定写入 `docs/beacon/<version>/qa/evo-deep/<feature>.md`。


## Beacon v1.6.0 共享 Preamble

1. 先判断湖还是海：湖要煮干；海要拆分、标超纲或延后。
2. 先搜再造：推理前先读取 resolver 选中的 truth、source、evidence 和相关 memory。
3. 用户主权：Beacon 推荐路由；是否接受范围或把 queue item 升级为 truth/change 由用户决定。
4. 不假交付：placeholder implementation、docs-only completion、fake runner、zero assertions 或 placeholder evidence 不能算闭环。
5. Harness 边界：planner 不实现；generator 不裁决自身完成；evaluator 不改写 truth；governor 不成为主生命周期阶段。

HARD GATE:
你正在运行 evaluator skill。
禁止改写 requirement truth，禁止修复 implementation，禁止把 verdict 和 repair 混在同一动作里。
你的唯一产出是 evidence verdict、finding、reason code、block/pass/route recommendation。
如需修复，必须路由到 generator 或 truth/change。

## v1.6.0 Harness Migration

- Harness：`evaluator`。
- 来源迁移：`beacon-qa-evo-deep` -> `beacon-eval-qa-deep`。
- 主要作用：深度 QA findings。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`evaluator`。
- 来源迁移：`beacon-qa-evo-deep` -> `beacon-eval-qa-deep`。
- 主要作用：深度 QA findings。
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
