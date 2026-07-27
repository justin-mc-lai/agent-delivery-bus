# Agent Delivery Bus contracts

## Authority map

| Concern | Authority |
|---------|-----------|
| Project slug, aliases, repo and docs version | `config/projects.json` |
| Requirement truth, freeze, QA and release gates | Target project's Beacon materials |
| Persistent task lifecycle, worker claim and retries | Hermes Kanban |
| Approval, idempotency, local dispatch state and audit events | Agent Delivery Bus SQLite |
| Inspiration and knowledge text | Personal Brain |

## Restricted stages

- `implement`: approval required; Hermes workspace must be `worktree:<repo>`.
- `freeze`: approval required; Beacon truth-canonical branch rules still apply.
- `release`: approval tokens can be issued for future compatibility, but dispatch is disabled in
  v0.0.1 and must return `stage_not_enabled`.

## Preflight

The preflight is read-only and checks:

1. registered repo exists and is a Git worktree;
2. Beacon docs root and registered docs version exist;
3. `beacon doctor verify-context --strict --json` passes;
4. Hermes gateway is healthy;
5. Hermes `coding` profile exists.

Do not run `setup-context`, sync, migration, checkout, merge, or any write command as part of
preflight.

## Idempotency

The key is derived from schema version, project slug, canonical repo, docs version, stage and
feature. Repeat the same request unchanged after a known failure. After a timeout or unknown
Hermes result, reconcile by idempotency key before retrying.

## Completion

Hermes states such as `done` or `completed` move a dispatch to `reconciling`. The dispatch reaches
`completed` only when stage-specific Beacon evidence is present:

- plan: plan/truth/research artifact;
- implement: Beacon implementation evidence;
- qa: Beacon QA pass;
- freeze: frozen truth plus revision artifact.

## Hard prohibitions

- no Hermes internal database access;
- no automatic target-project repair;
- no direct target-project edits from preflight;
- no automatic release;
- no mapping of worker prose or Hermes success directly to Beacon completion.
