---
slug: vision-flywheel
version: v0.0.4
status: frozen
revision_id: R1
language: zh
domain_required: true
domain_kind: business
ux_required: false
package_maturity: filled
parser_contract: beacon-feature-package-v2
materials_status: current
truth_source_model: intent_first
git_canonical_branch: main
program: personal-production-flywheel
promotion_ref: user-ack-愿景立项-2026-08-04
canonical_refs:
  prd: docs/beacon/v0.0.4/features/vision-flywheel/truth.md
  user_story: docs/beacon/v0.0.4/features/vision-flywheel/truth.md
  test_case: docs/beacon/v0.0.4/features/vision-flywheel/tests.md
---

# Requirement Truth: vision-flywheel（个人 AI 生产飞轮 · 调度心跳层）

## 人话

把「个人 AI 生产飞轮」愿景正式立项。本版本（v0.0.4）只做 ADB 侧的调度底座：**让定时任务有登记、有配额、有证据**——ADB 不自己造定时器，而是把「该不该跑、跑一次花多少配额、跑完有没有证据」管起来；到点的触发交给 hermes cron。反馈回路、选题池、pi 执行器、多格式创作是后面的湖，本包不碰。

## User Intent

> 飞轮愿景立项：先给 ADB 装上「调度心跳层」——定时任务登记 + quota 记账 + 心跳证据对账。

## User Intent Snapshot

```yaml
lake_or_ocean: 海
language: zh
scope_mode: ocean_split
feature: vision-flywheel
program: personal-production-flywheel
depends_on: [ops-digest-cron, nl-intent-envelope, worker-beacon-binding]
cron_owner: hermes
auto_dispatch_from_heartbeat: false
quota_enabled: true
```

## Alignment Surface（本 revision 必煮干）

1. **调度条目注册**：`adb schedule register` 把定时任务登记进 ADB 调度注册表（slug/command/engine/cron 表达式/配额上限），**不内嵌 daemon**，触发委托 hermes cron（与 ops-digest-cron 同构）。
2. **should-run 确定性判定**：`adb schedule should-run <slug>` 按门卫链（quota 门卫 → 健康门卫）输出 run/blocked，无 LLM 判定。
3. **quota 记账**：每次被触发的执行计 slot（来源 heartbeat/controller 白名单），quota 耗尽 → `throttled`；evidence 落盘后才计配额（spend-after-validated-writeback）。
4. **心跳证据对账**：每次心跳运行追加写 dispatch ledger（事件流），复用 truth-gate closure 对账逻辑；缺证据保持 `reconciling`。

## Phased Backlog（显式不在本包 closure）

| ID | 内容 | revisit |
|----|------|---------|
| PB-FLY-1 | 反馈回路：published-track 指标采集激活 + analyze-data 实现 + brain-writeback 回流 | 跨项目（selfmedia-creator 侧），v0.0.5 或独立立项 |
| PB-FLY-2 | 选题池化与证据化（知识来源 + 市场信号 + 状态） | 知识环，v0.0.5 |
| PB-FLY-3 | pi agent 执行器（driver_pi：beacon truth → pi 长程交付） | 产品环，v0.1.x |
| PB-FLY-4 | 多格式矩阵（vlog / 漫剧 openmontage 从 scaffold 到能力） | 内容环，v0.0.5+ |
| PB-FLY-5 | 平台补齐（toutiao / bilibili-video / douyin publish） | sync-ai 侧，v0.0.5 |
| PB-FLY-6 | 矩阵自动调度（creator DL-011 解除） | 内容环，v0.1.x |
| PB-FLY-7 | 多 agent 并发控制（硬 lease / write_scopes） | 调度层深化，v0.0.6 |

## Deferral Ledger

| ID | Item | user_decision | note |
|----|------|---------------|------|
| D1 | feedback-loop-in-adb-core | rejected | 反馈回路归 selfmedia-creator 侧，ADB 只提供调度与记账 |
| D2 | adb-embedded-cron-daemon | rejected | 延续 ops-digest-cron D 方向：触发委托 hermes cron |
| D3 | auto-dispatch-from-heartbeat | rejected | 心跳只报告/记账，不自动派工（与 AC-FLY-007 对齐） |
| D4 | auto-release | rejected | global |
| D5 | pi-agent-executor-in-v004 | accepted_defer | driver_pi 入 v0.1.x（PB-FLY-3） |

## 用户旅程

1. 操作者（人或 hermes cron）触发定时任务 → ADB 按注册条目查 should-run。
2. should-run 通过 → 执行（现有派发链路）→ 计 quota slot → evidence 落盘对账。
3. quota 耗尽 / 健康门卫拦截 → blocked + 稳定 reason_code + resume_action。
4. 失败时 fail-closed，不静默跳过，不自动重试超限。

### 失败旅程

- 未注册条目 / 预检失败：blocked + 稳定 reason_code。
- quota 耗尽仍尝试执行：blocked（throttled），提示人工放行或调配额。
- 心跳尝试自动派工/自动 approve：illegal fail-closed。

## Domain Model

| Entity | Key fields | Invariant |
|--------|------------|-----------|
| ScheduleEntry | slug, command, engine, cron_expr, quota_limit | 注册表单一真值；不内嵌 daemon |
| QuotaLedger | slug, window, slots_spent, slots_allowed, next_eligible_at | spend-after-validated-writeback |
| HeartbeatRun | entry_slug, status, evidence_refs, quota_spent | 追加式事件；缺证据保持 reconciling |

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-FLY-001 | covered |
| INT-002 | user | must | AC-FLY-002 | covered |
| INT-003 | user | must | AC-FLY-003 | covered |
| INT-004 | user | must | AC-FLY-004 | covered |
| INT-005 | user | must | AC-FLY-005 | covered |
| INT-006 | user | must | AC-FLY-006 | covered |
| INT-007 | user | must | AC-FLY-007 | covered |

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-FLY-001 | `adb schedule register` 登记定时任务（slug/command/engine/cron_expr/quota_limit）到调度注册表；重复 slug 幂等更新；未注册引擎拒绝 |
| AC-FLY-002 | `adb schedule list` 输出全部注册条目（含 quota 状态）；`adb schedule show <slug>` 单条目 |
| AC-FLY-003 | `adb schedule should-run <slug>` 确定性门卫链（quota 门卫 → 健康门卫），输出 run/blocked + reason_code；无 LLM 判定 |
| AC-FLY-004 | quota 记账：被触发的执行按来源（heartbeat/controller 白名单）计 slot；配额耗尽 → throttled + blocked；evidence 落盘后才计配额 |
| AC-FLY-005 | 心跳运行追加写 dispatch ledger（事件流：entry_slug/status/evidence_refs/quota_spent），可审计 |
| AC-FLY-006 | 心跳证据对账：复用 truth-gate closure 逻辑，缺证据保持 reconciling；证据齐才 completed |
| AC-FLY-007 | 心跳不得自动 approve/dispatch（illegal）；违规进入 blocked |

## Domain FSM — ScheduleHeartbeat

| State | From | Guard |
|-------|------|-------|
| idle | — | hermes cron or CLI trigger |
| checking | idle | entry registered + quota gate |
| running | checking | should-run pass |
| done | running | evidence validated |
| blocked | checking/running | quota exhausted / health gate / illegal |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| idle | tick | cron_or_cli | checking | resolve_entry |
| checking | should_run_pass | quota_ok_and_healthy | running | dispatch_execution |
| checking | should_run_block | quota_exhausted_or_unhealthy | blocked | emit_reason |
| running | ok | evidence_validated | done | record_heartbeat |
| running | fail | always | blocked | emit_reason |
| running | auto_dispatch_attempt | always | blocked | reject_illegal |

## Illegal transitions

- checking → dispatch · TC-FLY-ILL-001
- running → approve · TC-FLY-ILL-001
- idle → running（跳过 should-run）· TC-FLY-ILL-002

## Public CLI Contract

- `adb schedule register --slug <s> --command <cmd> --engine hermes --cron "<expr>" --quota-limit <n>`
- `adb schedule list [--json]`
- `adb schedule show <slug> [--json]`
- `adb schedule should-run <slug> [--json]`
- Hermes cron 触发模板（文档+fixture，与 ops-digest-cron 同构）

## Non-goals

- ADB 内嵌 cron daemon / 自建调度器
- 根据心跳自动派工 / 自动 approve
- 反馈回路实现（归 selfmedia-creator 侧）
- pi agent 执行器（v0.1.x）
- 多格式创作 / 平台补齐（Phased Backlog）

## Freeze readiness

- [x] Alignment / Phased / Deferral 已全部拍板，无未决项
- [x] FSM + Illegal 已具体化
- [x] AC↔TC Command+Assertion 齐全
- [x] 海级拆分：本包仅煮干调度心跳层，其余环显式进 Backlog
