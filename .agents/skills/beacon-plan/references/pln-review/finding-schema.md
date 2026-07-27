# Finding Schema

Every planner review finding must be structured. Free-form critique is allowed only after the schema fields are present.

```text
finding_id
reviewer
severity: P0 | P1 | P2 | P3
confidence: 0 | 25 | 50 | 75 | 100
authority_level: user_quote | package_truth | source_evidence | implementation_evidence | runtime_evidence | assumption
issue
evidence_refs
affected_truth_surface
recommended_route
user_decision_required
autofix_allowed
```

## Severity

- `P0`: Continuing would change user intent, freeze false truth, fake completion, or cross a hard gate.
- `P1`: Required truth/test/evidence surface is missing for the claimed scope.
- `P2`: Important ambiguity or missing reviewer output; can proceed only with visible risk.
- `P3`: Improvement, wording, or non-blocking cleanup.

## Defaults

- `autofix_allowed` defaults to `false` for all planner findings.
- `user_decision_required` is `true` for scope downgrades, deferrals, MVP cuts, parity gaps, and state-machine omissions.
- P0/P1 `intent-fidelity`, `source-parity`, `deferral-sovereignty`, or `state-machine` findings block direct `beacon-gen-implement`.
- Planner cannot mark QA/release pass. Evidence judgment routes to `beacon-eval-qa` or `beacon-eval-release`.

## Route Rules

| Finding class | P0/P1 route |
|---|---|
| `intent-fidelity` | `beacon-gen-truth` or `beacon-gen-change` |
| `source-parity` | `beacon-gen-truth` or `beacon-gen-change` |
| `deferral-sovereignty` | `beacon-gen-change` or wait for user decision |
| `state-machine` | `beacon-gen-truth`, `beacon-gen-change`, or `beacon-gen-refreeze` |
| `coverage-shape` | `beacon-gen-truth` for coverage or `beacon-eval-qa` for evidence judgment |
| `route-boundary` | Stop and reroute to the correct harness |

## Minimal Finding Example

```text
finding_id: review-state-001
reviewer: state-machine
severity: P1
confidence: 75
authority_level: user_quote
issue: User requested a resumable multi-agent loop, but truth lacks State Model and tests lack illegal transition coverage.
evidence_refs: user prompt, docs/beacon/v1.6.0/features/<slug>/truth.md
affected_truth_surface: truth.md, tests.md
recommended_route: beacon-gen-change
user_decision_required: true
autofix_allowed: false
```
