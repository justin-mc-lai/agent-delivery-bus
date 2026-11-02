---
slug: feishu-inbound
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
  truth: docs/beacon/v0.1.5/features/feishu-inbound/truth.md
  tests: docs/beacon/v0.1.5/features/feishu-inbound/tests.md
  tasks: docs/beacon/v0.1.5/features/feishu-inbound/tasks.md
  evidence: docs/beacon/v0.1.5/features/feishu-inbound/evidence.md
---

# Requirement Truth: feishu-inbound

## 人话

群里的消息现在进不来。这个湖做一根管子：
`lark-cli event consume` 吐出的每一行事件，都变成一条带 thread 的消息交给 intake。
不是文本的、机器人自己发的、字段不全的，一律跳过并说清楚原因，不猜、不崩。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "顺序修复以上缺口、断点，不绕过不回避不拖延"
goal: "把飞书事件流映射成 intake 事件：chat -> thread，message_id 作幂等键，非文本/机器人消息跳过并记录。"
non_goals: "不实现长连接；不做富文本；不改 router；不改出站。"
constraints: "Go 放 cmd/adbd（stdlib only）；stdin NDJSON；跳过必须留原因。"
scope_mode: "lake"
```

## 映射规则

| 事件字段 | 用途 |
|----------|------|
| `chat_id` | thread = `feishu:<chat_id>` |
| `message_id` | 幂等键 |
| `content` | 消息文本（lark-cli 已渲染成可读文本） |
| `message_type` | 仅 `text` 收；其它跳过 |
| `sender_type` | `bot` 跳过（避免自问自答） |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `adbd feishu-feed` 从 stdin 读 NDJSON，每行一个事件；映射为 channel=`feishu`、thread=`feishu:<chat_id>`、message_id=`<message_id>`、text=`content`，交给既有 intake；结束时输出 `accepted=<n> skipped=<m>` |
| AC-002 | `message_type` 不等于 `text` 的行跳过，`skipped` 计数，并输出 `reason=unsupported_message_type` |
| AC-003 | `sender_type` 等于 `bot` 的行跳过，`reason=from_bot` |
| AC-004 | 非 JSON 行、缺 `chat_id`/`message_id`/`content` 的行逐行跳过并给出 `reason`（`invalid_json`/`missing_field`），不中断后续行，退出码 0 |
| AC-005 | 端到端：一行文本事件进 intake，重复同一 message_id 只处理一次；输出包含 accepted 与 deduped 的分别计数 |

## 禁止的迁移（人工维护，内核不校验）

- illegal: accepted -> skipped（同一 message_id 只能有一个终态）

## Non-goals（冻结边界）

- 不管理长连接/重连（lark-cli 负责）
- 不解析富文本、图片、文件
- 不改 router 决策与出站投递

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | user utterance | truth.md#User-Intent-Snapshot |
| AC-002 | user utterance | truth.md#User-Intent-Snapshot |
| AC-003 | user utterance | truth.md#User-Intent-Snapshot |
| AC-004 | user utterance | truth.md#User-Intent-Snapshot |
| AC-005 | user utterance | truth.md#User-Intent-Snapshot |

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 入站从哪儿读？ | stdin NDJSON，由 lark-cli event consume 管道喂入；adbd 只管映射 | answered | stdin NDJSON，lark-cli 负责长连接与事件订阅。 | human |
