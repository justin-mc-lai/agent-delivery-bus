---
program_slug: superagent-router
version: v0.1.5
plan_mode: interactive
scope_mode: full_parity
lake_or_ocean: 海
program_phase: P1
---

# Module Map — superagent-router (v0.1.5)

## 一句话定位

把 ADB 从「受控派工命令行」升级为「常驻前台」：只接活、分活、汇报，不干活。
Session 变成实现细节，人不再记 5 个会话。

## 仓库拓扑（非 monorepo）

```
                  ┌──────────────────────────────┐
                  │  agent-delivery-bus  (owner)  │  唯一写代码 / 唯一发版 / 唯一跑 CI
                  │  v0.1.5 superagent-router     │
                  └──────┬──────────────┬─────────┘
                         │ 只读          │ 调 CLI / daemon
                         ▼              ▼
                      beacon          paseo
                    （定真相）        （出手干活）
                         ▲              ▲
                         │ 只读展示      │ 只发事件
                      studio        headlong
```

| 仓库 | 本期角色 | 是否改动 | 边界 |
|------|----------|----------|------|
| agent-delivery-bus | owner / 控制面 | ✅ 本版本全部代码 | 唯一有权写 router |
| beacon | 真相权威 | ❌ 只读 | 不改 truth 格式，不写 `.machine/` |
| paseo | worker plane | ❌ 不改 | 只走 CLI/daemon，不 import、不 fork |
| studio | 展示 | ❌ 不改 | 后续独立湖再接只读 index |
| headlong | 事件源 | ❌ 不改 | 本期不接入，先用手工/IM 触发 |

**判定：这不是 monorepo，是 1 个 owner + 3 条依赖边界。** 5 个仓库各自发版，互不阻塞。

## 模块图

```
入口（skill / IM / cron）
        │
        ▼
[ L1 intake ] ── 命中 agent_sessions.channel_thread ──▶ 直通 worker.followup()   Fast Path
        │ 未命中
        ▼
[ L2 route ]  ── 常驻循环 + 积压批处理 + 轻模型 ──▶ 派活 / 回话 / 忽略 / 升级
        │                                              │
        │                                              ▼
        │                                        paseo agent run
        ▼                                              │
[ L3 watchdog ] ◀────────── 状态 + 进展信号 ───────────┘   Slow Path / 外部控制回路

数据面：
  [ L0 requirement-index ] ── 全量需求一览（几 KB）──▶ L2 只读这一个文件
                                   │
                                   └─ 内联一跳引用 ID，Worker 按需展开（链式召回）
```

## 湖清单（拓扑顺序）

| # | slug | 做什么 | 依赖 | AC 方向 |
|---|------|--------|------|---------|
| 1 | `requirement-index` | beacon 真相包 + studio pool → 单一 `requirement-index.json`；**`truth_stage` 只读 / `exec_state` 可写** | — | 6 条：schema 稳定、幂等生成、缺失不静默、单文件可解析、双列写权隔离、超 10KB 自动分层 |
| 2 | `intake-fastpath` | `adb intake` 入口；按 (channel, channel_thread, actor) 命中绑定即直通；租约过期回退慢路径 | `requirement-index`（共用 req_id 命名） | 7 条：直通命中、租约过期、汇报即续租、跨 thread 不串、未知来源拒绝、审计落账、**intake 幂等键** |
| 3 | `router-loop` | 常驻循环 + 积压合并 + 轻模型四选一（route/reply/ignore/escalate）；审批继承 ADB 现有一次性审批 | `intake-fastpath`（表与事件流） | 9 条：批处理、四类决策、**运行时枚举工具集为空**、decision trace、幂等、kill -9 后重放不重派、100 字上限、**重试退避**、**不可信内容标记** |
| 4 | `watchdog` | **进程外**（paseo schedule）定时判「有无实质进展」，同时看 worker 与 **router 心跳**；无事输出 NOTHING | `router-loop`（状态） | 5 条：进展判定、router 心跳、静默、介入留痕、连续失败升级 |

## 执行态 vs 真相态（P0 修复）

这是全program最重要的一条边界：

| 列 | 谁能写 | 来源 | 语义 |
|----|--------|------|------|
| `truth_stage` | ❌ Router 不能写 | beacon 冻结物 | 需求处于 plan/truth/implement/qa/release |
| `exec_state` | ✅ Router / ADB 可写 | worker 汇报 | 这条需求**执行到哪一步** |

Router 只推进 `exec_state`。要改 `truth_stage`，必须走 `beacon change` → refreeze。
否则 ADB 就成了第二个真相源——这正是 beacon 明令禁止的。

## 状态模型（每个湖的 truth 必须落，planner 不代写）

```text
state_model:
  required: true
  axes:
    thread_binding: unbound → bound → leasing → expiring → released
    worker:         created → running → reported → done | failed | abandoned
    dispatch:       queued → dispatched → acked | expired | cancelled
  invalid_transitions:
    - released → bound        # 必须新建绑定，不可复活
    - done → running
    - expired → dispatched    # 租约过期后必须重新分派
  recovery_paths:
    - router_restart → replay_inbox → dedupe_by_idempotency_key
    - worker_silent  → watchdog_probe → reassign_or_escalate
  terminal_states:
    - worker.done / worker.abandoned / dispatch.cancelled
```

## 失败矩阵

| 失败 | 后果 | 对策（落在哪个湖） |
|------|------|--------------------|
| paseo worker 静默死亡 | 任务无声卡住 | L3 watchdog 探测 → 重派或升级 |
| Webhook 重投 | 同一消息被处理两次 | L1 intake 幂等键 |
| 长任务租约到期 | 中途消息被派给第二个 worker | L1 汇报即续租 + 过期前 1h 提醒 |
| worker 立即失败 | Router 立刻再派 → 派工风暴 | L2 连续 3 次退避 + 升级 |
| adb 进程被杀 | 系统整体停摆 | L3 进程外心跳 + L2 重启重放 |
| IM 内容注入指令 | Worker 有工具，可能被利用 | L2 `source_trust=untrusted` 标记 + 剥离指令前缀 |

## Program 指标（不是功能 AC，是「稳没稳」的证明）

| 指标 | 基线 | 目标 |
|------|------|------|
| `human_relay_count` | 每条跨会话消息都是人转 | 稳态 0 |
| `time_to_first_worker` | 未测 | p50 < 60s，p95 < 3min |
| `task_completion` | 未测 | p50 与 **p95** 同时记，p95 优于人工基线 |
| `router_decisions_per_task` | 未测 | 直通命中后下降 ≥ 30% |

> 只看 p50 会自欺——文章里单 Session 的 p50 并不难看，难看的是那条 29.5 分钟的长尾。

## 硬约束（跨湖，不随湖 defer）

1. **Router 无执行工具。** 不给 Bash / 文件写 / 网络。物理上做不到「这事很小我自己来」。
2. **只有 Router 对外说话。** Worker 只向 Router 汇报，不直接回复信道。
3. **状态只在 ADB 的 SQLite。** 不写进 skill、不写进对话、不写进前端。
4. **paseo 是外部进程。** 失败按「未启动/超时/无进展」三态记录，不伪造成功。
5. **本期只做 CLI。** 不做 HTTP 服务、不做消息队列、不做多 Router 分片。
6. **Router 只写 `exec_state`，不写 `truth_stage`。** 真相变更走 beacon change。
7. **Watchdog 必须在 adb 进程之外。** 用 paseo schedule 跑，否则 adb 一死它跟着死。

## 风险与对策

| 风险 | 对策 |
|------|------|
| L0 从 5 个仓库汇总时口径不一 | 只读已冻结的 truth/evidence，不推理、不补全；缺失写 `unknown` 并计数 |
| 直通把消息送错 worker | 绑定键包含 channel + thread + actor 三轴（ADB 已有），租约 TTL 兜底 |
| 轻模型误判「不是活」 | 所有 ignore 落 decision trace；watchdog 抽样复查 |
| 看门狗自己变噪音 | 默认 NOTHING；同一异常 6 小时内只报一次 |
| 范围蔓延成「重写 paseo」 | 本文档 Non-goals 即冻结边界，超界走 change |

## Non-goals

- 合并仓库 / 新建第 7 个 repo
- fork paseo 或复制其代码
- 修改 beacon truth 格式
- 做 UI / dashboard
- A2A 协议 / 多实例协作
- 替换 ADB 现有 dispatch / approval 语义

## 待定（P2 评审时确认）

- Supervisor 进程形态：`adb serve` 单进程循环，还是 systemd/launchd 托管？
- Router 轻模型具体选型与单次上下文预算
- requirement-index 是全公司一个文件，还是 per-project 一个再汇总
