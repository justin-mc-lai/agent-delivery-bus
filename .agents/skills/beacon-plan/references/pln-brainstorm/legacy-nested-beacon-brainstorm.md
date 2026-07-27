---
name: beacon-brainstorm
description: Use when the user wants Beacon's built-in thinking sidecar to continue requirement brainstorming, converge gaps from docs/beacon/<version>, or explore project evolution without rewriting frozen truth or entering implementation.
---

# Beacon Brainstorm

## Overview

Use this skill as Beacon's read-only convergence surface.

It is not a new main lifecycle stage. Its job is to challenge framing, expose gaps, and route the work back to truth or execution surfaces without silently promoting its own output into formal truth.

## When to Use

- continuing requirement brainstorming from `docs/beacon/<version>/`
- finding missing questions before editing `truth.define`, `truth.acceptance`, or `truth.coverage` materials
- exploring project evolution before promoting it into a formal Beacon change
- checking implementation prompts against frozen requirement evidence before `beacon-implement`

## Boundary

- not a truth source
- not a gate source
- not a new lifecycle stage
- does not rewrite `features/<slug>/truth.md`, `features/<slug>/tests.md`, `prototype`, or `.machine/`
- if accepted scope changes, route to `beacon-change`

## Workflow / Decision Loop

- reduce the prompt to user outcome, evidence, and closure constraints
- challenge framing, assumptions, route, and evidence gaps
- decide whether the result should go back to `beacon-truth`, `beacon-change`, or `beacon-implement`
- for truth work, prefer `truth.define -> truth.acceptance -> truth.coverage`; compatibility aliases are `beacon-prd`, `beacon-user-story`, and `beacon-test-case`
- stop at convergence output; do not self-promote into frozen truth

## Common Rationalizations

- “头暴结论已经够清楚，可以直接当正式 requirement。” -> 不允许；要回 truth surface
- “只是补几个 acceptance 点，不用走主链。” -> 不允许；如果改承诺面，就要回 `change` / `user-story` / `test-case`
- “brainstorm 顺手写进 `.machine` 或正式 docs 就行。” -> 不允许；只写它自己的分析面

## Red Flags

- 把 brainstorm 输出当成 freeze verdict
- 还在 challenge scope，却已经试图越级进入 `implement` 或 `release`
- requirement gap 明显存在，但输出没有给出回路由

## Verification

- 输出明确指向 truth、execution 或 gate surface 的下一跳
- 对复杂 feature，会帮助生成 research / business flow / state-machine 等人类可读闭环材料
- 不会直接把 brainstorm 结果写成正式 requirement truth

## Evidence Produced

- convergence analysis
- gap list and focus areas
- recommended route and supplement direction

## State Updated

- brainstorm artifact only
- optional guidance/read model projection

## Gate Impact

- no direct gate verdict
- may block execution by routing back to `change`, `user-story`, or `test-case` when drift or gaps are found

## Thinking Protocol

Always apply:

- First principles: reduce the prompt to user outcome, evidence, and closure constraints.
- Socratic questioning: ask what must be true, what could falsify it, and which frozen evidence proves it.
- Occam's razor: remove redundant mechanisms, duplicate docs, and broad speculation.

## Operator rule

Prefer the installed CLI (works in any project directory once `beacon` is on `PATH`; does **not** depend on `skills/beacon/scripts/` being vendored):

```bash
beacon brainstorm run "<prompt>" \
  --project-root . \
  --version auto \
  --feature "<feature-or-contract-slug>" \
  --mode requirement-convergence \
  --json
```

Runtime note: current CLI line reports **`Beacon CLI v1.4.3`**. Adjust `--version` / docs paths if your project pins a different docs line.

Optional compatibility (Beacon repo or vendored skill pack only):

```bash
bash skills/beacon/scripts/run_beacon.sh brainstorm run "<prompt>" \
  --project-root . \
  --version auto \
  --mode requirement-convergence \
  --json
```

默认规则：

- Beacon runtime v1.4.9+ 的 feature / contract 头暴默认写入 `docs/beacon/<version>/research/<feature-slug>.md`
- feature / contract 名称必须通过 `--feature` 显式传入；如果 prompt 中包含 `*-contract` 等稳定 slug，runtime 可推断，但不要依赖长句标题命名
- 无 feature 的自由头暴才允许写入 `docs/brainstorms/`
- 外部项目或外部仓库的无 feature 分析会自动隔离到 `docs/brainstorms/external/`
- 如果需要写到明确目标位置，始终显式传 `--output`

Supported modes:

- `requirement-convergence`
- `project-evolution`
- `implementation-readiness`
- `scope-simplification`

Legacy aliases:

- `frozen-truth-delta` -> `implementation-readiness`
- `evolution-exploration` -> `project-evolution`
- `redundancy-pruning` -> `scope-simplification`

## Human Closure Contract

When the feature is materially complex, brainstorm must help produce a human-readable research closure package before freeze:

- `docs/beacon/<version>/research/<feature-slug>.md`
- architecture explanation
- business flow explanation
- state-machine explanation when states or transitions matter
- decision-priority explanation when conflicting rules must be resolved

This package is the upstream human-readable contract for:

- `truth.define`
- `truth.acceptance`
- `truth.coverage`
- `requirement_clarity`
- `freeze`

Do not treat this as decoration. These materials are upstream inputs to `truth.define -> truth.acceptance -> truth.coverage -> requirement_clarity -> freeze`.
Compatibility mapping: `prd -> user-story -> test-case -> requirement_clarity -> freeze`.

## Examples

- 适用例：
  - “这个 PRD 看起来方向对，但我怀疑还漏了关键问题，先帮我收敛 gap。”
  - “需求不想立刻改 truth，先 challenge 一下假设、范围和证据缺口。”
  - “实现前想确认 frozen requirement 有没有明显洞，但不想直接进入 execution。”
- 不适用例：
  - “我已经确定要改 frozen scope，直接写回正式变更。” -> 应路由到 `beacon-change`
  - “需求已经稳定，现在要开始做实现。” -> 应路由到 `beacon-implement`

## Cold-start anchors

- `brainstorm` 是 read-only convergence surface，不是新 stage。
- 它的主要价值是 challenge framing、收敛 gap、解释 route，不是直接产出 truth。
- 如果结论会改正式承诺面，就必须回 `change`；如果只是确认 scope 稳定，就回主链继续执行。
