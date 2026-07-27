# Mode: eval-agent

> Archived from `beacon-eval-agent` during 1+6 merge. This is progressive disclosure content for `beacon-qa`.

# Beacon Evaluator Agent Eval

## Overview

This skill routes Agent Eval work through Beacon's public `beacon eval` surface.

Agent Eval produces **runtime evidence only** — dimension scores, composite scorecard, benchmark artifacts, and live skill matrix traces. It does **not** replace `beacon qa run` or `beacon release check`.

## When to Use

- unified Agent Eval catalog, run, scorecard, or benchmark
- full host-visible skill live matrix inspection (v1.6.1)
- eval score interpretation with explicit authority boundaries
- handoff to formal QA/release evaluators after runtime evidence is ready

## Boundary

- `beacon eval` is not permission to rewrite requirement truth
- it does not repair implementation
- simulated or replay scores must not be treated as formal QA or release verdict
- live runner requires explicit admission JSON

## Workflow / Decision Loop

- materialize catalog: `beacon eval catalog --materialize --version v1.6.1 --json`
- regression replay: `beacon eval run --split regression-core --runner replay --version v1.6.1 --json`
- scorecard: `beacon eval scorecard --version v1.6.1 --json`
- benchmark: `beacon eval benchmark --version v1.6.1 --json`
- route to `beacon-eval-qa` for formal feature QA
- route to `beacon-eval-release` for release readiness

## Backstop CLI

```bash
beacon eval catalog|run|scorecard|benchmark --project-root . --version v1.6.1 --json
```

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

## Agent Eval 扩展边界

- 禁止给出 release verdict 或 formal QA pass；Agent Eval scorecard 仅为 `runtime_evidence_only`。
- 如需 formal QA/release，必须路由到 `beacon-eval-qa` / `beacon-eval-release`。

## 职责

- Harness：`evaluator`。
- 主要作用：Agent Eval 分数面路由与 runtime evidence 解读。
- 默认语言：中文为主；英文只用于稳定术语、路径、命令或协议标识。

## 边界

- Evaluator 只产出 evidence verdict、finding、reason code 和 route recommendation，不改 truth、不修 implementation。
- Agent Eval composite 分数不得冒充 formal QA/release authority。

## 路由

- formal feature QA → `beacon-eval-qa`
- release readiness → `beacon-eval-release`
- implementation gaps → `beacon-gen-implement`
- truth drift → `beacon-gen-change`
