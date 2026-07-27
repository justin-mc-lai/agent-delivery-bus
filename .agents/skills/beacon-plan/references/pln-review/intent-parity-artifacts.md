# Intent, Parity, And Deferral Artifacts

Planner review must preserve user intent before Beacon converts planning into package truth.

## Intent Snapshot

Capture:

- original user wording or stable paraphrase;
- strong intent signals such as "完整复刻", "同等能力", "做全", "不要 MVP", "same capability", or "full parity";
- named source projects and URLs;
- non-negotiable boundaries;
- any user-approved deferral or scope cut.

## Scope Mode

Use exactly one primary mode:

- `full_parity`
- `mvp`
- `patch`
- `research`
- `brownfield_takeover`
- `compatibility_only`

When the user says "完整复刻", "同等能力", "做全", "参考成熟项目实现", "parity", "same capability", or "不要 MVP", default to `full_parity`. Downgrading from `full_parity` requires a deferral ledger entry and explicit user decision.

## Source Capability Inventory

Required when source parity is in scope. Include:

- source capability;
- source workflow/state;
- source boundary or permission model;
- source evidence reference;
- target Beacon landing surface;
- target coverage/evidence expectation.

## Parity Matrix

Map each source capability to Beacon truth, tasks, tests, implementation evidence, and status.

| Source capability | Target truth | Target tests | Target evidence | Status | Deferral |
|---|---|---|---|---|---|
| `<capability>` | `<truth ref>` | `<tests ref>` | `<evidence ref>` | covered / missing / partial | none / user-approved / blocked |

## Promotion Diff

When planner support advisory is accepted, list exactly what must enter `beacon-gen-truth`, `beacon-gen-change`, or `beacon-gen-refreeze`:

- new promise;
- changed acceptance;
- new state model;
- added test obligation;
- approved deferral;
- unresolved risk.

## Deferral Ledger

Every deferred, cut, MVP-only, partial, or unimplemented capability needs:

```text
deferral_id
capability
reason
impact
user_decision: accepted | rejected | pending
expiry_or_revisit
recommended_route
```

If `user_decision` is `pending`, the planner must not recommend freezing the smaller truth as if it were complete.
