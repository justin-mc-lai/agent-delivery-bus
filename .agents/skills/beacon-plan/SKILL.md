---
name: beacon-plan
description: >
  Public plan harness (1+6 core). Planning, multi-angle review, brainstorm before freeze, source-anchored research (对着代码/源码/别瞎编), program/sea-to-lake parity, friction capture. Triggers: 规划, 审查, review, brainstorm, 先想清楚, 源码, 代码, 拆海, program, parity, 状态机. Modes under references/modes/. Not longrun (use goal). No implement/truth freeze/QA verdict.
metadata:
  version: "v1.6.10"
  brand: Beacon
  public_surface: true
  public_id: "plan"
  scheme: "A-1plus6-merged"
  progressive_map: skills/beacon/references/public-surface-progressive-map.v1.json
  dual_install_with: loom-plan
  longrun: false
---

# beacon-plan

**Public surface**: `plan` (Scheme A — only 1+6 host-visible cores)  
**Brand**: beacon  
**Capability home**: this package (`references/modes/` + nested refs). No separate fine-grained skill install required.

## HARD GATE

Planner only. Forbidden: implement, write frozen truth, QA/release verdict, .machine writes as delivery.

Shared preamble:

1. Lake vs sea — boil lakes; split/defer seas.
2. Search before invent — truth, source, evidence, memory first.
3. User sovereignty on scope and freeze.
4. No fake delivery (placeholder, docs-only, zero assertions).
5. Harness boundaries — planner ≠ implement; generator ≠ self-certify; evaluator ≠ rewrite truth.

## Mandatory progressive load (do not skip)

1. Select **exactly one** primary `mode_id` from the table (use subintent/runtime when available).
2. **Read the mode file completely** before acting.
3. Open nested `references/<mode>/…` only when the mode file links them.
4. Emit the mode output contract + `mode_id` + `recommended_next_harness`.
5. Stop at harness boundary; route to another public core instead of stretching this one.

| mode_id | Load file | Purpose |
|---------|-----------|---------|
| `pln-review` | `references/modes/pln-review.md` | multi-angle planner review |
| `pln-brainstorm` | `references/modes/pln-brainstorm.md` | brainstorm before freeze |
| `pln-source` | `references/modes/pln-source.md` | source-anchored research |
| `pln-program` | `references/modes/pln-program.md` | sea-to-lake program plan |
| `pln-program-auto` | `references/modes/pln-program-auto.md` | auto program mode |
| `pln-program-interactive` | `references/modes/pln-program-interactive.md` | interactive program mode |
| `pln-friction` | `references/modes/pln-friction.md` | friction intake |

Index: `references/modes/INDEX.md`

## Workflow

1. Classify request → primary mode.
2. Read mode file + required nested references.
3. Produce artifacts for that mode only.
4. Recommend next public harness (`goal` pipeline order when long-run).
5. Never claim completion of another harness's job.

## Subintent → mode (plan only)

| Subintent | Mode | Mode file |
|-----------|------|-----------|
| review | pln-review | references/modes/pln-review.md |
| brainstorm | pln-brainstorm | references/modes/pln-brainstorm.md |
| source | pln-source | references/modes/pln-source.md |
| program | pln-program | references/modes/pln-program.md |
| friction | pln-friction | references/modes/pln-friction.md |
| default | pln-program | references/modes/pln-program.md |

Runtime: `detect_plan_subintent` / `plan_progressive_loads_for_utterance` → `primary_mode` + `primary_mode_path`.
Legacy ids like `beacon-pln-review` mean the same mode files (compat only).


## Review mode audit path

When mode is `pln-review` (or user asks multi-angle review / parity / 状态机):

1. Read `references/modes/pln-review.md` fully.
2. Load needed files under `references/pln-review/` (catalog, finding-schema, multi-agent-runtime, …).
3. Prefer runtime evidence:
   ```bash
   beacon planner-review run <feature> --project-root . --version <version> --prompt "<utterance>" --json
   ```
4. Artifact: `.beacon/state/planner-review/<feature>/<review_id>.json`
5. If no subagent runtime: `execution_mode=single_process_multi_reviewer` + `fallback_reason` (never claim parallel).

## Output contract (minimum)

- `mode_id` (required)
- `public_id`: `plan`
- `evidence_refs` (paths/commands used)
- `recommended_next_harness` one of: goal|plan|truth|design|implement|qa|release|stop
- Mode-specific fields (plan_artifact, findings, truth package paths, QA scorecard, release brief, …) per mode file


## Underspec still emit

Missing OSS/source/feature/version does **not** exempt the output contract. Emit `mode_id`, `intent_snapshot`, `scope_mode` (honor full_parity language), empty-or-blocked `parity_matrix`/`deferral_ledger`, P0 `findings`, and `recommended_next_harness=stop|plan`, then list clarifying questions.

## Evals

Smoke + contract prompts: `evals/evals.json`  
Examples (when present): `references/examples/`

## Shared package refs

- `skills/beacon/references/public-surface-1plus6.md`
- `skills/beacon/references/public-surface-progressive-map.v1.json`
- `skills/beacon/references/git-worktree-execution-flow.md`
