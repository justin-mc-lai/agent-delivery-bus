# Mode: gen-design-init

> Archived from `beacon-gen-design-init` during 1+6 merge. This is progressive disclosure content for `beacon-design`.

# Beacon Generator Design Bootstrap

## Overview

This is a host-visible child skill for the Beacon design `bootstrap` route.

It checks installed skill, mirrored source tree, or bootstrap route availability for upstream design capabilities. It is capability-admission-only.

## When to Use

- verify upstream design skills are installed
- verify mirrored open-source design sources exist
- inspect bootstrap route availability
- diagnose missing design capability prerequisites

## Boundary

- not a truth source
- not a gate source
- not a new lifecycle stage
- not a release verdict
- capability-admission-only
- support-only bootstrap check
- does not replace `beacon-gen-design`
- does not prove design quality or release readiness

## Workflow / Decision Loop

- run or reason against `beacon design route bootstrap "<feature>" --project . --version auto --json`
- use `beacon design bootstrap --project . --version auto --json` for capability admission details
- report installed, mirrored, or bootstrap availability only
- route accepted prototype input to `beacon prototype adapt`
- route promise-changing facts to `beacon change ... --refreeze-chain`
- hand off to `beacon implement` only after accepted facts are truth-bound

## Verification

- route payload reports `route=beacon-gen-design-init`, `intent=bootstrap`, and `upstream=upstream-compatibility`
- route payload marks `capability_admission_only=true`, `support_only=true`, `truth_source=false`, `gate_source=false`, and `lifecycle_stage=false`
- bootstrap payload marks `release_verdict=false`

## Operator Rule

Prefer:

```bash
beacon design route bootstrap "<feature>" --project . --version auto --json
beacon design bootstrap --project . --version auto --json
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
- 来源迁移：`beacon-design-bootstrap` -> `beacon-gen-design-init`。
- 主要作用：设计能力准入。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`generator`。
- 来源迁移：`beacon-design-bootstrap` -> `beacon-gen-design-init`。
- 主要作用：设计能力准入。
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
