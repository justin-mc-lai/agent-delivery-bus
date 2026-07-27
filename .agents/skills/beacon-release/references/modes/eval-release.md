# Mode: eval-release

> Archived from `beacon-eval-release` during 1+6 merge. This is progressive disclosure content for `beacon-release`.

# Beacon Evaluator Release

## Overview

This skill routes final ship / release-readiness work through Beacon's `release` surface.

Release consumes layered QA proof and version-surface governance, then makes the go/no-go judgment. It does not invent acceptance truth or finish implementation.
The active human-facing contract is aligned to the current `v1.5.2` package-authoritative docs line.
Compatibility reference: v1.4.7 docs line `docs/beacon/v1.4.7/`.
For v1.5.2 features, release judgment must bind to resolver-selected `features/<slug>/{truth,tests,tasks,evidence}.md`, layered QA proof, and version governance. Legacy truth files may be migration or archive inputs only.

## When to Use

- release readiness
- final go / no-go checks
- last-mile delivery confidence
- checking whether implementation and QA are sufficient to ship
- release proof chain validation

## Boundary

- `release` is not a place to finish implementation
- it does not rewrite frozen truth
- it does not bypass version-surface governance

## Workflow / Decision Loop

- continue in `release` only when implementation and QA have already produced enough evidence
- block on missing QA layers, unresolved truth drift, version-surface drift, or unresolved release blockers
- prefer explicit no-go over soft optimism when evidence is incomplete
- route backward when the missing condition belongs to `qa`, `implement`, or `change`

## Common Rationalizations

- “dashboard 看着都绿了，应该能发。” -> 不允许；要看证据链
- “real-project-validation 过了，其他层可以省。” -> 不允许；它是高权重层，不是唯一层
- “版本号差一点没关系，先发再说。” -> 不允许；version drift 必须阻断

## Red Flags

- QA 证据链未闭合，却已经寻求 go verdict
- docs/package/runtime version surfaces 不一致
- release 试图替 requirement truth 或 implementation gap 收尾

## Verification

- `skill-contract / scenario-suite / trace-scorecard / real-project-proof` 四层证明可追溯
- no-go blocker 和 reason code 明确
- docs/package/runtime version surfaces 一致

## Evidence Produced

- release readiness verdict
- final blocker list
- release-facing proof summary

## State Updated

- release scorecard / release-facing gate state

## Gate Impact

- direct go / no-go gate
- may block ship and route back to `qa`, `implement`, or `change`

## Inputs Contract

- `project-root`
- `version`
- `feature` or version release target
- layered QA proof
- release gate status
- version surface governance status

## Decision Protocol

- Continue in `release` only when implementation and QA have already produced sufficient evidence.
- Block on missing QA layers, unresolved truth drift, version-surface drift, or unresolved release gate blockers.
- Prefer explicit no-go verdicts over soft optimism when evidence is incomplete.
- Treat `real-project-validation` as highest proof, but still require the other layers.

## Anti-Inertia Notes

- `release` is not a place to finish implementation or invent acceptance truth.
- A green-looking dashboard without evidence chain is not release closure.
- Version drift across docs, package, and runtime surfaces must block ship.
- `release` cannot rewrite frozen truth or override gate truth by itself.

## Learning Notes

- Reuse prior release blockers as warning signals for current no-go checks.
- Learning may influence inspection order, but it does not override gate verdicts.
- Stale release learning should trigger re-check, not silent acceptance.

## Backstop CLI

- `beacon release ...`
- `beacon qa ...` when proof is incomplete
- `beacon doctor verify-version-surfaces --project-root . --strict`

## v1.5.2 Package-Authoritative Read Order

For v1.5.2+ features, read the resolver output before judging release readiness:

```bash
beacon truth-map resolve <feature> --project . --version v1.5.2 --json
```

Release may consume generated QA/release reports as evidence, but requirement truth must come from the resolved Markdown feature package. Generated `.machine` JSON, manifests, and gate reports are generated state, not requirement truth, and not requirement authority.

## Supporting References

Use these lightweight references when the release conversation is blocked by human-readable ambiguity rather than missing commands:

- `../references/release-blocker-taxonomy.md`
- `../references/qa-layer-checklist.md`
- `../references/support-surface-routing-cheatsheet.md`

These references help classify blockers and route fallback correctly, but they do not replace release evidence or version-surface governance.

## Examples

- 适用例：
  - “这条 feature 的实现和 QA 都做完了，现在要判断能不能放行。”
  - “我要看最后的 release blocker 还剩什么。”
  - “我怀疑 dashboard 是绿的，但证据链未必真的闭合。”
- 不适用例：
  - “实现还没完成，只是想继续修功能。” -> 应回 `beacon-gen-implement`
  - “acceptance proof 还没跑完整。” -> 应优先去 `beacon-eval-qa`

## Cold-start anchors

- `release` 判断的是 go / no-go，不是补实现或补真相的地方。
- 绿色表象不等于 release closure，证据链和版本治理缺口都必须阻断。
- `real-project-validation` 是高权重证明，但不是唯一证明。


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

## GIT_ADMISSION (mandatory — Tier A/B when version+feature known)

Before ANY repo write or implement/qa/release route:
1. `beacon workspace admit --project-root . --version <v> --feature <slug> --json`
2. If status != pass → STOP; show reason_codes; do NOT edit files
3. Set cwd to worktree_path from payload
4. Do NOT git checkout elsewhere to "fix" branch
5. When admission is soft-skipped (`require_workspace_admission=false`), still verify `current_branch == resolved target_branch` before repo writes (R5 UD-024)
6. (implement only) Before writing implementation, merge `truth_canonical` into the development branch so implementation bases on canonical frozen truth

## v1.6.0 Harness Migration

- Harness：`evaluator`。
- 来源迁移：`beacon-release` -> `beacon-eval-release`。
- 主要作用：发布就绪判断。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`evaluator`。
- 来源迁移：`beacon-release` -> `beacon-eval-release`。
- 主要作用：发布就绪判断。
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
