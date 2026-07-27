# Implement Team Mode

## Purpose

Use `team` for complex implementation that benefits from role separation, subagent lanes, or parallel evidence.

## Admission

- Requirement truth is frozen.
- Implementation spans multiple modules or concerns.
- Planner / implementer / reviewer / tester separation is useful.
- QA handoff needs role-specific context.

## Command

```bash
beacon implement run "<feature>" --project . --version <version> --mode team
```

## Evidence

- Team plan and role evidence.
- Implementation handoff.
- Review or QA readiness notes.

## Boundary

Team mode remains internal runtime behavior under `beacon-gen-implement`; it is not a public lifecycle skill and does not produce QA or release verdicts.
