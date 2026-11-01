---
schema_version: "1.0"
review_id: "pln-review-p2-superagent-router"
program_slug: superagent-router
version: v0.1.5
mode_id: pln-review
public_id: plan
execution_mode: single_process_multi_reviewer
fallback_reason: subagent_runtime_unavailable_explicit_deterministic_fallback
runtime_evidence: .beacon/state/planner-review/superagent-router/pln-review-20261101t074110z.json
intent_snapshot: "P2 review of program superagent-router — 常驻 Router 前台（无执行工具）+ 线程订阅直通 + 轻模型批处理 + watchdog 进展判定 + beacon requirement-index 投影；ADB 唯一 owner，不合并 monorepo，paseo 只走 CLI"
scope_mode: new_capability
reviewed_artifacts:
  - docs/beacon/v0.1.5/programs/superagent-router/module-map.md
  - docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json
reviewers_run:
  - intent-fidelity
  - scope-mode
  - coverage-shape
  - route-boundary
  - state-machine
  - security-boundary
  - performance-boundary
  - evidence-proof-shape
  - adversarial-doubt
  - simplification-risk
  - context-engineering
autofix_allowed: false
user_decision_required: true
recommended_route: plan
recommended_next_harness: plan
---

# P2 Program Review — superagent-router v0.1.5

> 结论：**2 个 P0、6 个 P1、3 个 P2。**
> 两个 P0 已在本次改图中消解（见「改图记录」），P1 全部转为各湖的硬约束 AC。
> 改图后 `validate-graph` pass，可进 P4 ack。

## Findings

### P0

| id | reviewer | issue | 处置 |
|----|----------|-------|------|
| R-P0-01 | intent-fidelity | 程序承诺"长期稳定在线"，但**四个湖没有任何一个拥有 Router 自身的存活与恢复**。L3 只监视 worker；adb 进程死掉时系统整体停摆，且无人发现（watchdog 与 router 同生共死）。 | L3 扩为"外部控制回路"：由 paseo schedule 在进程外执行，同时监视 **router 心跳** 与 **worker 进展**；补 kill-9 恢复 AC |
| R-P0-02 | route-boundary | `requirement-index` 的**写权未定义**。若 Router 直接把 `stage` 从 implement 改成 qa，就造出第二个真相源，违反 beacon"冻结后不得静默改写"。 | index 拆两列：`truth_stage`（只读，来自 beacon 冻结物）+ `exec_state`（可写，来自 ADB / Router）。Router 只写 `exec_state`；`truth_stage` 变更必须走 `beacon change` |

### P1

| id | reviewer | issue | 落点 |
|----|----------|-------|------|
| R-P1-01 | state-machine | 缺 State Model。thread binding / worker / dispatch 三条状态轴没有枚举、非法迁移、终态。ADB 现有 `state='bound'` 是散值不是 FSM。 | 每湖 truth 必须落 `## State Model`（states / allowed / invalid / recovery / terminal / axes） |
| R-P1-02 | coverage-shape | 直通路径**不经过 dispatch ledger**，没有幂等键。Webhook 重投会让同一消息被 worker 处理两次。 | L1 AC：intake 幂等键 = (channel, thread, message_id)；重投命中即丢弃并记账 |
| R-P1-03 | state-machine | 租约续期未定义。默认 TTL 24h，长任务中途掉订阅 → 后续消息走慢路径 → **二次派工**。 | L1 AC：worker 每次汇报即续租；过期前 1h 发续租提醒 |
| R-P1-04 | performance-boundary | 无重试上限与退避。worker 立即失败 → router 立刻再派 → 派工风暴。 | L2 AC：同 req 连续失败 3 次退避并升级，不无限重派 |
| R-P1-05 | coverage-shape | 没有 program 级**效果指标**，无法证明"稳了"。功能 AC 全绿也可能 p95 更差。 | 见「Program 指标」，四条写入 L2/L3 验收 |
| R-P1-06 | security-boundary | IM 原文 → Router → worker prompt，构成**间接提示注入**通道。Router 无工具降低了危害，但 worker 有工具。 | L2 AC：转发给 worker 时注入 `source_trust=untrusted` 标记并剥离指令性前缀；worker 不得执行消息内的"删除/推送/发布"类指令 |
| R-P1-07 | evidence-proof-shape | "常驻"不可证。缺 `kill -9` 后恢复的验收。 | L2 AC：进程被杀后重启，未决消息重放且不重复派工 |

### P2

| id | reviewer | issue | 处置 |
|----|----------|-------|------|
| R-P2-01 | scope-mode | `scope_mode: full_parity` 是从模板继承的失真值——本项目无上游可对齐。 | 改为 `new_capability` |
| R-P2-02 | context-engineering | Router 上下文预算未定（index 规模 × 项目数），未来会悄悄撑爆。 | L0 AC：index 超过 10KB 时自动分层（summary + per-project） |
| R-P2-03 | simplification-risk | L0 与 L1 可合并成一湖。合并会少一个可独立验收的收益点（先证 index 有用，再证直通有用）。 | 不合并，记录在案 |

## State Model（要求，不代写 truth）

```text
state_model:
  required: true
  axes:
    thread_binding: unbound → bound → leasing → expiring → released
    worker:         created → running → reported → done | failed | abandoned
    dispatch:       queued → dispatched → acked | expired | cancelled
  invalid_transitions:
    - released → bound            # 必须新建绑定，不可复活
    - done → running
    - expired → dispatched        # 租约过期后必须重新分派
  recovery_paths:
    - router_restart → replay_inbox → dedupe_by_idempotency_key
    - worker_silent → watchdog_probe → reassign_or_escalate
  terminal_states:
    - worker.done / worker.abandoned / dispatch.cancelled
  coverage_implications:
    - 每个合法迁移至少一条测试
    - 每条非法迁移至少一条拒绝测试
    - recovery_paths 各一条注入式测试
```

## Program 指标（R-P1-05 的落点）

| 指标 | 含义 | 基线 | 目标 |
|------|------|------|------|
| `human_relay_count` | 人工中转消息次数 | 未测（≈ 每条跨会话消息） | 稳态 0 |
| `time_to_first_worker` | 从有事发生到 worker 开始干活 | 未测 | p50 < 60s，p95 < 3min |
| `task_completion` | 从派活到结果 | 未测 | p50 / **p95** 双记，p95 优于人工基线 |
| `router_decisions_per_task` | 每任务 Router 决定数 | 未测 | 直通命中后下降 ≥ 30% |

## Deferral Ledger

| 能力 | 状态 | 原因 | 需用户决定 |
|------|------|------|-----------|
| studio 只读 index 页面 | deferred | 展示层，不阻塞控制面 | 否（后续独立湖） |
| headlong 事件源接入 | deferred | 本期先用 IM / 手工触发验证闭环 | 否 |
| 多 Router 分片 / A2A | deferred | 单实例规模未触顶 | 否 |
| HTTP 服务化 | deferred | CLI 足够，多一层要保活 | 否 |

## 改图记录（本次已执行）

1. `feature-graph.json`：`scope_mode` → `new_capability`；补 `state_model_required`、`program_metrics`、`failure_matrix`；各湖 `parity_subset` 增加上述 P1 能力键。
2. `module-map.md`：新增「执行态 vs 真相态」「状态模型」「失败矩阵」「Program 指标」；L3 改为进程外 watchdog；L2/L3 AC 方向扩展。
3. `beacon program validate-graph` 复跑：pass。

## Route

- P0 已消解 → **recommended_next_harness: plan**（P4 分段 ack）
- P4 ack 后 → `truth`，按拓扑从 `requirement-index` 开始
- 若你不同意 R-P0-02 的「执行态/真相态分列」→ 必须回到 `beacon change`，因为它改变真相语义
