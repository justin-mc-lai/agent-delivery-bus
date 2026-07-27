---
name: beacon-design
description: >
  Public design harness (1+6 core). Design system, explore, polish, prototype, UX delivery. Triggers: 设计, design system, prototype, polish, UI/UX. Modes under references/modes/. No silent truth rewrite or implement.
metadata:
  version: "v1.6.10"
  brand: Beacon
  public_surface: true
  public_id: "design"
  scheme: "A-1plus6-merged"
  progressive_map: skills/beacon/references/public-surface-progressive-map.v1.json
  dual_install_with: loom-design
  longrun: false
---

# beacon-design

**Public surface**: `design` (Scheme A — only 1+6 host-visible cores)  
**Brand**: beacon  
**Capability home**: this package (`references/modes/` + nested refs). No separate fine-grained skill install required.

## HARD GATE

Design/prototype artifacts only. Forbidden: silent truth rewrite, implement as design completion.

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
| `gen-design` | `references/modes/gen-design.md` | design delivery |
| `gen-design-init` | `references/modes/gen-design-init.md` | design init |
| `gen-design-system` | `references/modes/gen-design-system.md` | design system |
| `gen-design-explore` | `references/modes/gen-design-explore.md` | explore variants |
| `gen-design-polish` | `references/modes/gen-design-polish.md` | polish |
| `gen-prototype` | `references/modes/gen-prototype.md` | prototype |

Index: `references/modes/INDEX.md`

## Workflow

1. Classify request → primary mode.
2. Read mode file + required nested references.
3. Produce artifacts for that mode only.
4. Recommend next public harness (`goal` pipeline order when long-run).
5. Never claim completion of another harness's job.

## Output contract (minimum)

- `mode_id` (required)
- `public_id`: `design`
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
