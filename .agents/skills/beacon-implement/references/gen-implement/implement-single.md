# Implement Single Mode

## Purpose

Use `single` for small, low-risk, clearly scoped implementation under the single public `beacon-gen-implement` skill.

## Admission

- Requirement truth is frozen.
- Scope is narrow and low-risk.
- Role separation is not needed.
- Multi-round closure control is not needed.

## Command

```bash
beacon implement run "<feature>" --project . --version <version> --mode single
```

## Evidence

- Implementation summary.
- Changed file list.
- QA handoff when implementation evidence is ready.

## Boundary

Single mode cannot rewrite frozen truth, run QA, pass release, or override gate verdicts.
