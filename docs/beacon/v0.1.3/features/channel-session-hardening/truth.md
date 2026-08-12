---
slug: channel-session-hardening
version: v0.1.3
status: frozen
language: zh
domain_required: true
ux_required: false
package_maturity: filled
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
materials_status: current
canonical_refs:
  prd: docs/beacon/v0.1.3/features/channel-session-hardening/truth.md
  user_story: docs/beacon/v0.1.3/features/channel-session-hardening/truth.md
  test_case: docs/beacon/v0.1.3/features/channel-session-hardening/tests.md
---

# Requirement Truth: channel-session-hardening (v0.1.3)

## 人话

把"任意聊天渠道 + `adb + 项目编号 + 业务描述` → 对应 agent 会话调度"加固到稳定不串：① 入站桥按 payload 平台感知渠道（feishu/weixin/line 同一脚本，不再写死 feishu）；② 每次派发默认独立目标会话（pi 用任务级 `--session-id`），同线程并发任务不串话；③ 固定目标会话加 lease 互斥（忙则 `session_busy`）；④ 目标 agent 决议顺序固化并写进 SKILL/README；⑤ 结果按真实渠道回发。

- 能做：channel-aware 入站；任务级 target_session；SessionRegistry acquire/release lease；决议顺序（显式 > 绑定 > 项目 executor_policy > 通道默认）回显来源；按渠道回发。
- 不能做：自动 webhook 外部回调配置；跨设备同步；接管 codex/claude CLI 内部会话续跑。
- 怎样算完：五条能力均有行为测试；同线程并发两任务目标会话互不相同；同固定会话并发第二个任务 blocked；reconcile 释放 lease；既有 177 测试全绿；release 人工门。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: channel-session-hardening
revision: R1
program: channel-session-hardening (v0.1.3)
depends_on:
  - session-routing (v0.1.2)
channels: feishu / weixin / line
release_gate: human always
```

## 用户旅程

1. 触发：用户任意渠道发 `adb <编号> <业务描述>`。
2. 关键操作：入站桥读 payload platform → 按渠道 bind → `intent parse --agent` → 草稿 → 确认 → dispatch（决议顺序确定目标 + 任务级会话）。
3. 结果：并发任务各用独立会话；固定会话互斥；完成后回发原渠道线程。
4. 异常：渠道未知 → channel_unsupported blocked；固定会话忙 → session_busy；过期 → session_stale。

## First principles

- 不串 = 身份轴确定性 + 幂等键完整 + 目标会话互斥；本版本补第三项并去掉渠道硬编码。
- 不可变：决议顺序不静默跳级；lease 必须释放；release 人工门。

## Domain Model

| Entity | Key fields | Invariant |
|--------|------------|-----------|
| ChannelAwarePayload | platform, chat, thread, actor, text | platform 必填，未知 fail-closed |
| TaskSession | task_session_id, dispatch_id, target_executor, state | 每派发唯一；acquire/release 成对 |
| SessionLease | session_id, dispatch_id, acquired_at | 同 session 仅一个未完成 lease |
| ResolutionTrace | target_executor, source(explicit/binding/policy/channel_default) | envelope 回显 source |

## Entity Precedence

| Entity | Order |
|--------|-------|
| ChannelAwarePayload | 1 |
| TaskSession | 2 |
| SessionLease | 3 |
| ResolutionTrace | 4 |

## Domain FSM — TaskSessionLease

| State | From | Guard |
|-------|------|-------|
| idle | — | — |
| acquired | idle | acquire(dispatch) |
| released | acquired | release(dispatch) 匹配 |
| busy | acquired | 第二个 dispatch 尝试 acquire |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| idle | acquire(d) | 无未完成 lease | acquired | 记录 dispatch |
| acquired | acquire(d2) | 同 session 已有 lease | busy | 返回 session_busy（不落账） |
| acquired | release(d) | dispatch 匹配 | released | 清 lease |

**终态：** released；release 仍为独立人工门。

### Legal walks

1. **W-CH-01** 同线程并发两任务 → 两个不同 task_session_id · TC-CH-002
2. **W-CH-02** fixed 会话 acquire→release→再 acquire · TC-CH-003
3. **W-CH-03** feishu/weixin/line 同脚本同 envelope · TC-CH-001/005

## Illegal transitions

- acquire 跳过（无 lease 并发同会话）· TC-CH-ILL-001
- release 不匹配 dispatch · TC-CH-ILL-002
- 渠道未知静默按 feishu · TC-CH-ILL-003
- release 自动放行 · TC-CH-ILL-004

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-CH-001 | 入站桥 channel-aware：payload 取 platform（feishu/weixin/line/telegram 等）；bind/deliver 用真实渠道；未知渠道 → channel_unsupported blocked |
| AC-CH-002 | 任务级目标会话：channel_thread 存在时每次派发默认独立 target_session（`<target>-<digest[:12]>`）；`--target-session auto|fixed:<id>|<id>` 三态 |
| AC-CH-003 | SessionRegistry.acquire/release lease：同 fixed 会话并发第二个未完成 dispatch → session_busy；reconcile completed/failed 时 release；重试同 dispatch 幂等不重复 acquire |
| AC-CH-004 | 目标决议顺序固化：显式 --target-executor > 线程绑定 target > 项目 executor_policy.stages > 通道默认（hermes coding）；envelope 回显 resolution_source；SKILL/README 写明 |
| AC-CH-005 | 按渠道回发：dispatch 账本记录 channel；reconcile deliver 目标 `<channel>:<thread>`；feishu/weixin/line 同路径 |
| AC-CH-006 | 兼容回归：旧绑定（共享 target_session）仍可解析；既有 177 测试全绿 |

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-CH-001 | covered |
| INT-002 | user | must | AC-CH-002 | covered |
| INT-003 | user | must | AC-CH-003 | covered |
| INT-004 | user | must | AC-CH-004 | covered |
| INT-005 | user | must | AC-CH-005 | covered |
| INT-006 | user | must | AC-CH-006 | covered |

## Non-goals

- 不做外部 webhook 回调配置自动化；不做跨设备同步；不接管 codex/claude CLI 会话续跑。

## Freeze readiness

- [x] Intent Matrix must 无 missing
- [x] 用户旅程无待补文案
- [x] First principles / Domain Model 已具体化
- [x] 业务 FSM 五元组 + illegal
- [x] 每 AC ≥ 1 TC（Command+Assertion）+ exec-layer TC
- [x] tasks 均有 ac= + evidence= 绑定
- [x] package_maturity: filled
- [ ] `beacon truth review` pass 后再 freeze
