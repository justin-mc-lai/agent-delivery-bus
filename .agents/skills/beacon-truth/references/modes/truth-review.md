# Mode: truth-review

> Progressive disclosure for `beacon-truth` (Scheme A 1+6). Implements Plan→Truth **Mandatory Review Gate** (v1.6.10+).

## Purpose

Force intent / FSM / testability review **before** freeze. Wrong truth cascades into tasks, implement, and tests.

## CLI

```text
beacon truth review <feature> --project-root . --version <ver> [--json]
```

- pass ⇒ `review_gate_pass=true` (report under `docs/beacon/<ver>/.machine/execution/<slug>.truth-review-gate.json`)
- fail ⇒ reason_code `truth_review_gate_failed` — **blocks** `beacon freeze`

## Three-step pipeline

```text
1. Plan / Draft Truth (scaffold = draft only)
2. Truth Review Gate
   A Intent Coverage Matrix (must ≠ missing)
   B FSM State|Event|Guard|To|Action + hang + illegal (domain_required)
   C every AC ≥ 1 TC (Command + Assertion)
   C_exec business domain: ≥1 execution-layer TC (not docs-only rg)
   D fill depth: package_maturity≠scaffold; 旅程 filled; no meta-only FSM
   then Humanizer: host agent reads skill fulltext + CLI rules safety-net (after A/B/C; preserve_machine_fields; **no API key**)
3. Freeze only if review_gate_pass (A–D)
4. Freeze post: tasks.md derived from AC
```

## Boundaries

- **No old-shape soft-compat (global-boundaries §2.1)**：v1.6.10+ 不合格包必须 replan+refreeze，不得降级 Check A–D / tasks evidence 形状。


- `beacon-plan` / pln-review: draft + advisory only — **cannot** authorize freeze alone.
- Humanizer: polish L0/narrative only; `preserve_machine_fields=true`.
- refreeze: re-run Gate A/B/C; reopen marks tests/tasks `materials_status: stale`.

## Output contract

- `mode_id`: `truth-review`
- `public_id`: `truth`
- `review_gate_pass`: bool
- `reason_code`: empty | `truth_review_gate_failed` | …
- `recommended_next_harness`: `truth` (freeze) if pass else stay on `truth` (fix)

## Gold reference

- Feature package: `docs/beacon/v1.6.10/features/truth-gold-order-pay-v1610/`
- Skill example: `skills/beacon/beacon-truth/references/examples/gold-order-pay-package.md`
