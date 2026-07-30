# Module Map (draft) — adb-nl-stable-ops

| Module | Owner surface | Responsibility | Lakes |
|--------|---------------|----------------|-------|
| Intent parse | ADB core (new thin) | NL/话术 → IntentEnvelope；歧义 fail-closed；pytest 契约 | nl-intent-envelope |
| Intent confirm UX | Hermes skill (thin) | 展示 envelope；人确认后再调 approve/dispatch | nl-intent-envelope |
| Assign bridge | ADB assign (existing) | envelope 可喂给 candidates；仍非自动派工 | nl-intent-envelope |
| Worker binding | Hermes adapter + task body schema | stage → Beacon skill + runner（codex/coding profile） | worker-beacon-binding |
| Stage policy | ADB service | ENABLED_STAGES 扩展策略（goal 待 ack） | worker-beacon-binding |
| Kanban ops | ADB boards/fleet + Hermes CLI | 看板状态/关注项/表格反馈 | kanban-ops-nl |
| Beacon read | Beacon adapter (read-only) | 版本、最新需求摘要 | beacon-read-surface |
| Ops digest | Hermes cron + ADB render | 定期 digest → 飞书载荷 | ops-digest-cron |
| Knowledge digest | Personal Brain + cron/skill | 知识库梳理反馈（ADB 外） | knowledge-curation-digest |

## Data contracts (planned)

### IntentEnvelope (L1)

```text
schema_version, utterance_hash,
action (dispatch|approve|fleet|boards|assign|awaiting|beacon_status|curate_kb|unknown),
project_slug | null, project_candidates[],
stage | null, feature | null,
confidence, ambiguity_codes[],
requires_confirmation, requires_approval
```

Fail-closed: `project_slug` 歧义或缺失且 action 需要项目 → `blocked` + `resume_action=clarify_project`。

### WorkerBinding (L2)

```text
stage → { beacon_skill_or_command, runner_profile, admission_required, approval_gate }
```

## Non-modules

- Feishu OpenAPI client inside ADB
- Hermes SQLite reader
- Embedded NLU model service
- Auto-release orchestrator
