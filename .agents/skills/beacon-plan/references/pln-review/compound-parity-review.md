# Compound Parity Review

This reference defines the planner-only Compound-style review contract for `beacon-pln-review`.

It is not requirement truth, not implementation, not QA verdict, and not release verdict. It produces runtime/support evidence and route recommendations only.

## Output Contract

The review artifact must include:

- `intent_snapshot`
- `scope_mode`
- `diff_scope`
- `source_capability_inventory`
- `parity_matrix`
- `coverage_mapping`
- `deferral_ledger`
- `selected_reviewers`
- `reviewer_lanes`
- `findings`
- `synthesis`
- `autofix_routing`
- `release_ops_review`
- `qa_evidence_hygiene`
- `state_model`
- `recommended_route`
- `user_decision_required`
- `runtime_evidence_only`
- `requirement_truth`
- `formal_verdict_authority`
- `release_verdict_authority`

Required authority flags:

```text
runtime_evidence_only=true
requirement_truth=false
formal_verdict_authority=false
release_verdict_authority=false
```

## Reviewer Catalog

Always-on reviewers:

- `correctness-reviewer`
- `testing-reviewer`
- `maintainability-reviewer`
- `project-standards-reviewer`
- `intent-fidelity-reviewer`
- `route-boundary-reviewer`

Conditional reviewers:

- `source-parity-reviewer`
- `diff-scope-reviewer`
- `security-reviewer`
- `performance-reviewer`
- `api-contract-reviewer`
- `reliability-reviewer`
- `browser-ops-reviewer`
- `release-ops-reviewer`
- `adversarial-reviewer`
- `state-machine-reviewer`

These are internal reviewer lanes, not host-visible child skills.

## Source Parity Delivery

When the user says "完整复刻", "同等能力", "做全", "不要 MVP", `full parity`, `same capability`, or references an OSS/source project, default `scope_mode` to `full_parity`.

Full parity requires:

- `source_capability_inventory`
- `parity_matrix`
- `coverage_mapping`
- `deferral_ledger`

Missing source inventory or parity matrix blocks freeze/implement routing. Any downgrade from full parity to MVP, patch, or defer requires explicit user decision in the deferral ledger.

## Finding Schema

Each finding must include:

```text
finding_id
reviewer
lane_id
severity
confidence
authority_level
issue
evidence_refs
affected_surface
dedup_key
merged_from
conflict_group
owner
autofix_class
requires_verification
recommended_route
user_decision_required
```

Synthesis must merge by `dedup_key`, preserve `merged_from`, keep conflict groups, raise severity conservatively, and emit one `recommended_route`.

## Autofix Routing

Planner review never executes repairs.

| Class | Meaning | Route |
|---|---|---|
| `safe_auto` | Local deterministic repair candidate. | `beacon-gen-implement` |
| `gated_auto` | Repair changes behavior, contract, permission, version, or user promise. | `beacon-gen-change` or user-approved `beacon-gen-implement` |
| `manual` | Needs human or downstream resolver. | `human`, `beacon-pln-friction`, or `defer/queue` |
| `advisory` | Non-blocking evidence note. | `defer/queue` |
| `release` | Affects ship, install, canary, rollback, or projection. | `beacon-eval-release`, `beacon-gov-doctor`, or `beacon-gov-hooks` |

## Diff Scope

When code changes are in scope, review must record:

- base ref or reason unavailable
- changed tracked files
- untracked files
- generated/projection files
- deleted files
- touched harness surfaces
- high-risk file categories
- scope confidence

Unknown scope produces a degraded review, not a complete-review claim.

## Release Ops

Release/install/projection review must check:

- repo source skill
- project `.agents/skills` projection
- global agent skill projection
- stale duplicate skill copies
- post-release hook evidence
- canary or smoke install evidence
- rollback or cleanup route
- version surface alignment

Missing evidence routes to `beacon-eval-release`, `beacon-gov-doctor`, or `beacon-gov-hooks`.

## QA Evidence Hygiene

QA/benchmark/browser/compatibility review must check:

- real command execution
- `junit.xml`
- `assertion_count > 0`
- fake runner absence
- placeholder evidence absence
- browser admission explanation
- compatibility pair evidence
- benchmark metadata, grading, and analysis

`assertion_count_zero_pass`, `junit_missing_or_invalid`, `placeholder_evidence`, and `fake_runner` must become findings.

