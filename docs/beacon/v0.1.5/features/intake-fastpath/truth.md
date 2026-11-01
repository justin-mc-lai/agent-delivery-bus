---
slug: intake-fastpath
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
  truth: docs/beacon/v0.1.5/features/intake-fastpath/truth.md
  tests: docs/beacon/v0.1.5/features/intake-fastpath/tests.md
  tasks: docs/beacon/v0.1.5/features/intake-fastpath/tasks.md
  evidence: docs/beacon/v0.1.5/features/intake-fastpath/evidence.md
---

# Requirement Truth: intake-fastpath

## 人话

同一个话题的第二条消息，不应该再让前台重新判一遍、重新分派一次。
它应该认出「这个话题已经有人在干了」，直接把消息送到那个人手上。
这样人不用当中转站，前台也不用为每条消息烧一次模型。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "现在所有动作都是我手动敲命令……我想要做完所有需求交付一个 superagent"
goal: "已绑定话题的后续事件由程序直通到原 worker，不再经过 Router 的 LLM；未绑定话题才走慢路径。"
non_goals: "不做 Router 的四选一决策；不做 watchdog；不做 IM 接入；不改 paseo；不做 UI。"
constraints: "常驻循环用 Go，代码放 ADB 仓库 cmd/adbd/；状态只在 ADB SQLite；paseo 只走 CLI；真相只读。"
scope_mode: "lake"
```

## 状态模型

三条状态轴，全部落在 ADB SQLite：

```text
thread_binding: unbound -> bound -> leasing -> expiring -> released
intake_event:   received -> accepted -> forwarded -> deduped | rejected
worker_report:  none -> reported
```

- `unbound`：没有绑定，落慢路径队列。
- `bound`：有 worker，可直通。
- `leasing`：直通中，每次成功转发即续租。
- `expiring`：租约剩余不足 1 小时，需要续租提醒。
- `released`：显式解绑；**不能**用新消息把它复活，必须新建绑定。

终态：`released`、`deduped`、`rejected`。

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|
| unbound | bind | worker 存在 | bound | 写绑定 + 租约 |
| bound | intake | 幂等键未见 | leasing | 转发 paseo |
| bound | intake | 幂等键已见 | bound | 丢弃并记账 |
| leasing | report | 任意 | leasing | 续租 |
| leasing | expire | now > expires_at-1h | expiring | 发续租提醒 |
| expiring | expire | now > expires_at | unbound | 解绑 + 落慢路径 |
| bound | release | 显式调用 | released | 清租约 |

## 禁止的迁移（人工维护，内核不校验）

- illegal: released -> bound
- illegal: unbound -> leasing
- illegal: expiring -> released

## 用户旅程

1. 运维在飞书话题里提第一句 → 前台建 worker，写 `(channel, thread) -> agent-id` 绑定，租约 24h。
2. 同一话题第二句 → `adbd intake` 命中绑定，`paseo send <agent-id>` 直通，`route=fast`；人不需要再说一遍背景。
3. 租约剩不到 1 小时 → 输出 `lease=expiring`；worker 任一次汇报都会续租。
4. 租约过期 → 绑定解为 `unbound`，下一条消息落慢路径队列，前台重新决定。
5. 重复投递（webhook 重放）→ 幂等键命中，`route=deduped`，worker 不会被喂第二遍。

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | constraints：常驻循环用 Go，放 cmd/adbd/ | truth.md#Acceptance-Criteria |
| AC-002 | goal：已绑定话题直通原 worker | truth.md#Acceptance-Criteria |
| AC-003 | constraints：状态只在 SQLite | truth.md#Acceptance-Criteria |
| AC-004 | 状态模型：续租与过期 | truth.md#状态模型 |
| AC-005 | non_goals：不做 Router 决策 | truth.md#Acceptance-Criteria |
| AC-006 | 状态模型：非法迁移必须拒绝 | truth.md#状态模型 |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | ADB 仓库内新增 `cmd/adbd/` 独立 Go module（含 `go.mod`）；`go build ./...` 在 `cmd/adbd` 下 exit 0；`adbd --version` 输出非空；不得把 Go 源码混进 Python 包目录 |
| AC-002 | `adbd intake --channel <c> --thread <t> --message-id <m> --text <s>` 在已有绑定时调用一次 paseo，输出 `route=fast`；paseo 调用经过可配置的 runner（二进制路径 + argv），集成测试把 runner 指向一个真实的记录型可执行脚本，断言该进程实际收到 `send <agent-id> <text>` |
| AC-003 | 同一 `(channel, thread, message_id)` 重复投递只转发一次；第二次输出 `route=deduped` 且不调用 runner；幂等键在 SQLite 有唯一约束 |
| AC-004 | 每次成功转发把租约续到 now+TTL；租约剩余不足 1 小时时状态转 `expiring` 并输出 `lease=expiring`；超过过期时间则解绑为 `unbound` 并输出 `route=slow` |
| AC-005 | 无绑定的 thread 返回 `route=slow` 且不调用 runner，事件落慢路径队列；未知来源（未在允许列表的 channel）返回 `route=rejected` 且 reason_code 为 `source_not_allowed` |
| AC-006 | 同一 thread 的绑定只对应一个 worker；不同 thread 的事件不会串到彼此（测试用两个 thread 交叉投递，断言各自的 runner 只收到自己的文本）；试图从 `released` 复活绑定返回 `binding_released` |

## Non-goals（冻结边界）

- 不做 Router 的四选一决策与 LLM 调用（router-loop 湖）
- 不做 watchdog 判定（watchdog 湖）
- 不做 IM/webhook 接收端（调用方喂事件）
- 不 fork paseo，不 import 它的代码
- 不改 beacon 真相

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 转发用 paseo send 到已绑定 agent，还是每次 paseo run 新建？ | paseo send <agent-id> <text>；绑定里存 agent-id | answered | paseo send <agent-id> <text>；绑定记录里存 paseo agent-id，同话题后续消息不新建 agent。 | human |
| Q2 | 租约 TTL 默认多少？临期提醒阈值？ | TTL 沿用 ADB 现有 24h；剩余不足 1h 转 expiring 并提醒 | answered | TTL 沿用 ADB 现有 DEFAULT_TTL_SECONDS = 24h；剩余不足 1h 转 expiring 并提醒。 | human |
