# Mode: gen-design

> Archived from `beacon-gen-design` during 1+6 merge. This is progressive disclosure content for `beacon-design`.

# Beacon Generator Design Support

## Overview

`beacon-gen-design` is Beacon's support-only design route surface.

It preserves the useful intent split from `ui-ux-dev` while adding Beacon truth boundaries:

- `simplify` is Beacon-native with no upstream dependency.

- `explore` and `extract` preserve `vibe-to-ui`.
- `system` preserves `ui-ux-pro-max`.
- `review` and `polish` preserve `impeccable`.
- `library` preserves `awesome-design-md` as reference-only.
- `bootstrap` checks upstream installed/mirror/bootstrap availability.

## When to Use

- A feature needs a first-principles simplicity audit before design exploration.
- UI direction is unclear and needs exploration.
- A screenshot or reference needs structure extraction.
- A version-wide component system or design baseline is needed.
- Existing UI needs UX review or ship-readiness polish.
- The operator needs curated design references.
- A machine needs design upstream capability admission checks.

## Boundary

- not a truth source
- not a gate source
- not a new lifecycle stage
- does not replace `beacon-gen-change`
- does not replace `beacon-gen-prototype`
- does not treat upstream design skills as Beacon truth
- does not treat `awesome-design-md` as an execution skill

## Workflow / Decision Loop

- normalize the request into `explore`, `system`, `review`, `polish`, `extract`, `library`, `bootstrap`, or `simplify`
- use `beacon design route <intent> "<feature>" --project . --version auto --json` to resolve the deterministic contract
- draft design truth candidates under Beacon version-wide design surfaces when needed
- route accepted UI/UX facts to `beacon prototype adapt`
- route promise-changing design facts to `beacon change ... --refreeze-chain`
- hand off to `beacon implement` only after accepted design facts are truth-bound

## Common Rationalizations

- “design skill 输出了，所以就是 truth。” -> 不允许；accepted facts still need Beacon truth binding.
- “参考库里有案例，可以直接当项目设计规范。” -> 不允许；`awesome-design-md` is reference-only.
- “bootstrap 通过了，所以 release 可以过。” -> 不允许；bootstrap is capability admission only.
- “review/polish 是小建议，不用看 PRD/user-story/test-case。” -> 不允许；promise-changing facts route to change/refreeze.

## Red Flags

- accepted UI/UX facts exist but no prototype adapter binding exists
- `awesome-design-md` is used as an execution authority
- upstream skill output is copied into implementation without Beacon truth refs
- design support becomes a mandatory lifecycle stage for non-UI features

## Verification

- `beacon design contract --project . --version auto --json` lists all seven routes
- route outputs mark `support_only=true`, `truth_source=false`, `gate_source=false`, and `lifecycle_stage=false`
- `library` is reference-only
- `bootstrap` is capability-admission-only
- accepted design results can be bound through `beacon prototype adapt`

## Evidence Produced

- route contract payload
- version-wide design truth draft paths
- capability admission payload
- support-only design plan

## State Updated

- no lifecycle state directly
- optional version-wide design truth drafts when the CLI is explicitly called with `design truth --write`
- optional prototype adapter binding only through `beacon prototype adapt`

## Gate Impact

- no direct release gate verdict
- can block downstream execution only when accepted design facts remain unbound by Beacon truth

## Operator Rule

Prefer unattended visual OS when no human design decision exists:

```bash
beacon design baseline --project . --version auto --surface auto --write --json
```

If `DESIGN.md` already exists, baseline writes a fine-tune proposal (non-destructive). Secondary polish still uses design routes, then binds through prototype adapt / change. See `skills/beacon/examples/design-md-complete.md` and multi-end copy packs under `skills/beacon/examples/design-surfaces/` (pc-console / web / h5 / client).

Prefer:

```bash
beacon design route review "<feature>" --project . --version auto --json
beacon design contract "<feature>" --project . --version auto --json
beacon design truth --project . --version auto --write --json
```

If a design result is accepted:

```bash
beacon prototype adapt "<feature>" --project . --version auto --design-result-ref <accepted-design-result>
```

If design facts change product promise, behavior, AC, or QA coverage:

```bash
beacon change "<feature>" --project . --version auto --reason "accepted design truth changed requirement promise" --refreeze-chain
```

## Examples

- 适用例：
  - “帮我给这个功能出 UI 方向，但要能进 Beacon truth。”
  - “这个界面做一次 UX review，结论如果接受要能写回。”
  - “检查一下 design 上游能力在这台机器是否可用。”
- 不适用例：
  - “我要直接发布。” -> 应去 `beacon-eval-release`
  - “我要让参考库直接决定项目设计规范。” -> 不允许


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
- 来源迁移：`beacon-design` -> `beacon-gen-design`。
- 主要作用：设计支持总入口。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`generator`。
- 来源迁移：`beacon-design` -> `beacon-gen-design`。
- 主要作用：设计支持总入口。
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
