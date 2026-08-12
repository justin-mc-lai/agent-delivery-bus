---
slug: session-routing
version: v0.1.2
status: frozen
language: zh
domain_required: true
ux_required: false
package_maturity: filled
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
materials_status: current
canonical_refs:
  prd: docs/beacon/v0.1.2/features/session-routing/truth.md
  user_story: docs/beacon/v0.1.2/features/session-routing/truth.md
  test_case: docs/beacon/v0.1.2/features/session-routing/tests.md
---

# Requirement Truth: session-routing (v0.1.2)

## 人话

让 ADB 能在聊天会话（飞书/微信/Line）里**稳定区分"哪个会话、谁在说话、派给哪个 agent、干到哪个会话里"**。核心是四根身份轴：通道线程（channel_thread）、宿主会话（host_session）、目标执行器（target_executor：codex/claude/pi）、目标执行会话（target_session）。ADB 维护 SessionRegistry 持久映射，dispatch envelope 升级 v1.1 携带全部身份轴并纳入幂等键；任务 body 注入 `### Session context`；reconcile 完成后回发原线程；审批可绑定渠道身份。

- 能做：`adb session bind/resolve/list/status`；envelope v1.1 + 六要素幂等键；`adb intent parse --agent` 目标选择；Session context 注入（pi 固定 `--session-id` resume）；reconcile 回发（`hermes send`）；approve `--channel-actor` 校验。
- 不能做：自动 webhook 入站桥；跨设备配置同步；session 内容审计。
- 怎样算完：SessionRegistry + envelope v1.1 + intent agent + 注入 + 回发 + 渠道审批全部有行为测试；同一线程同意图幂等；session 过期 fail-closed；既有 167 测试全绿；QA 后 release 人工门。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: session-routing
revision: R1
program: session-routing (v0.1.2)
depends_on:
  - pi-executor (v0.1.0)
  - pi-curator (v0.1.1)
session_axes: channel_thread / host_session / target_executor / target_session / actor
targets: codex | claude | pi
release_gate: human always
```

## 用户旅程

1. 触发：用户在飞书 DM/话题对承载 adb 的宿主 agent 说"派发给 pi 实现 xxx"。
2. 关键操作：宿主 `adb session bind --channel feishu --thread oc_x[:topic] --actor open_id --host-session <hermes_id> --target pi` → `adb intent parse --agent pi` → 回显 envelope（含 session 身份）→ 确认 → dispatch（幂等键含六要素）→ pi 以固定 `--session-id` 执行 → reconcile。
3. 结果：completed/blocked 回发到原线程；同一线程重复说同意图 → 幂等复用不重复派发。
4. 异常：session 过期 → `session_stale` blocked + 重绑指引；目标执行器未知/歧义 → blocked + candidates；审批渠道身份不匹配 → approval_channel_actor_mismatch。

## First principles

- 会话不是单一概念：消息从哪来（线程）、谁解析（宿主会话）、谁执行（目标执行器+目标会话）、谁审批（actor）是四根正交轴。
- 稳定 = 每轴确定性来源 + 六要素幂等键 + 证据绑定 dispatch_id；任何一轴靠猜都会串会话/重复派发。
- 不可变：幂等键六要素（thread+actor+target_executor+stage+feature+project）；不静默回落；release 人工门。

## Domain Model

| Entity | Key fields | Invariant / requires |
|--------|------------|----------------------|
| SessionBinding | session_id, channel, channel_thread, actor_id, host_session, target_executor, target_session | session_id 确定性哈希；upsert + last_seen |
| DispatchEnvelopeV11 | schema_version=1.1, channel/channel_thread/actor_id/host_session_ref/target_executor/target_session_ref | 旧 1.0 请求解析兼容 |
| SessionContextBlock | channel, actor_id, host_session_ref, target_executor, target_session_ref | 注入任务 body；pi 固定 session-id resume |
| DeliveryTarget | channel, channel_thread | reconcile 结果回发目标 |
| ChannelApproval | actor + channel_actor | 匹配才 reserve；宽松模式仅标记 |

## Entity Precedence

| Entity | Order | Requires |
|--------|-------|----------|
| SessionBinding | 1 | bind 调用 |
| DispatchEnvelopeV11 | 2 | SessionBinding resolve |
| SessionContextBlock | 3 | envelope 字段 |
| DeliveryTarget | 4 | dispatch 账本 |
| ChannelApproval | 5 | approve 时校验 |

## Domain FSM — SessionBindingLifecycle

| State | From | Guard |
|-------|------|-------|
| unbound | — | — |
| bound | unbound | bind upsert |
| active | bound | last_seen 心跳 |
| stale | active | last_seen 超时 |
| rebound | stale | 重新 bind |
| archived | bound/active | 显式归档 |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| unbound | bind() | 身份字段非空 | bound | 写 session_id |
| bound | heartbeat() | 绑定存在 | active | 更新 last_seen |
| active | probe() | last_seen 超时 | stale | 返回 session_stale |
| stale | bind() | 同身份 | rebound | 覆盖 target_session |
| bound/active | archive() | — | archived | 停止解析 |

**终态：** archived；release 仍为独立人工门。

### Legal walks

1. **W-SR-01** unbound → bound → active → dispatch → reconcile → 回发 · TC-SR-005/007
2. **W-SR-02** 同线程同意图重试 → 同 idempotency key · TC-SR-002
3. **W-SR-03** stale → rebound · TC-SR-001

## Illegal transitions

- active → dispatch without session identity · TC-SR-ILL-001
- 幂等键不含六要素（丢会话轴）· TC-SR-ILL-002
- 目标执行器未知/歧义静默回落 · TC-SR-ILL-003
- 审批渠道身份不匹配仍放行 · TC-SR-ILL-004
- release 自动放行 · TC-SR-ILL-005

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-SR-001 | SessionRegistry：`adb session bind/resolve/list/status`；session_id=`sess_<sha256(channel\|thread\|actor\|host_session)>` 确定性；upsert + last_seen；stale（默认 TTL 24h）→ session_stale |
| AC-SR-002 | dispatch envelope v1.1：normalized_request 增 channel/channel_thread/actor_id/host_session_ref/target_executor/target_session_ref；幂等键 = digest(thread+actor+target_executor+stage+feature+project)；旧 1.0 兼容 |
| AC-SR-003 | `adb intent parse --agent pi|codex|claude|auto`：显式 > 项目 executor_policy.stages > 通道默认；歧义/未知 blocked + candidates；envelope 带 target_executor |
| AC-SR-004 | 任务 body 注入 `### Session context`；pi create_task 支持 session_id → `--session-id` 固定 resume；codex/claude 目标回传 `--resume <host_session>` 指引 |
| AC-SR-005 | reconcile 回发：dispatch 账本记录 channel_thread；HermesAdapter.deliver() 用 `hermes send --to feishu:<chat>[:<topic>]`；completed/blocked 均回发；发送失败不影响状态（记录 deliver_failed） |
| AC-SR-006 | 审批渠道身份：approve 支持 `--channel-actor <open_id>`；reserve 校验绑定；不匹配 → approval_channel_actor_mismatch；未提供时兼容旧行为并标记 unverified |
| AC-SR-007 | 自然语言全流程：bind → intent parse --agent → envelope 确认 → dispatch → pi 执行 → reconcile → 回发；同线程同意图幂等；既有 167 测试全绿 |

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-SR-001 | covered |
| INT-002 | user | must | AC-SR-002 | covered |
| INT-003 | user | must | AC-SR-003 | covered |
| INT-004 | user | must | AC-SR-004 | covered |
| INT-005 | user | must | AC-SR-005 | covered |
| INT-006 | user | must | AC-SR-006 | covered |
| INT-007 | user | must | AC-SR-007 | covered |

## Non-goals

- 不做渠道入站 webhook 自动化（宿主手动 bind 先跑通）。
- 不做跨设备同步、session 内容审计、多租户隔离。

## Freeze readiness

- [x] Intent Matrix must 无 missing
- [x] 用户旅程无待补文案
- [x] First principles / Domain Model 已具体化
- [x] 业务 FSM 五元组 + illegal
- [x] 每 AC ≥ 1 TC（Command+Assertion）+ exec-layer TC
- [x] tasks 均有 ac= + evidence= 绑定
- [x] package_maturity: filled
- [ ] `beacon truth review` pass 后再 freeze
