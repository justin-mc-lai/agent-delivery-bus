# Mode: gen-refreeze

> Archived from `beacon-gen-refreeze` during 1+6 merge. This is progressive disclosure content for `beacon-truth`.

# Beacon Generator Truth Refreeze

## Overview

`beacon-gen-refreeze` is the stable human-facing route for accepted post-freeze requirement refreeze intent.

It is a narrow change-family support skill. It makes one-shot refreeze discoverable for natural language requests, then routes to the existing governed machine path:

```bash
beacon change ... --refreeze-chain
```

It does not create a second refreeze implementation.

## When to Use

- change 一键冻结
- change refreeze
- 重新冻结
- refreeze-chain
- post-freeze requirement closure
- a `deep` or support-surface finding has been accepted and must enter governed change/refreeze closure

## Boundary

- not a lifecycle stage
- not a truth source
- not a gate source
- does not write canonical PRD, user-story, or test-case truth directly
- does not execute implementation, QA, release, or direct canonical truth writes
- does not replace `beacon-gen-change`; it routes into the existing `beacon change ... --refreeze-chain` transaction

## Workflow / Decision Loop

- confirm the intent is accepted post-freeze refreeze closure, not early requirement drafting
- route the operator to `beacon change ... --refreeze-chain`
- keep the closure chain explicit: `change -> prd -> user-story -> test-case -> requirement_clarity -> freeze`
- fail closed back to `beacon-gen-change` when required truth surfaces are missing or clarity blocks
- route onward only after the refreeze transaction reports complete

## Common Rationalizations

- “有了 `beacon-gen-refreeze`，它就是一个新的 stage。” -> 不允许；它只是 change-family route surface.
- “skill 已命中，可以直接写 truth。” -> 不允许；truth still belongs to PRD, user-story, test-case, requirement clarity, and freeze.
- “refreeze 完就可以自动 implement / QA / release。” -> 不允许；it only unlocks the safe next route after the transaction completes.

## Red Flags

- operator wants to change accepted scope but no PRD/user-story/test-case refresh exists
- refreeze intent is used to bypass requirement clarity
- support findings are treated as canonical truth before adoption
- implementation resumes while `refreeze_status` is blocked

## Verification

- the requested route maps to `beacon change ... --refreeze-chain`
- the chain remains `change -> prd -> user-story -> test-case -> requirement_clarity -> freeze`
- any blocked transaction stays in `beacon-gen-change`
- no implementation, QA, release, or direct truth write is claimed by this skill

## Evidence Produced

- route explanation for the operator
- expected refreeze command
- pointer to the change refreeze transaction artifacts produced by the CLI

## State Updated

- no state is updated by the skill itself
- state changes only occur through `beacon change ... --refreeze-chain`

## Gate Impact

- no direct gate verdict
- can unblock downstream `implement`, `qa`, or `release` only after the existing refreeze transaction completes
- blocked or partial refreeze remains a `beacon-gen-change` problem

## Operator Rule

Prefer:

```bash
beacon change "<feature>" --project . --version v1.4.10 --reason "<accepted change>" --refreeze-chain
```

If the feature is already reopened, use the same route. The transaction must close the active refreshed requirement package before execution resumes.

## Examples

- 适用例：
  - “change 后帮我一键重新冻结。”
  - “deep 的这个 finding 已经接受，走 change/refreeze 闭环。”
  - “这个 feature 的 PRD/user-story/test-case 都改好了，现在 refreeze。”
- 不适用例：
  - “我还在想需求怎么写。” -> 应先去 `beacon-gen-truth` / `beacon-gen-truth` / `beacon-gen-truth`
  - “我想直接跑实现。” -> 应先确认 refreeze transaction 已 complete，再去 `beacon-gen-implement`

## Cold-start Anchors

- `beacon-gen-refreeze` 是稳定人类入口，不是新主链段。
- 唯一机器执行路径是 `beacon change ... --refreeze-chain`。
- refreeze 只重新闭合 requirement truth；不执行 implement、QA 或 release。


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

## GIT_ADMISSION (mandatory — truth authoring on canonical branch)

Before ANY truth/change/refreeze/freeze write when version is known:
1. Resolve `truth_canonical` from project governance (default: `main` or `master`)
2. Verify `git rev-parse --abbrev-ref HEAD` equals `truth_canonical`
3. If mismatch → STOP with `reason_code=truth_canonical_branch_mismatch`; do NOT edit requirement truth
4. Commit refreeze/freeze artifacts on `truth_canonical` before loop-goal may route to implement
5. Do NOT use feature worktree for truth authoring — use the canonical branch checkout only
6. Shared process: `skills/beacon/references/git-worktree-execution-flow.md`

## v1.6.0 Harness Migration

- Harness：`generator`。
- 来源迁移：`beacon-change-refreeze` -> `beacon-gen-refreeze`。
- 主要作用：受治理的需求重冻结。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`generator`。
- 来源迁移：`beacon-change-refreeze` -> `beacon-gen-refreeze`。
- 主要作用：受治理的需求重冻结。
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
