---
name: beacon-goal
description: >
  Public goal harness (1+6 core). Long-run delivery facade: plan→truth→design?→implement→qa→release. Triggers: 长程, goal run, 全流程, supervise, driver, multi-stage. Mode loop-goal under references/modes/. Not a substitute for single-surface plan/truth/implement alone when user only wants that step.
metadata:
  version: "v1.6.10"
  brand: Beacon
  public_surface: true
  public_id: "goal"
  scheme: "A-1plus6-merged"
  progressive_map: skills/beacon/references/public-surface-progressive-map.v1.json
  dual_install_with: loom-goal
  longrun: true
---

# beacon-goal

**Public surface**: `goal` (Scheme A — only 1+6 host-visible cores)  
**Brand**: beacon  
**Capability home**: this package (`references/modes/` + nested refs). No separate fine-grained skill install required.

## HARD GATE

Orchestrate pipeline only. Complete needs verifier evidence. Release always human gate. Longrun/supervise only on goal.
Hard stage exits via `goal_stage_exit_gates` + implement `ensure_goal_implement_admission` (workspace admit + freeze ack; fill-then-pass, not soft tick).

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
| `loop-goal` | `references/modes/loop-goal.md` | cross-harness loop orchestration |

Index: `references/modes/INDEX.md`

## Workflow

1. Classify request → primary mode.
2. Read mode file + required nested references.
3. Produce artifacts for that mode only.
4. Recommend next public harness (`goal` pipeline order when long-run).
5. Never claim completion of another harness's job.

## Goal pipeline

```text
plan → truth(freeze) → design? → implement → qa → release
```

- Load `references/modes/loop-goal.md` when cross-harness sequencing is required.
- Self-heal pre/mid/post from package map `skills/beacon/references/public-surface-progressive-map.v1.json`.
- Forbidden user sequence: treating loop-goal as a separate public skill (merged into this core).

## Output contract (minimum)

- `mode_id` (required)
- `public_id`: `goal`
- `evidence_refs` (paths/commands used)
- `recommended_next_harness` one of: goal|plan|truth|design|implement|qa|release|stop
- Mode-specific fields (plan_artifact, findings, truth package paths, QA scorecard, release brief, …) per mode file

## Evals

Smoke + contract prompts: `evals/evals.json`  
Examples (when present): `references/examples/`

## Shared package refs

- `skills/beacon/references/public-surface-1plus6.md`
- `skills/beacon/references/public-surface-progressive-map.v1.json`
- `skills/beacon/references/git-worktree-execution-flow.md`
