---
name: agent-delivery-bus
description: Govern local multi-project agent delivery through strict preflight, scoped approval, idempotent executor dispatch, and truth-gate evidence reconciliation. Use when listing or resolving registered projects, checking dispatch readiness, approving implement/freeze work, creating or inspecting executor tasks, or reconciling worker results with delivery evidence.
---

# Agent Delivery Bus

Use the repository CLI as the control plane. Treat project routing, authorization,
execution, and delivery verdicts as separate authorities.

## Architecture

```text
Human / Knowledge OS intent
        |
        v
 Agent Delivery Bus Core
 registry -> preflight -> approval -> idempotent dispatch -> reconcile
   |            |                         |                    |
   |            +-- TruthGateAdapter      |                    +-- evidence
   |            +-- ExecutorAdapter       v
   +-- projects.json                 example: Hermes
```

Knowledge folders (projects / assets / inspiration / collaboration rules) stay
outside this control plane. They may produce intent, but they never become the
scheduler, the worker, or the delivery gate.

## Decision boundaries

- Resolve the project from the registry; never infer a target from a similar name.
- For natural-language intents: call `adb intent parse` first, show the IntentEnvelope, and obtain human confirmation before `adb assign` / `adb approve` / `adb dispatch`.
- Never call `adb dispatch` from an unconfirmed envelope (`requires_confirmation` / missing actor ack).
- Run strict preflight before proposing a real dispatch.
- Require a matching one-time approval for `implement` and `freeze`.
- Treat `release` as disabled even when an approval exists.
- Reuse the same normalized request for retries so executor idempotency remains stable.
- Treat worker completion as an execution receipt, then reconcile truth-gate evidence.
- Stop on blocked results and report `reason_code` plus `resume_action`; do not run the repair.

## Minimal tool surface

```bash
bin/adb projects list --json
bin/adb projects resolve --slug <slug> --json
bin/adb doctor --project <slug> --json
bin/adb intent parse --utterance "<natural language>" --json
bin/adb intent parse --utterance "<natural language>" --project <slug> --json
bin/adb assign candidates --project <slug> --stage <stage> --feature <feature> --json
bin/adb dispatch --project <slug> --stage <plan|implement|qa|freeze> --feature <feature> --dry-run --json
bin/adb approve --actor <actor> --project <slug> --stage <implement|freeze|release> --feature <feature> --json
bin/adb dispatch --project <slug> --stage <stage> --feature <feature> --approval-token <token> --json
bin/adb task show <dispatch-id> --json
bin/adb reconcile <dispatch-id> --json
bin/adb fleet --json
bin/adb fleet --project <slug> --json
bin/adb boards status --project <slug>
bin/adb boards status --project <slug> --json
bin/adb approvals awaiting --channel feishu --json
```

Dispatch task bodies embed the project's stage→worker binding profile plus an
evidence spec for the local Hermes `coding` (or Codex) runner. `beacon` is the
built-in reference profile; any truth-gate system may be used as long as the
project declares its profile/adapters through the registry contract (Beacon is
the reference implementation, not a dependency). `goal` is not an enabled
dispatch stage by default.

### Confirm gate (Hermes skill contract)

1. Parse: `adb intent parse --utterance ... --json`
2. If `blocked`: report `reason_code` / `resume_action` / candidates; do not dispatch.
3. Present `data.envelope` to the human.
4. Only after explicit confirmation may you call assign / approve / dispatch using envelope fields.
5. Parse/confirm paths must never create executor tasks or consume approval tokens.

Read [references/contracts.md](references/contracts.md) before performing a real
dispatch or interpreting reconciliation.

## Required reporting

Return the resolved project, stage, feature, dry-run/real mode, preflight result,
dispatch id, executor task id when present, current state, reason code, and next
safe action.

Never include approval tokens in logs, task bodies, or later status responses.
