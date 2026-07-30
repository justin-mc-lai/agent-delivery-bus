# Agent Delivery Bus contracts

## Authority map

| Concern | Authority |
|---------|-----------|
| Project slug, aliases, repo and docs version | Project registry (`config/projects.json`) |
| Requirement truth, freeze, QA and release gates | TruthGateAdapter (demo: null, example: Beacon) |
| Persistent task lifecycle, worker claim and retries | ExecutorAdapter (demo: null, example: Hermes Kanban) |
| Approval, idempotency, local dispatch state and audit events | Agent Delivery Bus SQLite |
| Inspiration, knowledge notes, templates, collaboration prose | External Knowledge OS (not part of core) |

## Adapter SPI

Core depends only on:

- `TruthGateAdapter.preflight_checks` / `closure`
- `ExecutorAdapter.preflight_checks` / `board_for` / `workspace_for` / `ensure_board` / `create_task` / `show_task` / `find_by_idempotency`

Beacon and Hermes are example adapters. Replace them without changing the ledger.

## Restricted stages

- `implement`: approval required; executor workspace should isolate writes.
- `freeze`: approval required; truth-canonical branch rules still apply.
- `release`: approval tokens can be issued for future compatibility, but dispatch is disabled and must return `stage_not_enabled`.

## Worker ↔ Beacon binding (task body)

Successful `adb dispatch` Hermes task bodies include a `### Beacon worker binding` section with:

- `stage` → `beacon_skill` / `public_harness` / `beacon_command` (at least `plan` → `beacon-plan`)
- `runner_kind: local_agent` with Hermes `coding` profile (or explicit `codex` / equivalent); `cloud_scheduler_forbidden: true`
- Enabled stages: `plan`, `implement`, `qa`, `freeze`. `goal` is deferred (`goal_stage_deferred`) until an explicit promote/change.

Binding never bypasses approval for restricted stages, and admission/preflight failures must not create executor tasks.

## Preflight

Core preflight is read-only and always checks:

1. registered repo exists;
2. repo is a Git worktree;

Then it aggregates adapter checks. Example adapters currently verify:

- truth docs root/version and Beacon strict context;
- Hermes CLI, gateway, and coding profile.

Do not run setup, sync, migration, checkout, merge, or any write command as part of preflight.

## Idempotency

The key is derived from schema version, project slug, canonical repo, docs version, stage and feature. Repeat the same request unchanged after a known failure. After a timeout or unknown executor result, reconcile by idempotency key before retrying.

## Completion

Executor states such as `done` or `completed` move a dispatch to `reconciling`. The dispatch reaches `completed` only when stage-specific truth-gate evidence is present.

## Hard prohibitions

- no executor internal database access;
- no automatic target-project repair;
- no direct target-project edits from preflight;
- no automatic release;
- no mapping of worker prose or executor success directly to delivery completion;
- no embedding a personal knowledge wiki into the scheduling core.
