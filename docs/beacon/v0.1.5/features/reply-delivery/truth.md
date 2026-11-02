---
slug: reply-delivery
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
  truth: docs/beacon/v0.1.5/features/reply-delivery/truth.md
  tests: docs/beacon/v0.1.5/features/reply-delivery/tests.md
  tasks: docs/beacon/v0.1.5/features/reply-delivery/tasks.md
  evidence: docs/beacon/v0.1.5/features/reply-delivery/evidence.md
---

# Requirement Truth: reply-delivery

## 人话

前台会决定"回一句"，但这句话现在只写进日志，从来没发出去。
这个湖把它真发到那个聊天里：能发就发、发不了就说清楚为什么、绝不假装发过。
默认是关的——没配发信器时行为和现在完全一样。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "顺序修复以上缺口、断点，不绕过不回避不拖延"
goal: "reply 决策真正投递到来源聊天；投递结果写 trace；失败不改变决策、不假装成功。"
non_goals: "不做飞书入站；不做富文本/卡片；不接其他 IM；不改四选一。"
constraints: "Go 放 cmd/adbd（stdlib only）；发送器可注入；默认关闭；真相只读。"
scope_mode: "lake"
```

## 投递规则

| 情况 | 结果 | delivery_reason |
|------|------|-----------------|
| 未配置 `adbd_reply_bin` | 不发送 | `not_configured` |
| channel 不是 feishu | 不发送 | `unsupported_channel` |
| 已配置且是 feishu | 调 `lark-cli im +messages-send` | `sent` / `send_failed` |

发送失败**不影响**决策成立：条目仍算 applied，只把事实记进 trace。

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `adbd_reply_bin` 非空且 `thread` 属于 feishu 频道时，reply 决策调用一次 `<bin> im +messages-send --chat-id <chat_id> --text <reply> --as <adbd_reply_identity> --idempotency-key <message_id>`，其中 `<chat_id>` 是 thread 去掉 `feishu:` 前缀后的裸 ID；`adbd_reply_identity` 默认 `bot` |
| AC-002 | 每条 reply 决策的 trace 增加 `delivered`（true/false）与 `delivery_reason`（`sent`/`not_configured`/`unsupported_channel`/`send_failed`）两个字段；其余字段不变 |
| AC-003 | 发送失败时：条目仍然 applied（不进入重试），trace 记 `delivered=false`、`delivery_reason=send_failed`，并带 `delivery_detail`（stderr 前 200 字符）；进程退出码不受影响 |
| AC-004 | 同一 `message_id` 重复 reply 时使用同一 `--idempotency-key`；测试断言两次调用携带相同键 |
| AC-005 | 非 feishu 线程（`skill:*`、`gh:*`）不调用发送器，trace 记 `delivered=false`、`delivery_reason=unsupported_channel`，不报错 |
| AC-006 | `adbd_reply_bin` 为空（默认）时完全不调用发送器，trace 记 `not_configured`；行为与引入本湖之前一致（决策文本照常写 trace） |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_reply_bin` | 空（关闭） | 发送器二进制，例如 `lark-cli` |
| `adbd_reply_identity` | `bot` | 发送身份（`bot` 或 `user`） |

## 禁止的迁移（人工维护，内核不校验）

- illegal: sent -> send_failed（同一 message_id 只能有一个终态）

## Non-goals（冻结边界）

- 不做飞书入站事件
- 不做富文本、卡片、附件
- 不接企业微信、Slack
- 不改四选一判断

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | user utterance | truth.md#User-Intent-Snapshot |
| AC-002 | user utterance | truth.md#User-Intent-Snapshot |
| AC-003 | user utterance | truth.md#User-Intent-Snapshot |
| AC-004 | user utterance | truth.md#User-Intent-Snapshot |
| AC-005 | user utterance | truth.md#User-Intent-Snapshot |
| AC-006 | user utterance | truth.md#User-Intent-Snapshot |

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 出站用哪个发送器？ | lark-cli im +messages-send，可注入；未配置时默认关闭 | answered | lark-cli im +messages-send --chat-id <thread> --text <reply>；adbd_reply_bin 为空则关闭。 | human |
