# Stabilize eval notes (2026-07-21)

## Iteration-1 (full 4 cases)

After grader tighten:
- hard-gate: with_skill 1.0 vs baseline 0.25 (delta +0.75)
- parity: both 0.2 (both only ask questions — underspec contract missing)
- cli / mode-selection: both 1.0

mean_delta ≈ +0.19 (positive but parity weak)

## Fixes applied for iteration-2

1. `references/modes/pln-review.md` + SKILL: **Underspec still emit**
2. Eval system prompt forces structured fields even when blocking
3. Case prompt requires contract before clarifying questions
4. Grader rejects field-name-only mentions without structured headers
5. Host reinstall via `scripts/beacon-install.sh` (Scheme A flat)

## Success criteria

- with_skill parity score ≥ 0.8 and delta > 0 vs baseline
- hard-gate stays 1.0
