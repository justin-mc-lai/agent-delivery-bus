# Mode: pln-program-interactive

> Archived from `beacon-pln-program-interactive` during 1+6 merge. This is progressive disclosure content for `beacon-plan`.

# Beacon Planner Program — Interactive Mode

## Overview

Interactive mode front-loads **brainstorm convergence** before P1–P3. It preserves full program parity matrix while narrowing Alignment Surface per phase. For需求梳理、多轮探讨对齐、MVP vs 全景并存.

## When To Use (delegation only)

- `plan_mode=interactive` from `beacon-pln-program`
- user asks for 头脑风暴、探讨、对齐、先想清楚
- classifier confidence low for auto
- no trustworthy source URL yet

## Boundary

- same as umbrella; additionally must use `beacon-pln-brainstorm` for BR rounds
- brainstorm output stays in `research/` or `programs/<slug>/plan-rounds.jsonl` — **not** package truth
- `user_decision=promoted` on research required before P3 feature-graph finalize

## Workflow (BR + P0–P4 interactive profile)

| Step | Action | Output |
|------|--------|--------|
| BR-0 | context scan (repo/docs if any) | research stub |
| BR-1..N | `beacon brainstorm run` **one question per turn** (`requirement-convergence`) | `research/<program>.md` updates |
| BR-方案 | 2–3 approaches with trade-offs | research section |
| P0-lite | intent + partial inventory (explicit gaps OK) | inventory with `confidence: low` flags |
| P1 | module-map **draft** from research | `module-map.md` draft |
| P2 | mandatory pln-review | review artifact |
| P3 | feature-graph **draft** | `feature-graph.json` |
| P4 | sectional ack + deferral sovereignty | `program-ack.json` |

**Intent chain:** append each round to `programs/<slug>/plan-rounds.jsonl` (`utterance_delta`, `user_decision`, `timestamp`).

## CLI

```bash
beacon plan start "<utterance>" --mode interactive --program <slug> --version <v> --json
beacon plan interactive --program <slug> ...
```

## Cognitive Load Rules

- **one question per message** at BR phase (align Superpowers brainstorming)
- multiple-choice preferred when possible
- full parity capabilities live in `program-parity-matrix.md`; MVP only marks `phase`/`deferral`, never deletes rows
- stop BR when user says promoted / 可以出图了 / 进入拆书

## Route After P4

- umbrella → per-lake `beacon-gen-truth` seven-round loop
- if BR not promoted → stay interactive; **block** P5

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
