# State Machine Review

Finite state machine review is a hard standard for lifecycle and parity work, not a decorative checklist.

## Trigger

Run the `state-machine` reviewer when any condition appears:

- lifecycle, phase, state, queue, status, resume, stop, rollback, recovery, retry, or long-running loop;
- multi actor, multi agent, multi harness, permission boundary, or cross-interface state;
- source parity, full parity, same capability, or complete clone;
- illegal transition, blocked state, partial completion, fake completion, or release gate risk;
- diagram truth identifies `branch-flow`, `state-machine`, `decision-priority`, or more than two state axes.

## Required Output

```text
state_model:
  required: true | false
  states:
  allowed_transitions:
  invalid_transitions:
  side_effects:
  recovery_paths:
  rollback_paths:
  resume_paths:
  terminal_states:
  state_axes:
  axis_combinations:
  coverage_implications:
```

## Truth Landing

If state-machine review is required, package truth must have a `State Model` or `Diagram Truth Layer` landing that declares:

- state-machine required or not required;
- named states;
- allowed and blocked transitions;
- side effects and generated artifacts;
- recovery, rollback, resume, and terminal states;
- state axes and important axis combinations.

Planner review cannot write this truth. It must route to `beacon-gen-truth`, `beacon-gen-change`, or `beacon-gen-refreeze`.

## Tests Landing

`tests.md` must cover:

- allowed transition behavior;
- illegal transition rejection;
- blocked state handling;
- recovery/rollback/resume;
- terminal state behavior;
- state axis combinations;
- diagram truth propagation into acceptance and evidence.

If tests only prove static existence while runtime behavior is required, emit a `coverage-shape` finding and route to coverage or QA.

## State Reviewer Negative

If a prompt asks the planner to "just implement the state machine" or "mark it QA pass", emit a `route-boundary` P0 finding and stop.
