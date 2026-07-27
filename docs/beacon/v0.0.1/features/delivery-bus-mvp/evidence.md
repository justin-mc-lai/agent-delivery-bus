# Evidence: delivery-bus-mvp

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| requirements | Beacon truth | `truth.md` | draft | truth review → freeze |
| executable acceptance | Beacon tests contract | `tests.md` | planned | unittest + package exec TC |
| implementation | source + per-AC receipts | `.beacon/evidence/implement/delivery-bus-mvp/` | pending | implement |
| Hermes integration | public JSON CLI receipts | `.beacon/evidence/integration/hermes/` | pending | canary reconcile |
| skill compliance | skill-creator validator | `.beacon/evidence/skill/quick-validate.json` | pending | quick_validate.py |
| qa | Beacon QA verdict | `.beacon/evidence/qa/delivery-bus-mvp/` | pending | qa |
| release | human-only | none | disabled | no automatic route in v0.0.1 |

## Required evidence before completion

- All AC-bound unit/contract tests pass.
- Hermes adapter test proves exact argv and JSON parsing without touching internal DB.
- Canary dispatch is dry-run first and then runs only against a project whose strict preflight passes.
- Reconciliation evidence demonstrates Hermes success does not bypass Beacon closure.
- Skill validates and both install targets are checked without overwriting existing content.
- Adversarial review records evidence for duplicate dispatch, approval replay, unknown Hermes result,
  wrong-project routing, and false worker success.
