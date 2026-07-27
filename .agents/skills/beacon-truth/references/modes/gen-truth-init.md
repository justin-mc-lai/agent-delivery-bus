# Mode: gen-truth-init

> Archived from `beacon-gen-truth-init` during 1+6 merge. This is progressive disclosure content for `beacon-truth`.

# Beacon Generator Truth Bootstrap

This optional skill reduces requirement startup friction while keeping Beacon safe-by-default.

## Use for

- one-click requirement bootstrap
- creating `prd + user-story + test-case` docs in one run
- low-risk initialization with strict guardrails

## Safety defaults

- strict context verify (`doctor verify-context --strict`)
- dry-run unless explicitly `--apply`
- no-overwrite by default
- lock + state artifact + rollback manifest
- bootstrap only creates the first requirement package shell; freeze still depends on `research -> prd -> user-story -> test-case -> requirement_clarity`

## Operator rule

Prefer the installed CLI (hidden command surface):

- `beacon prd-bootstrap run "<feature>" --desc "<需求描述>" --version <version> --dry-run --json`
- `beacon prd-bootstrap run "<feature>" --desc "<需求描述>" --version <version> --apply --allow-overwrite`

Optional repo-local shell wrappers under `skills/beacon/scripts/` remain supported only when that skill pack is vendored into the repo.

Do not treat this as Beacon mainline replacement.

## Examples

- 适用例：
  - “这个项目还没有 requirement 材料，我想先安全地 bootstrap 一套初始 PRD/user-story/test-case。”
  - “我要先 dry-run 看看 Beacon 会生成什么，再决定是否 apply。”
- 不适用例：
  - “这条 feature 已经有冻结 truth，只是要修改它。” -> 应优先去 `beacon-gen-change`

## Cold-start anchors

- `prd-bootstrap` 解决的是 requirement 冷启动，不是主链替代。
- 默认应先安全、可验证、可回滚，再考虑 apply。
- 一旦项目已经进入正式 truth 演化，后续应回到 `prd / user-story / test-case / change` 主链。
- 如果 feature 属于复杂场景，bootstrap 之后仍必须补齐 research / architecture / business flow / state-machine 等人类闭环材料，才能进入 freeze。


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
- 来源迁移：`beacon-prd-bootstrap` -> `beacon-gen-truth-init`。
- 主要作用：需求真相包初始化。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`generator`。
- 来源迁移：`beacon-prd-bootstrap` -> `beacon-gen-truth-init`。
- 主要作用：需求真相包初始化。
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
