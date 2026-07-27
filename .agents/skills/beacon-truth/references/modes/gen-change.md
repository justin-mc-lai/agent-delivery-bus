# Mode: gen-change

> Archived from `beacon-gen-change` during 1+6 merge. This is progressive disclosure content for `beacon-truth`.

# Beacon Generator Truth Change

## Overview

`beacon-gen-change` is the host-visible router for post-freeze requirement revisions.

It does not create a new primary stage. It reopens frozen truth in a governed way, refreshes the requirement package, and routes the work back into the existing mainline.

## When to Use

- frozen feature scope must change
- a hotfix or repair needs requirement truth updated before implementation continues
- requirement supplement is needed after freeze
- a support surface concluded that accepted scope, promise, or coverage must change

## Boundary

- `beacon-gen-change` is not a parallel workflow
- it does not bypass `prd -> user-story -> test-case`
- it does not directly claim implementation, QA, or release completion

## Workflow / Decision Loop

- confirm that current truth is already frozen enough to require a change loop
- reopen the affected requirement truth and revision context
- refresh `prd`, `user-story`, and `test-case` as needed
- re-freeze before returning to `implement`, `qa`, or `release`
- when the accepted change already has refreshed truth surfaces, use `beacon change ... --refreeze-chain` for the governed one-shot `change -> prd -> user-story -> test-case -> requirement_clarity -> freeze` closure

## Common Rationalizations

- “只是一个小修，不用回 formal truth。” -> 不允许；如果改 promise/acceptance，就必须进 `change`
- “support surface 已经下结论，可以直接改实现。” -> 不允许；support surface 只能 route，不直接写 truth
- “先修代码，最后再补文档。” -> 不允许；`change` 先于实现修改

## Red Flags

- frozen truth 已经漂移，但还试图直接进入 `implement`
- 把 `change` 当成内部脚本步骤，而不是 requirement reopen surface
- 修改 acceptance 或 scope，却没有显式 re-freeze 意图

## Verification

- 修改目标确实属于 frozen truth revision，而不是未冻结的 requirement clarification
- 变更会回写到正式 requirement package，而不是只停留在分析或实现面
- 后续路由会回到 `implement`、`qa` 或 `release`，而不是停在 reopen 中间态

## Evidence Produced

- change intent and reopen rationale
- affected requirement surfaces
- re-freeze expectation and downstream route

## State Updated

- revision/reopen state
- requirement package status for the affected feature

## Gate Impact

- blocks `beacon-gen-implement` / `beacon-eval-qa` / `beacon-eval-release` from trusting stale frozen truth
- re-enables execution only after requirement truth is refreshed and re-frozen

## Operator rule

Prefer:

- `beacon change ...`
- `beacon change ... --refreeze-chain` when closing an accepted post-freeze change through the full requirement chain in one governed transaction
- `beacon-gen-refreeze` when the user expresses natural-language one-shot refreeze intent and needs a stable route to the same transaction

Treat `beacon-gen-change` as a router, not a new public Beacon stage.
Do not introduce a new Beacon primary command such as `change`, `revision`, or `hotfix`.

## Examples

- 适用例：
  - “这条 feature 已经 freeze 了，但 scope 现在必须改。”
  - “这是 hotfix，但在动实现前必须先更新 requirement truth。”
- 不适用例：
  - “我还没冻结，只是在继续澄清需求。” -> 应先去 requirement surfaces


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
- 来源迁移：`beacon-change` -> `beacon-gen-change`。
- 主要作用：受治理的需求真相变更。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`generator`。
- 来源迁移：`beacon-change` -> `beacon-gen-change`。
- 主要作用：受治理的需求真相变更。
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

## GoalRun main axis (UD-AXIS — v1.6.7+)

If the change touches AC/promise/stages/parallel/longrun/entry/domain FSM:

1. Update feature truth as usual
2. **Regenerate** program main axis: `beacon goal axis generate -v <ver> --program <program>`
3. **Validate** fail-closed: `beacon goal axis validate ...`
4. Then `beacon change ... --refreeze-chain`

Do not implement against a stale main axis.


## Truth human-readable (v1.6.7+ / v2)

When writing or changing feature `truth.md`:

1. Include L0 section: `## 人话` (zh) or `## Plain language` (en); bilingual needs both.
2. Keep L1 contract (AC / FSM) as gate authority — never drop AC IDs to "sound human".
3. Follow norms:
   - `skills/beacon/references/truth-humanizer/truth_prose.zh.md`
   - `skills/beacon/references/truth-humanizer/truth_prose.en.md`
4. Examples: `skills/beacon/examples/truth-human-readable.zh.md` / `.en.md`
5. Missing L0 → reason_code `truth_human_l0_missing` (freeze fail-closed after gate is wired).

Lakes: `beacon-truth-human-readable-i18n-v167`, `beacon-v2-truth-human-readable-i18n` (on GoalRun main axis).
