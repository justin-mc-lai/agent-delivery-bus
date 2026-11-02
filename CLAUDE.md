<!-- BEACON:START -->
<!-- BEACON:VERSION:v1.6.16 -->
<!-- BEACON:DOCS_VERSION:v0.1.4 -->
# CLAUDE.md

This file is Beacon-managed runtime guidance.

## Beacon usage
- Use Beacon commands for product workflow execution.
- Keep outputs evidence-first and machine-readable when possible.
- Prefer `beacon <subcommand>` form for cross-environment stability.

## Key commands (beacon-go; the Python v1 `beacon` CLI is not installed)
- `beacon-go goal run "<utterance>" --project . --feature <slug> --executor-id <id>`
- `beacon-go goal status --run-id <run> --json`
- `beacon-go truth list --project .`
- `beacon-go truth show --feature <slug> --project . --json`
- `beacon-go gate status --feature <slug> --project . --json`
- `beacon-go spine show --project . --feature <slug> --json`
- `beacon-go doctor rebuild --project .`
- `python3 -m pytest tests/ -q`
- `cd cmd/adbd && go test ./...`
- No beacon-go equivalent yet: program graph validation, v1 feature-package validation.
- `beacon status --verbose`

## Requirement materials and docs paths
- Runtime target version: `v1.6.16`
- Docs target version: `v0.1.4`
- `docs/beacon/global-boundaries.md` is the canonical cross-version global constraint source.
- `docs/beacon/<version>/` is the canonical source of truth.
- `docs/beacon/<version>/SUMMARY.md`
- `docs/beacon/<version>/prd/`
- `docs/beacon/<version>/user-story/`
- `docs/beacon/<version>/qa/test-cases/`
- `docs/beacon/<version>/execution/`
<!-- BEACON:END -->


