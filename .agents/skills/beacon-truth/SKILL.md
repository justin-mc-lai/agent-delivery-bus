---
name: beacon-truth
description: >
  Public truth harness (1+6 core). Freeze/init feature package truth, change, refreeze, user-story/prd/test-case materials. Triggers: 冻需求, freeze, truth, change, refreeze, 写 truth. Modes under references/modes/. No implement or QA self-pass.
metadata:
  version: "v1.6.10"
  brand: Beacon
  public_surface: true
  public_id: "truth"
  scheme: "A-1plus6-merged"
  progressive_map: skills/beacon/references/public-surface-progressive-map.v1.json
  dual_install_with: loom-truth
  longrun: false
---

# beacon-truth

**Public surface**: `truth` (Scheme A — only 1+6 host-visible cores)  
**Brand**: beacon  
**Capability home**: this package (`references/modes/` + nested refs). No separate fine-grained skill install required.

## HARD GATE

May write/freeze package truth only after plan readiness. Forbidden: implement, fake freeze, QA self-pass.

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
| `gen-truth` | `references/modes/gen-truth.md` | authoritative truth package |
| `gen-truth-init` | `references/modes/gen-truth-init.md` | init feature package |
| `gen-change` | `references/modes/gen-change.md` | change log / delta |
| `truth-review` | `references/modes/truth-review.md` | Plan→Truth Review Gate before freeze |
| `gen-refreeze` | `references/modes/gen-refreeze.md` | refreeze after change |

Index: `references/modes/INDEX.md`

## Workflow

1. Classify request → primary mode.
2. Read mode file + required nested references.
3. Produce artifacts for that mode only.
4. Recommend next public harness (`goal` pipeline order when long-run).
5. Never claim completion of another harness's job.

## Output contract (minimum)

- `mode_id` (required)
- `public_id`: `truth`
- `evidence_refs` (paths/commands used)
- `recommended_next_harness` one of: goal|plan|truth|design|implement|qa|release|stop
- Mode-specific fields (plan_artifact, findings, truth package paths, QA scorecard, release brief, …) per mode file

## Evals

Smoke + contract prompts: `evals/evals.json`  
Examples (when present): `references/examples/`  
Gold (v1.6.10 订单支付完整包): `references/examples/gold-order-pay-package.md`  
Docs twin: `docs/beacon/v1.6.10/features/truth-gold-order-pay-v1610/`

## Shared package refs

- `skills/beacon/references/public-surface-1plus6.md`
- `skills/beacon/references/public-surface-progressive-map.v1.json`
- `skills/beacon/references/git-worktree-execution-flow.md`
