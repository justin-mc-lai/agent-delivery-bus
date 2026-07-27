---
name: beacon-implement
description: >
  Public implement harness (1+6 core). Code implementation after workspace admit and frozen truth. Modes: single/team/ralph under references/gen-implement/. Triggers: 实现, implement, 写代码, ralph. No QA self-certify or release pass.
metadata:
  version: "v1.6.10"
  brand: Beacon
  public_surface: true
  public_id: "implement"
  scheme: "A-1plus6-merged"
  progressive_map: skills/beacon/references/public-surface-progressive-map.v1.json
  dual_install_with: loom-implement
  longrun: false
---

# beacon-implement

**Public surface**: `implement` (Scheme A — only 1+6 host-visible cores)  
**Brand**: beacon  
**Capability home**: this package (`references/modes/` + nested refs). No separate fine-grained skill install required.

## HARD GATE

Implement only after workspace admit + frozen truth base. Forbidden: QA self-pass, release claim.

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
| `gen-implement` | `references/modes/gen-implement.md` | implement umbrella |

Index: `references/modes/INDEX.md`

## Workflow

1. Classify request → primary mode.
2. Read mode file + required nested references.
3. Produce artifacts for that mode only.
4. Recommend next public harness (`goal` pipeline order when long-run).
5. Never claim completion of another harness's job.

## Implement mode details

After reading `references/modes/gen-implement.md`, load the matching lane:

- `references/gen-implement/implement-single.md`
- `references/gen-implement/implement-team.md`
- `references/gen-implement/implement-ralph.md`

Always run workspace admission before writes.

## Output contract (minimum)

- `mode_id` (required)
- `public_id`: `implement`
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
