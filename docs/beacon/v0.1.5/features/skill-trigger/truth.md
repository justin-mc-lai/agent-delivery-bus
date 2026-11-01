---
slug: skill-trigger
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
  truth: docs/beacon/v0.1.5/features/skill-trigger/truth.md
  tests: docs/beacon/v0.1.5/features/skill-trigger/tests.md
  tasks: docs/beacon/v0.1.5/features/skill-trigger/tasks.md
  evidence: docs/beacon/v0.1.5/features/skill-trigger/evidence.md
---

# Requirement Truth: skill-trigger

## 人话

现在要把活送进系统，得自己拼一长串 `adbd intake --channel ... --thread ...`。
这个湖给一个「遥控器」：在任何会话里说一句话，就能把它变成一个带线程的事件，
送进已经建好的直通/分派链路，而且不重复、不串线、出错能看懂。

它是"主动"的第一块：先证明「一条事件能从人这里走到 worker」，
再谈飞书、GitHub、选题这些自动源。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "主动，可以是飞书，也可以是我使用 skill 稳定的触发router沟通"
goal: "提供一个稳定的 skill 触发契约：一句话 -> 带 thread 的事件 -> intake -> route -> worker，且幂等、可诊断。"
non_goals: "不接飞书/GitHub/自媒体事件源；不改 router 决策；不做 UI；不改 paseo。"
constraints: "Go 放 ADB 仓库 cmd/adbd/；复用现有 intake/route；不依赖 launchd；真相只读。"
scope_mode: "lake"
```

## 状态模型

```text
feed: received -> validated -> submitted -> accepted | rejected
thread: skill:<project>:<slug>
```

- `received`：接到一次触发请求。
- `validated`：thread 形态、文本、幂等键都合法。
- `submitted`：已交给 intake。
- `accepted`：intake 返回 fast（有绑定）或 slow（入队）。
- `rejected`：校验失败，带 reason_code，不落任何队列。

## 用户旅程

1. 人在任意会话里说"把 studio 的 AC-003 做掉"。
2. skill 把它变成一次 `adbd feed --project studio --slug ac-003 --text "..."`。
3. feed 生成 `thread=skill:studio:ac-003` 和内容幂等键，交给 intake。
4. 有绑定时直通 worker；没有时落慢路径，等 route 在 2 分钟内处理。
5. 同一句话再说一遍，幂等键相同，不会重复派活。
6. 出错时（thread 不合法、文本为空、派发失败）返回明确 reason_code，不静默成功。

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 用户原话：用 skill 稳定触发 | truth.md#Acceptance-Criteria |
| AC-002 | goal：一句话 -> 带 thread 的事件 | truth.md#Acceptance-Criteria |
| AC-003 | goal：不串线 | truth.md#Acceptance-Criteria |
| AC-004 | goal：能看见它真的动 | truth.md#Acceptance-Criteria |
| AC-005 | goal：出错能看懂 | truth.md#Acceptance-Criteria |
| AC-006 | constraints：不依赖 launchd | truth.md#Acceptance-Criteria |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `skills/agent-delivery-bus/SKILL.md` 中存在一节固定标题 `## 触发 router（skill intake）`，给出唯一推荐命令形态 `adbd feed --project <p> --slug <s> --text <t> [--channel skill]`，并声明它是触发 router 的唯一入口（不再教人手拼 `adbd intake`） |
| AC-002 | `adbd feed` 接受 `--project` / `--slug` / `--text` / 可选 `--channel`（默认 `skill`）/ 可选 `--message-id`；未提供 message-id 时按 `sha256(channel|thread|text)` 取前 16 位十六进制生成；同一输入两次调用生成同一键，第二次输出 `route=deduped` |
| AC-003 | thread 固定为 `skill:<project>:<slug>`；`project` 与 `slug` 只允许 `[a-z0-9._-]`，长度 1–64；不合法返回 `thread_invalid` 且不写任何队列；不同 project 的同一 slug 互不影响 |
| AC-004 | 端到端：`adbd feed` 一次调用后，`.beacon/adbd-decisions.jsonl` 恰好出现一行对应这个 message_id（直通路径、慢路径两者之一），runner 收到的文本以 `UNTRUSTED_INPUT` 开头；输出含 `route=fast`、`route=slow`、`route=deduped` 三者之一 |
| AC-005 | 三种错误各自返回明确 reason_code 且不静默：文本为空、文本全为空白 → `text_empty`；thread 组成不合法 → `thread_invalid`；派发时 runner 失败 → `dispatch_failed`（保留 attempts，条目回队列）。三种情况退出码均为 2，成功为 0 |
| AC-006 | `adbd feed` 是同步单次命令，不读心跳、不依赖 launchd、不依赖任何常驻进程；测试在无托管环境（直接执行二进制）下通过，且连续两次调用结果一致 |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_allow_channels` | 空（fail-closed） | skill 触发需要把 `skill` 加进允许列表 |
| `adbd_feed_project_channel` | `skill` | feed 默认 channel |

## 禁止的迁移（人工维护，内核不校验）

- illegal: rejected -> submitted
- illegal: accepted -> validated

## Non-goals（冻结边界）

- 不接飞书、GitHub、自媒体事件源（后续湖）
- 不改 router 的四选一决策
- 不做交互式 TUI，不做批量导入
- 不改 paseo，不改 beacon 真相

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 触发入口用新子命令 adbd feed，还是纯 shell 包装 intake？ | adbd feed：thread 形态和幂等键都在代码里校验，避免 shell 拼串出错 | answered | adbd feed 子命令；thread 与幂等键在代码里校验和生成。 | human |
| Q2 | 没给 message-id 时怎么生成幂等键？ | sha256(channel\ | answered | sha256(channel\|thread\|text) 前 16 位十六进制。 | open |
