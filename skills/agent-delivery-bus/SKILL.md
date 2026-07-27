---
name: agent-delivery-bus
description: Govern local Beacon project work through strict preflight, scoped approval, idempotent Hermes Kanban dispatch, and Beacon evidence reconciliation. Use when listing or resolving registered Beacon projects, checking dispatch readiness, approving implement/freeze work, creating or inspecting Hermes tasks, or reconciling worker results with Beacon delivery gates.
---

# Agent Delivery Bus

Use the repository CLI as the control plane. Treat project routing, authorization, execution,
and delivery verdicts as separate authorities.

## Decision boundaries

- Resolve the project from `config/projects.json`; never infer a target from a similar name.
- Run strict preflight before proposing a real dispatch.
- Require a matching one-time approval for `implement` and `freeze`.
- Treat `release` as disabled in v0.0.1 even when an approval exists.
- Reuse the same normalized request for retries so Hermes idempotency remains stable.
- Treat Hermes worker completion as an execution receipt, then reconcile Beacon evidence.
- Stop on blocked results and report `reason_code` plus `resume_action`; do not run the repair.

## Minimal tool surface

From the Agent Delivery Bus repository:

```bash
bin/adb projects list --json
bin/adb projects resolve --slug <slug> --json
bin/adb doctor --project <slug> --json
bin/adb dispatch --project <slug> --stage <plan|implement|qa|freeze> --feature <feature> --dry-run --json
bin/adb approve --actor <actor> --project <slug> --stage <implement|freeze|release> --feature <feature> --json
bin/adb dispatch --project <slug> --stage <stage> --feature <feature> --approval-token <token> --json
bin/adb task show <dispatch-id> --json
bin/adb reconcile <dispatch-id> --json
```

Read [references/contracts.md](references/contracts.md) before performing a real dispatch or
interpreting reconciliation.

## Required reporting

Return the resolved project, stage, feature, dry-run/real mode, preflight result, dispatch id,
Hermes task id when present, current state, reason code, and next safe action.

Never include approval tokens in logs, task bodies, or later status responses.
