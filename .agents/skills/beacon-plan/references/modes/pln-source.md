# Mode: pln-source

> Archived from `beacon-pln-source` during 1+6 merge. This is progressive disclosure content for `beacon-plan`.

# Beacon Planner Source Driven

## Overview

This is a support surface for source-anchored execution and review.

It keeps work tied to actual source truth, code, interfaces, tests, and evidence rather than letting the workflow drift into speculative restatement. It is not itself a truth author or a release gate.

## When to Use

- implementation should stay tightly anchored to existing code or interfaces
- review should prefer actual evidence over paraphrased intent
- the user wants stronger source-driven discipline before `beacon-gen-implement` or `beacon-eval-qa`
- there is risk of the model inventing behavior that source materials do not support

## Boundary

- not a truth source
- not a gate source
- not a new lifecycle stage
- does not redefine canonical requirement truth
- if frozen promise or acceptance must change, route to `beacon-gen-change`

## Workflow / Decision Loop

- identify the highest-authority source material for the current task
- keep claims anchored to requirement docs, code, interfaces, tests, or runtime evidence
- challenge any reasoning that outruns available source support
- route back to truth or execution surfaces with explicit source-backed guidance

## Common Rationalizations

- “大概意思差不多，直接按常识补齐。” -> 不允许；先看 source 是否支持
- “source-driven 太慢，不如直接发挥。” -> 不允许；无锚推断更容易漂移
- “既然 source 不完整，就顺手把 acceptance 也改了。” -> 不允许；改 truth 要回 `change`

## Red Flags

- 关键结论没有 source anchor
- 代码或接口已经给出反例，但 reasoning 仍继续自洽
- 把 source-driven review 错当成 formal QA/release verdict

## Verification

- 输出能指出本轮判断依赖的主要 source
- 没有 source 支撑的推断被明确降级为假设或疑点
- 若 source 与 frozen truth 冲突，会显式 route back

## Evidence Produced

- source anchor summary
- unsupported-claim warnings
- route recommendation for execution, truth repair, or QA follow-up

## State Updated

- none by default
- optional support/read model projection only

## Gate Impact

- no direct gate verdict
- can block blind continuation by exposing claims that outrun source support

## Examples

- 适用例：
  - “这轮实现必须强锚定现有代码和接口，不要让解释跑到 source 前面。”
  - “我想先看这条判断有没有被 requirement、代码、测试或证据真正支撑。”
- 不适用例：
  - “我要正式给 release verdict。” -> 应去 `beacon-eval-release`
  - “我要正式补 frozen requirement truth。” -> 应去 `beacon-gen-change`

## Cold-start anchors

- `source-driven` 的核心不是多读材料，而是提高 claims 的锚定强度。
- 它更像执行前和评审时的防漂移支持层。
- source 不足时，可以暴露风险，但不能擅自改 truth。


## Beacon v1.6.0 共享 Preamble

1. 先判断湖还是海：湖要煮干；海要拆分、标超纲或延后。
2. 先搜再造：推理前先读取 resolver 选中的 truth、source、evidence 和相关 memory。
3. 用户主权：Beacon 推荐路由；是否接受范围或把 queue item 升级为 truth/change 由用户决定。
4. 不假交付：placeholder implementation、docs-only completion、fake runner、zero assertions 或 placeholder evidence 不能算闭环。
5. Harness 边界：planner 不实现；generator 不裁决自身完成；evaluator 不改写 truth；governor 不成为主生命周期阶段。

HARD GATE:
你正在运行 planner skill。
禁止调用 implementation/generator/evaluator/release 类 skill。
禁止写代码、搭脚手架、修改 package truth、修改 `.machine/`、给 QA/release verdict。
你的唯一产出是 research/planning artifact、gap/risk 分析、湖/海判断和 route recommendation。
如需进入 truth/change/refreeze/generate/evaluate，必须停止并路由，等待用户确认。

## GIT_ADMISSION (mandatory — Tier A/B when version+feature known)

Before ANY repo write or implement/qa/release route:
1. `beacon workspace admit --project-root . --version <v> --feature <slug> --json`
2. If status != pass → STOP; show reason_codes; do NOT edit files
3. Set cwd to worktree_path from payload
4. Do NOT git checkout elsewhere to "fix" branch
5. When admission is soft-skipped (`require_workspace_admission=false`), still verify `current_branch == resolved target_branch` before repo writes (R5 UD-024)
6. (implement only) Before writing implementation, merge `truth_canonical` into the development branch so implementation bases on canonical frozen truth

## v1.6.0 Harness Migration

- Harness：`planner`。
- 来源迁移：`beacon-source-driven` -> `beacon-pln-source`。
- 主要作用：源码和证据锚定分析。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`planner`。
- 来源迁移：`beacon-source-driven` -> `beacon-pln-source`。
- 主要作用：源码和证据锚定分析。
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
