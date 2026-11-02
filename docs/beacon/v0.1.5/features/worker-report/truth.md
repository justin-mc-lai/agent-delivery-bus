---
slug: worker-report
version: v0.1.5
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v0.1.5/features/worker-report/truth.md
  tests: docs/beacon/v0.1.5/features/worker-report/tests.md
  tasks: docs/beacon/v0.1.5/features/worker-report/tasks.md
  evidence: docs/beacon/v0.1.5/features/worker-report/evidence.md
---

# Requirement Truth: worker-report

## 人话

现在派出去的活干完了，系统不知道。
所以看门狗把"干完了"当成"卡住了"，索引里的执行状态永远是 unknown。
这个湖补上那条回执：定时去看一眼 worker 是不是从"在干活"变成了"闲下来"，
一变就记一笔"完成"，并把执行状态写回索引能读的地方。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "顺序修复以上缺口、断点，不绕过不回避不拖延"
goal: "worker 完成后系统能知道：写回执、更新绑定、产出 exec_state，让看门狗不再误报、索引能显示执行进度。"
non_goals: "不改 worker 内部；不要求 worker 主动调用 CLI；不接飞书；不改 router 决策。"
constraints: "Go 放 cmd/adbd（stdlib only）；状态只在本地文件；可注入 runner；真相只读。"
scope_mode: "lake"
```

## 状态模型

```text
binding: bound -> working -> done | stalled
report:  none -> recorded
exec:    unknown -> running -> done | failed
```

- `working`：派发后观察到 agent 处于 running。
- `done`：观察到 agent 从 running 变成 idle。
- `stalled`：超时仍无状态变化（watchdog 负责，不重复）。

终态：`done`、`stalled`。

## 用户旅程

1. route 派活后，绑定进入 `bound`，并记下 `dispatched_at`。
2. 每 2 分钟 `adbd reconcile --once` 读一次 paseo 的 agent 状态。
3. 第一次看到 running → 绑定转 `working`。
4. 后来看到 idle → 绑定转 `done`，写一条回执，写一份 exec_state。
5. 看门狗看到 `done` 不再告警；索引显示这条需求 `exec_state=done`。
6. 同一 agent 重复 reconcile 不会重复写回执。

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | goal：系统要知道干完了 | truth.md#Acceptance-Criteria |
| AC-002 | goal：写回执 | truth.md#Acceptance-Criteria |
| AC-003 | goal：索引能显示执行进度 | truth.md#Acceptance-Criteria |
| AC-004 | goal：索引合并 | truth.md#Acceptance-Criteria |
| AC-005 | 稳定性：不重复 | truth.md#Acceptance-Criteria |
| AC-006 | 稳定性：失败不静默 | truth.md#Acceptance-Criteria |
| AC-007 | 用户要求：看门狗不再误报 | truth.md#Acceptance-Criteria |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `adbd reconcile --once` 遍历所有持租约的绑定，通过可注入的 status runner 读取每个 agent 的状态（`paseo inspect <id> --json` 的 `Status`）；输出 `checked=<n> done=<m> running=<k>`；无绑定时输出 `checked=0` |
| AC-002 | 观察到 `running → idle` 迁移时：绑定 `state` 置 `done`，向 `.beacon/adbd-reports.jsonl` 追加一行，字段固定 `thread`/`agent_id`/`dispatched_at`/`finished_at`/`status`；同一 agent 只写一次 |
| AC-003 | 同时写 `.beacon/adbd-exec-state.json`：`{"<req_id>": {"exec_state","worker","thread","updated_at"}}`；`skill:<project>:<slug>` 映射为 `req:<project>:<slug>`；非 skill thread 不产生条目；文件原子写入 |
| AC-004 | `adb index generate` 读取 `.beacon/adbd-exec-state.json`（若存在）：对应 `req_id` 的 `exec_state` 取文件值，缺失的仍为 `unknown`；`truth_stage` 不受影响；文件不存在时行为不变 |
| AC-005 | 重复 `reconcile` 不产生重复回执行；绑定已 `done` 的线程不再被统计为 done（输出里 `done` 只计本次新完成的） |
| AC-006 | paseo 不可用（二进制缺失、非零退出、输出不可解析）时输出 `reason_code=paseo_unavailable`，退出码 2，且不改动任何绑定、回执、exec_state |
| AC-007 | 绑定状态为 `done` 的线程不再被 watchdog 判为 stalled（watchdog 读取绑定状态，`done` 直接跳过）；测试用注入时钟验证不再出现 `stalled` |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_paseo_bin` | `paseo` | 读取 agent 状态用的二进制 |

## 禁止的迁移（人工维护，内核不校验）

- illegal: done -> working
- illegal: done -> bound

## Non-goals（冻结边界）

- 不要求 worker 主动上报（靠观察 agent 状态，不靠配合）
- 不改 worker 内部行为
- 不接飞书、不做指标埋点
- 不改路由决策

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 回执靠 worker 主动上报，还是靠观察 agent 状态？ | 观察 paseo agent 状态（running->idle），不依赖 worker 配合 | answered | 观察 paseo agent 的 Status，running->idle 即视为完成。 | human |
