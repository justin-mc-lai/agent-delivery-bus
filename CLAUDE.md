<!-- BEACON:START -->
<!-- BEACON:VERSION:v1.6.12 -->
<!-- BEACON:DOCS_VERSION:v0.1.4 -->
# CLAUDE.md

This file is Beacon-managed runtime guidance.

## Beacon usage
- Use Beacon commands for product workflow execution.
- Keep outputs evidence-first and machine-readable when possible.
- Prefer `beacon <subcommand>` form for cross-environment stability.

## Key commands
- `beacon init --apply`
- `beacon doctor setup-context --project-root .`
- `beacon doctor verify-context --project-root . --strict`
- `beacon doctor sync-materials --project-root . --version <version> --all-features`
- `beacon workflow start "<feature>" --version <version> --project-root .`
- `beacon skill list`
- `beacon implement list-runners --json`
- `beacon implement run "<feature>" --version <version> --runner <runner>`
- `beacon workflow --help`
- `beacon status --verbose`

## Requirement materials and docs paths
- Runtime target version: `v1.6.12`
- Docs target version: `v0.1.4`
- `docs/beacon/global-boundaries.md` is the canonical cross-version global constraint source.
- `docs/beacon/<version>/` is the canonical source of truth.
- `docs/beacon/<version>/SUMMARY.md`
- `docs/beacon/<version>/prd/`
- `docs/beacon/<version>/user-story/`
- `docs/beacon/<version>/qa/test-cases/`
- `docs/beacon/<version>/execution/`
<!-- BEACON:END -->

