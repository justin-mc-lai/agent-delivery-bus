---
schema: beacon-program-research-v0
program_slug: adb-nl-stable-ops
version_hint: v0.0.3 (proposed; not yet in governance)
plan_mode: interactive
status: p4_acked
user_decision: ack_三节
first_lake: nl-intent-envelope
language: zh
updated: 2026-07-30
---

# Research: ADB NL-stable ops (BR-0)

## Intent one-liner

让自然语言经 Hermes 稳定触发 ADB：意图解析 → 准确项目路由 → 审批门控派工 → Hermes 调度本机 agent（如 Codex）跑 Beacon skill（plan / goal 等）；并定期反馈看板进度、Beacon 版本/最新需求、知识库梳理摘要。

## Lake or ocean

**海（ocean）**。覆盖面横跨：NLU/意图契约、Kanban 运维、worker 绑定、cron 摘要、Beacon 只读查询、知识库梳理。不得单湖一次煮干。

## Baseline already shipped

| Lake / surface | Version | What it gives | What it does NOT |
|----------------|---------|---------------|------------------|
| `delivery-bus-mvp` | v0.0.1 | registry / doctor / approve / idempotent dispatch / Hermes boards / reconcile / skill | NL intent, cron digest, goal stage, auto-release |
| `memory-adapter-auto-assign` | v0.0.2 | scoped recall, assign candidates, Feishu *payload* for awaiting | hard keyword router, auto-push Feishu, NL→dispatch |

v0.0.1 program explicitly deferred: 定时调度、Web UI、Orca 主调度器、深度知识采集。

## Authority boundaries (must keep)

```text
NL / Hermes chat     → intent + UX only (not delivery verdict)
ADB                  → resolve / preflight / approve / dispatch ledger / reconcile
Hermes Kanban + cron → task persistence + periodic trigger
Beacon               → truth / freeze / QA / version materials
Personal Brain       → knowledge body + curation content
Local agent (Codex)  → executes Beacon skills inside admitted workspace
```

ADB must stay control-plane: no Hermes private DB, no auto-release, no embedding NLU model as core.

## Gap map (utterance → current)

| Desired outcome | Current | Gap |
|-----------------|---------|-----|
| 自然语言稳定派发 | Skill 自由发挥 + CLI 结构化 | 缺稳定 IntentEnvelope + 歧义 fail-closed |
| 准确项目分配 | registry slug/alias + assign scorer | 缺 NL→候选→确认 的强制契约 |
| Kanban 管理 | `boards status/sync` 只读偏多 | 缺运维动作面（列/关注项）与 NL 映射 |
| Hermes→Codex→Beacon skill | task body 泛化提示 | 缺 stage→skill/runner 绑定契约 |
| `goal` 长程 | ENABLED_STAGES 无 goal | 缺 stage 扩展 + 门控策略 |
| 定期看板反馈 | Hermes cron 存在；ADB 无 digest | 缺 cron job 模板 + fleet 渲染 |
| Beacon 版本/最新需求 | doctor 查 context；无摘要命令 | 缺只读 query 面 |
| 知识库定期梳理反馈 | personal-brain / personal-delivery-bus | 应在 Brain+cron，不进 ADB 核心 |

## Approaches (BR-方案)

### A — Skill-only（最快）

只强化 Hermes/`personal-delivery-bus` 话术与示例。

- 利：零 core 改动，飞书立即可试
- 弊：意图不稳定；难测；易绕过 approve

### B — ADB Intent CLI + 薄 Hermes skill（推荐）

新增只读/结构化面：`intent parse` → IntentEnvelope；dispatch 仍只吃结构化参数；Hermes skill 强制「先 parse 再确认再 approve/dispatch」；cron 调 `fleet`/`approvals awaiting`/`beacon status` 产摘要。

- 利：稳定性可测；权威边界清晰；与现有 approve FSM 对齐
- 弊：需 2–3 个湖分期交付

### C — ADB 内嵌 NLU 服务

- 利：端到端可控
- 弊：违背「薄控制面」、运维重、与 Hermes 职责重叠 → **defer / 拒绝**

## Draft lakes (parity rows; MVP marks phase only)

1. **nl-intent-envelope** — NL/飞书话术 → 结构化 Intent（project/stage/feature/action）；歧义拒绝；pytest 契约
2. **worker-beacon-binding** — Hermes task → coding profile / Codex（或等价）→ Beacon skill 映射（含 plan；goal 是否入湖待决）
3. **kanban-ops-nl** — fleet/boards 运维动作 + 表格反馈契约
4. **ops-digest-cron** — Hermes cron 定期 digest（进度看板 + 待拍板）到飞书
5. **beacon-read-surface** — 版本/最新需求只读摘要（经 Beacon 公开 CLI）
6. **knowledge-curation-digest** — Brain 梳理摘要（ADB 外；cron+skill）→ **可 defer 为 support lake**

## Recommended first lake

**已确认：`nl-intent-envelope`**（用户选择：先完整 feature-graph，再首湖 intent）。

完整拓扑见 `feature-graph.json`；parity/deferral 见 `program-parity-matrix.md`。

## Non-goals (program-level)

- 自动 release / merge / deploy
- 读取 Hermes/Beacon 私有 DB
- 绕过 approve 的 implement/freeze
- 把 Personal Brain 正文写入 ADB SQLite
- Web UI

## Route

- P4 acked (`user-ack-三节-2026-07-30`)
- **Next harness: `truth`** for first lake `nl-intent-envelope`
- Prerequisite: register `v0.0.3` in project-version-governance (proposed)
- Do **not** implement from this research; planner stops at harness boundary
