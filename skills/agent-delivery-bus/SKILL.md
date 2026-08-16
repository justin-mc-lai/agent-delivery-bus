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
bin/adb projects list --numbered --json
bin/adb projects resolve --slug <slug> --json
bin/adb projects resolve --index <N> --json
bin/adb projects register --slug <slug> --class <platform|managed|knowledge> --repo <path> [--aliases a,b] [--truth-gate <g>] [--executor <e>] [--binding-profile <p>] --json
bin/adb projects delete <index|slug> --yes --json   # soft delete: archive, keep index
bin/adb projects restore <index|slug> --json
bin/adb workflow list --json
bin/adb workflow install --name <name> --preset <superpowers|openspec> --json
bin/adb workflow show <name> --json
bin/adb workflow remove <name> --yes --json
bin/adb workflow ingest --source <path|url> --name <name> --json
bin/adb workflow draft apply --name <name> --request-json <req> --response-json <resp> --json
bin/adb workflow draft show --name <name> --json
bin/adb workflow confirm --name <name> --yes --json
bin/adb workflow verify --name <name> --project <slug> --json
bin/adb workflow trace --name <name> --json
bin/adb workflow debug --name <name> --json
bin/adb intent keywords --json
bin/adb doctor --project <slug> --json
bin/adb intent parse --utterance "<natural language>" --json
bin/adb intent parse --utterance "<natural language>" --project <slug> --json
bin/adb assign candidates --project <slug> --stage <stage> --feature <feature> --json
bin/adb dispatch --project <slug> --stage <plan|implement|qa|freeze> --feature <feature> --dry-run --json
bin/adb approve --actor <actor> --project <slug> --stage <implement|freeze|release> --feature <feature> --json
bin/adb dispatch --project <slug> --stage <stage> --feature <feature> --approval-token <token> --json
bin/adb task show <dispatch-id> --json
bin/adb reconcile <dispatch-id> --json
bin/adb reconcile-loop [--interval 60] [--max-runs 0|--once] [--project <slug>] --json
bin/adb reconcile-loop cron-template --json   # silent Hermes cron tick script
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

## Session routing resolution order

聊天里用 `adb <项目编号> <业务描述>` 触发调度时，目标 agent 按以下决议顺序确定（绝不静默跳级）：

1. 显式 `--target-executor`（explicit）
2. 该通道线程的会话绑定 target（binding，`adb session bind`）
3. 项目 `metadata.executor_policy.stages[<stage>]`（policy）
4. 通道默认（channel_default → hermes coding）

每次派发默认独立目标会话（`<target>-<digest[:12]>`，防止同线程并发任务串话）；
固定会话用 `--target-session fixed:<id>` 时加互斥 lease，忙则 `session_busy`。

### 会话路由的稳定性契约（v1.6.11+）

- 会话身份只由 `channel + channel_thread + actor_id` 构成；`host_session`
  仅作审计字段，不进身份键——同线程换宿主会话不丢绑定。
- 决议出的 target 会真正驱动适配器选择：`pi → PiExecutorAdapter`，
  `codex/claude/coding → HermesAdapter` 的对应 assignee profile；旧式
  resolver 若解析结果与绑定 target 不一致，fail-closed 返回
  `executor_mismatch`，绝不静默降级。
- 固定会话的 ADB 句柄直接作为 pi 的 `--session-id` 精确项目会话 ID；
  pi 异步执行，先落 `running` receipt 再更新 `done/failed`，超时/失败
  均有账可对（`pi_timeout` / `pi_dispatch_failed` / `pi_runner_failed`）。
- 业务幂等键只含 `schema/project/repo/docs_version/stage/feature/
  binding_profile`；渠道、actor、host_session、target 等路由字段不入键，
  同一业务任务跨线程重试复用同一 dispatch。
- 结果回传走独立 ChannelAdapter（`hermes send`），不再依赖执行适配器；
  pi 只负责执行，不承担渠道交付。
- 自动化回执：`adb reconcile-loop` 定时对账 dispatched/reconciling 派发，
  完成/失败自动回发原话题；`reconcile-loop cron-template` 输出静默
  Hermes cron 脚本（本机已注册 `adb-reconcile`，每分钟一轮）。单个
  dispatch 对账失败（如项目已注销）会被归置为 blocked，不会中断整轮。

Project lifecycle: `register` auto-assigns index = max+1 (never reused);
`delete` soft-archives (dispatchable=false, index kept); `restore` reactivates.
Project management writes also require explicit human confirmation.

Workflows are enforced skill pipelines. Beacon lifecycle
(plan/truth/implement/qa/freeze/goal) is ADB's first-party capability; presets
are open-source peer skill workflows (`superpowers`, `openspec`). Any repo can
be adapted generically: `ingest` inventories read-only and emits an analysis
request; the HOST AGENT (the agent running adb) fills the response; adb
validates → draft → human confirm → install → verify → bind. JSONL traces are
kept for debug/replay. Channels share one canonical keyword map
(`adb intent keywords`). Dispatched tasks force-load the bound skill
(`hermes --skill ...`); preflight blocks when the device lacks the skill.

Feishu chat playbook (numbered project table + trigger words + dialog flow):
[references/feishu-playbook.md](references/feishu-playbook.md). Pinnable
cheatsheet: `docs/feishu-dispatch-cheatsheet.md`.

## Required reporting

Return the resolved project, stage, feature, dry-run/real mode, preflight result,
dispatch id, executor task id when present, current state, reason code, and next
safe action.

Never include approval tokens in logs, task bodies, or later status responses.
