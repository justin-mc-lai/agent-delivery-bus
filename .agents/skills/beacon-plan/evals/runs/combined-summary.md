# beacon-plan combined eval (iter1 + iter2 parity fix)

- with_skill_mean: **1.0**
- baseline_mean: **0.762**
- mean_delta: **0.237**

| case | with_skill | baseline | delta |
|------|------------|----------|-------|
| plan-contract-hard-gate | 4/4 (1.0) | 1/4 (0.25) | +0.75 |
| plan-contract-review-parity | 5/5 (1.0) | 4/5 (0.8) | +0.2 |
| plan-contract-review-cli | 3/3 (1.0) | 3/3 (1.0) | +0.0 |
| plan-mode-selection-source | 3/3 (1.0) | 3/3 (1.0) | +0.0 |

## Notes
- iter1: full 4 cases; parity underspec failed (ask-only).
- iter2: re-ran parity after Underspec-still-emit skill fix → with_skill 5/5.
- Host real dirs replaced with Scheme A symlinks (agents/claude/codex).
- Install scripts now force-replace non-symlink core skill dirs.

