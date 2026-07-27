---
name: beacon-release
description: >
  Public release harness (1+6 core). Release readiness, human gate, install/projection checks. Triggers: 发布, release, gate, readiness. Mode eval-release under references/modes/. Human gate required; no fake pass.
metadata:
  version: "v1.6.10"
  brand: Beacon
  public_surface: true
  public_id: "release"
  scheme: "A-1plus6-merged"
  progressive_map: skills/beacon/references/public-surface-progressive-map.v1.json
  dual_install_with: loom-release
  longrun: false
---

# beacon-release

**Public surface**: `release` (Scheme A — only 1+6 host-visible cores)  
**Brand**: beacon  
**Capability home**: this package (`references/modes/` + nested refs). No separate fine-grained skill install required.

## HARD GATE

Release verdict + evidence. Human gate required. Forbidden: auto-pass without evidence.

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
| `eval-release` | `references/modes/eval-release.md` | release gate |

Index: `references/modes/INDEX.md`

## Workflow

1. Classify request → primary mode.
2. Read mode file + required nested references.
3. Produce artifacts for that mode only.
4. Recommend next public harness (`goal` pipeline order when long-run).
5. Never claim completion of another harness's job.

## Output contract (minimum)

- `mode_id` (required)
- `public_id`: `release`
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
