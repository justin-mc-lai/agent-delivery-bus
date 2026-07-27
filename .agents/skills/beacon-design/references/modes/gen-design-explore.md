# Mode: gen-design-explore

> Archived from `beacon-gen-design-explore` during 1+6 merge. This is progressive disclosure content for `beacon-design`.

# Beacon Generator Design Explore

## Overview

This is a host-visible child skill for the Beacon design `explore` route.

It preserves the `vibe-to-ui` exploration capability while keeping output as design direction candidates until accepted facts are bound back to Beacon truth.

## When to Use

- UI direction exploration
- concept direction comparison
- information hierarchy sketching
- layout model exploration
- interaction model candidates

## Boundary

- not a truth source
- not a gate source
- not a new lifecycle stage
- not a release verdict
- support-only design exploration
- does not replace `beacon-gen-design`
- does not treat `vibe-to-ui` output as Beacon truth

## Workflow / Decision Loop

- run or reason against `beacon design route explore "<feature>" --project . --version auto --json`
- produce concept directions, hierarchy options, layout models, and interaction candidates
- route accepted prototype input to `beacon prototype adapt`
- route promise-changing facts to `beacon change ... --refreeze-chain`
- hand off to `beacon implement` only after accepted facts are truth-bound

## Verification

- route payload reports `route=beacon-gen-design-explore`, `intent=explore`, and `upstream=vibe-to-ui`
- route payload marks `support_only=true`, `truth_source=false`, `gate_source=false`, and `lifecycle_stage=false`
- accepted design facts have a truth-binding route before implementation

## Operator Rule

Prefer:

```bash
beacon design route explore "<feature>" --project . --version auto --json
```

If a design result is accepted:

```bash
beacon prototype adapt "<feature>" --project . --version auto --design-result-ref <accepted-design-result>
```

If design facts change product promise, behavior, AC, or QA coverage:

```bash
beacon change "<feature>" --project . --version auto --reason "accepted design truth changed requirement promise" --refreeze-chain
```


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
- 来源迁移：`beacon-design-explore` -> `beacon-gen-design-explore`。
- 主要作用：UI 方向探索。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`generator`。
- 来源迁移：`beacon-design-explore` -> `beacon-gen-design-explore`。
- 主要作用：UI 方向探索。
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
