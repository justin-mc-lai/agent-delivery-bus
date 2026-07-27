# Multi-Agent Review Runtime

Implementation line: `beacon-pln-review-runtime-multi-agent-v160` (`beacon/utils/planner_review_runtime.py` + `beacon planner-review run`).

`beacon-pln-review` supports planner-only multi-reviewer runtime evidence.

This is not a new lifecycle stage and not a set of host-visible reviewer skills. Reviewer lanes are internal runtime evidence used to make planning review auditable before routing to truth/change/implement/QA/release.

## Runtime Contract

The runtime artifact records:

- `selected_reviewers`
- `reviewer_lanes`
- independent `findings`
- `synthesis`
- `state_model`
- `execution_mode`
- `fallback_reason`
- evidence authority flags

## Reviewer Lanes

Each selected reviewer gets an independent lane with:

- reviewer id
- lane id
- trigger reason
- input refs
- output status
- finding ids
- confidence
- authority level
- runtime evidence ref

A lane must produce either a finding or `no_findings`.

## Synthesis

The synthesis step merges duplicate findings, preserves `merged_from`, records conflicts, chooses the highest severity, and emits one `recommended_route`.

## Fallback

If the host has no subagent runtime, the artifact must use `single_process_multi_reviewer` and include a fallback reason. It must not claim parallel subagent execution.

## Boundary

Planner review runtime evidence is support evidence only:

- not requirement truth
- not implementation
- not formal QA verdict
- not release verdict
- not permission to write `.machine`

When review changes promises, route to `beacon-gen-change` or `beacon-gen-refreeze`.
