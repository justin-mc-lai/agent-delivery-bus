# Mode: pln-program

> Archived from `beacon-pln-program` during 1+6 merge. This is progressive disclosure content for `beacon-plan`.

# Beacon Planner Program (Umbrella)

## Overview

`beacon-pln-program` is the **only recommended host-visible entry** for version-level program planning (P0–P4). It classifies `plan_mode`, delegates to a sub-skill, and aligns with CLI governance. It is **not** package truth and **not** a new lifecycle stage.

## When To Use

- user utterance is **ocean** scope: full OSS parity, multi-module replication, book-shaped program truth
- user asks for `beacon plan`, program harness,海→湖拆分, feature graph, program manifest
- user already has or needs `docs/beacon/<version>/programs/<program-slug>/`
- after program P4 ack, route per-lake work to `beacon-gen-truth` (not this skill)

## Boundary

- writes **only** `docs/beacon/<version>/programs/<slug>/` advisory artifacts (via delegated sub-skill)
- **never** writes `features/<slug>/truth.md` or `.machine/` requirement truth
- **never** issues freeze/QA/release verdicts
- **never** skip P2 `beacon-pln-review` before P3 feature-graph promotion
- sub-skills (`beacon-pln-program-auto`, `beacon-pln-program-interactive`) are **delegation targets**, not alternate public catalog entries for operators

## Plan Mode Routing (mandatory)

Classify **exactly one** `plan_mode` before delegation:

| `plan_mode` | Delegate to | Typical signals |
|-------------|-------------|-----------------|
| `auto` | `beacon-pln-program-auto` | source URL/repo;「全功能/完整复刻/parity」; structured upstream repo |
| `interactive` | `beacon-pln-program-interactive` | vague product intent;「先聊聊/头脑风暴」; many open questions |
| `auto-detect` | run classifier below, then delegate | default when user did not pass `--mode` |

**Classifier (deterministic first, then host tie-break):**

1. explicit `--mode auto|interactive` from CLI wins
2. else if `source_url` or `github.com/` in utterance **and** full-parity signals → `auto`
3. else if brainstorm/explore/聊聊/对齐 signals → `interactive`
4. else if `lake_or_ocean=海` and confidence low → **stop**, ask user one multiple-choice question (`auto` vs `interactive`)
5. log decision to `programs/<slug>/program-manifest.md` frontmatter `plan_mode`

## CLI Governance (stable operator surface)

Prefer **one** human command; subcommands are implementation detail:

```bash
beacon plan start "<utterance>" \
  --project-root . \
  --version <version> \
  --program <program-slug> \
  --mode auto|interactive|auto-detect \
  --source <url> \
  --json
```

Until CLI ships (Phase E), mirror contract via skill delegation only; do not invent alternate command names in truth.

## Workflow

1. Resolve version + program slug; ensure `programs/<slug>/` exists or plan dry-run scaffold.
2. Classify `plan_mode` (table above).
3. **Delegate** to sub-skill; do not duplicate P0–P4 logic in this file.
4. After sub-skill completes P0–P4 artifacts, run **mandatory** `beacon-pln-review` (P2) if not already done in sub-skill.
5. Output route recommendation:
   - P4 not ack'd → stay in program planner
   - P4 ack'd → `beacon-gen-truth` per lake in graph topological order
   - change utterance → `beacon-gen-change` + pln-review + refreeze chain

## Cognitive Load Rules

- operators learn **one** phrase: `beacon plan start` or `/beacon-pln-program`
- artifact locations are fixed under `programs/<slug>/` (see `program-plan-contract.md`)
- mode selection is **either** explicit flag **or** one clarifying question — no unbounded questionnaire at umbrella layer
- do not expose harness-internal skill IDs to casual users in docs aimed at operators

## Red Flags

- sub-skill invoked without `plan_mode` recorded
- auto mode used without any source ref for full_parity scope
- interactive mode skipped brainstorm promotion and jumped to feature-graph
- umbrella skill writes package truth

## Evidence Produced

- `program-manifest.md` with `plan_mode`, `scope_mode`, intent one-liner
- delegation record (which sub-skill, classifier reason)
- route recommendation for P5/P6 or back to brainstorm/review

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
