# Mode: pln-program-auto

> Archived from `beacon-pln-program-auto` during 1+6 merge. This is progressive disclosure content for `beacon-plan`.

# Beacon Planner Program — Auto Mode

## Overview

Auto mode minimizes interactive rounds while preserving governance: **machine P0 → P1 → P2 review → P3 graph → sectional P4 ack**. For「复刻 GitHub 开源项目全功能」类 utterance.

## When To Use (delegation only)

- `plan_mode=auto` from `beacon-pln-program`
- `--source` URL or local repo path provided
- `scope_mode` is `full_parity` or `phased_full`
- **Not** for vague product ideas without source (route interactive)

## Boundary

- same as `beacon-pln-program` umbrella
- additionally: auto inventory lines **must** cite `path` or `symbol` evidence from source scan
- if P0 inventory confidence low, **stop** and route umbrella to reclassify `interactive` — do not fabricate capabilities

## Workflow (P0–P4 auto profile)

| Step | Action | Output |
|------|--------|--------|
| P0 | `beacon-pln-source` or `beacon plan scan` (when CLI exists) + README/package tree | `source_capability_inventory.json` |
| P1 | module-map from inventory | `module-map.md` |
| P2 | **mandatory** `beacon planner-review run <program> --prompt "<utterance>"` | review artifact + parity matrix draft |
| P3 | feature-graph lakes from module-map | `feature-graph.json` |
| P4 | **sectional ack** (manifest → module-map → lake list); one revision round per section max | `program-ack.json` |

P4 UX: present three short sections (Superpowers-style chunking), not one 50-page dump.

## CLI

```bash
beacon plan start "<utterance>" --mode auto --source <url> --program <slug> --version <v> --json
# implementation alias:
beacon plan auto --program <slug> ...
```

## Operator Rules

- prefer deterministic scan + source citations over LLM-only inventory
- deferrals go to program parity matrix + deferral ledger; **no pending**
- after P4 ack, return control to umbrella → route `beacon-gen-truth` per lake

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
