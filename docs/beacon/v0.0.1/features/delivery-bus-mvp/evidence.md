# Evidence Index: delivery-bus-mvp

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| requirements | requirement_truth | `truth.md` | linked | truth review → freeze |
| executable acceptance | acceptance_truth | `tests.md` | linked | unittest + package exec TC |
| test contract | test_truth | `tests.md` | linked | qa |
| implementation receipts | support_advisory | `.beacon/evidence/implement/delivery-bus-mvp/` | indexed | implement |
| Hermes integration receipts | support_advisory | `.beacon/evidence/integration/hermes/` | indexed | reconcile |
| skill validation receipts | support_advisory | `.beacon/evidence/skill/quick-validate.json` | indexed | quick_validate.py |
| qa | qa_verdict | `docs/beacon/v0.0.1/.machine/qa/delivery-bus-mvp.qa9-matrix.json` | linked | qa |
| release | release_verdict | `docs/beacon/v0.0.1/release/` | disabled-by-scope | human-only |

## Boundary

`evidence.md` 只是索引/read model，不声明最终 gate 结论，也不替代 Beacon QA 或
release authority。

## Required evidence before completion

- All AC-bound unit/contract tests pass.
- Hermes adapter test proves exact argv and JSON parsing without touching internal DB.
- Canary dispatch is dry-run first and then runs only against a project whose strict preflight passes.
- Reconciliation evidence demonstrates Hermes success does not bypass Beacon closure.
- Skill validates and both install targets are checked without overwriting existing content.
- Adversarial review records evidence for duplicate dispatch, approval replay, unknown Hermes result,
  wrong-project routing, and false worker success.
