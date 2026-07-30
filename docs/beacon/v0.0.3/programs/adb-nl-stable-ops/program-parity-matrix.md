# Program Parity Matrix — adb-nl-stable-ops

| Capability | Lake | Phase | Defer? | Notes |
|------------|------|-------|--------|-------|
| intent-parse-cli | nl-intent-envelope | L1 / v0.0.3 | no | 稳定性核心 |
| intent-envelope-schema | nl-intent-envelope | L1 | no | |
| alias-resolve-fail-closed | nl-intent-envelope | L1 | no | |
| ambiguity-reason-codes | nl-intent-envelope | L1 | no | |
| hermes-skill-confirm-gate | nl-intent-envelope | L1 | no | |
| assign-candidates-bridge | nl-intent-envelope | L1 | no | 复用 v0.0.2 |
| pytest-intent-contract | nl-intent-envelope | L1 | no | |
| stage-to-skill-map | worker-beacon-binding | L2 | no | |
| codex-or-coding-runner-profile | worker-beacon-binding | L2 | no | |
| task-body-contract | worker-beacon-binding | L2 | no | |
| workspace-admission-hook | worker-beacon-binding | L2 | no | |
| plan-stage-binding | worker-beacon-binding | L2 | no | |
| goal-stage-binding | worker-beacon-binding | L2+ | **default defer** | P4 可升格 |
| fleet-table-contract | kanban-ops-nl | L2 | no | |
| boards-status-nl-actions | kanban-ops-nl | L2 | no | |
| awaiting-approvals-table | kanban-ops-nl | L2 | no | |
| hermes-public-cli-only | kanban-ops-nl | L2 | no | |
| version-list-summary | beacon-read-surface | L2 | no | |
| latest-requirement-digest | beacon-read-surface | L2 | no | |
| beacon-public-cli-only | beacon-read-surface | L2 | no | |
| hermes-cron-job-template | ops-digest-cron | L3 / v0.0.4 | no | |
| digest-render-fleet-plus-awaiting | ops-digest-cron | L3 | no | |
| feishu-payload-handoff | ops-digest-cron | L3 | no | 不直连 OpenAPI |
| digest-idempotency | ops-digest-cron | L3 | no | |
| brain-curation-job | knowledge-curation-digest | L4 | deferrable | ADB 外 |
| curation-summary-payload | knowledge-curation-digest | L4 | deferrable | |
| outside-adb-core | knowledge-curation-digest | L4 | n/a | 硬边界 |
| embedded-nlu-service | — | — | **rejected** | Approach C |
| auto-release | — | — | **rejected** | global boundary |
| hermes-private-db-read | — | — | **rejected** | |

## Deferral ledger

| ID | Item | Default | Promote condition |
|----|------|---------|-------------------|
| D1 | goal-stage-binding | defer to after plan binding | 用户明确要 goal 一等 stage + 门控策略 |
| D2 | knowledge-curation-digest | support / v0.0.4+ | 看板 digest 稳定后 |
| D3 | Web UI | rejected for this program | 另开 program |
| D4 | Orca-as-primary-scheduler | rejected for this program | 保持 Hermes Kanban |
