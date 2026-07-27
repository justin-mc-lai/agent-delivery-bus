# Evo Harness Contract (v0.0.1)

## Goal

- Run fixed-budget, real-input experiments over Beacon learning-plane candidates.
- Keep the editable surface small and auditable.
- Produce explicit keep/review/discard decisions instead of open-ended drift.

## Budget Contract

- max_runtime_minutes: `30`
- max_iterations: `3`
- max_command_count: `12`
- max_evidence_budget: `20`

## Subject Surface
- `qa_policy`
- `design_policy`
- `contract_policy`
- `rewrite_policy`
- `debug_policy`
- `benchmark_policy`

## Program Surface
- `benchmark_corpus`
- `decision_scorecard`
- `keep_discard_ledger`

## Real Benchmark Inputs
- benchmark_corpus: `.beacon/state/eval-lab/benchmark-corpus.json`
- scorecard: `.beacon/state/evo/scorecards/latest.json`
- ledger: `.beacon/state/evo/ledgers/latest.json`

## Latest Experiment
- run_id: `-`
- label: `-`
- selected_candidates: `0`
- verdict: `-`

