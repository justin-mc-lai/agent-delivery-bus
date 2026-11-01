---
slug: router-loop
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
  truth: docs/beacon/v0.1.5/features/router-loop/truth.md
  tests: docs/beacon/v0.1.5/features/router-loop/tests.md
  tasks: docs/beacon/v0.1.5/features/router-loop/tasks.md
  evidence: docs/beacon/v0.1.5/features/router-loop/evidence.md
---

# Requirement Truth: router-loop

## 人话

没有绑定的话题，现在只是被扔进一个队列，没人管。
这个湖给队列加一个"前台"：它把积压的消息一次性读完，每条只做四个决定之一
——派人干、回一句话、不理、交给人类——然后记账。
它**没有执行工具**：不能跑命令、不能写文件，所以它不可能"这个小事我自己来"。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "现在所有动作都是我手动敲命令……我想要做完所有需求交付一个 superagent"
goal: "慢路径队列由常驻前台批量消费，每条消息产出一个四选一决定并留痕；前台无法执行任何工具。"
non_goals: "不接 IM 信道；不做 watchdog 判定；不调用真实 LLM；不改 paseo；不做 UI。"
constraints: "Go 放 ADB 仓库 cmd/adbd/；状态只在 ADB 本地文件；决策器可注入；无执行工具必须可枚举验证；真相只读。"
scope_mode: "lake"
```

## 状态模型

```text
queue_item: queued -> claimed -> decided -> applied | failed
decision:   route | reply | ignore | escalate
dispatch:   pending -> sent | backed_off -> escalated
```

- `queued`：慢路径队列里的一条。
- `claimed`：本次 drain 已取出，尚未落决定。
- `decided`：决策已写 trace。
- `applied`：派活或记账已完成。
- `failed`：应用失败，进入 `backed_off`，连续 3 次升级。

终态：`applied`、`escalated`、`ignore`。

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|
| queued | drain | batch 未满 | claimed | 取出 |
| claimed | decide | 决策合法 | decided | 写 trace |
| decided | apply | route 且未派过 | applied | bind + 派活 |
| decided | apply | reply | applied | 记账（不投递） |
| decided | apply | ignore | applied | 记账 |
| decided | apply | escalate | applied | 记账 + 升级 |
| decided | apply | 派活失败 | failed | 记 attempts |
| failed | apply | attempts >= 3 | escalated | 停止重试 |

## 用户旅程

1. 群里有条没有任何绑定的话题 → intake 落慢路径队列（M1 已有）。
2. `adbd route --once` 把队列一次读完（一批），每条产出一个决定。
3. `route` 的决定会建绑定并把消息派给 worker；`reply` 的决定记下要说的话（本期不投递）。
4. 人打开 `.beacon/adbd-decisions.jsonl` 就能看到：哪条消息、为什么这么判、属于哪一批。
5. 进程被杀后重启，未 applied 的条目还在队列里，不会重复派活。

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | goal：批量消费慢路径队列 | truth.md#Acceptance-Criteria |
| AC-002 | goal：四选一决定 | truth.md#状态模型 |
| AC-003 | 用户原话：不允许模型自己干活 | truth.md#Acceptance-Criteria |
| AC-004 | goal：留痕 | truth.md#Acceptance-Criteria |
| AC-005 | 状态模型：失败重试 | truth.md#状态模型 |
| AC-006 | 状态模型：重启恢复 | truth.md#状态模型 |
| AC-007 | constraints：不可信输入 | truth.md#Acceptance-Criteria |
| AC-008 | 用户决策：先接 LLM 再发版 | truth.md#Acceptance-Criteria |
| AC-009 | 用户要求：判断完之后真的派得出去 | truth.md#Acceptance-Criteria |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `adbd route --once` 一次 drain 把慢路径队列里的全部条目作为**一个批次**交给决策器；决策器接口在一次 drain 内只被调用一次；输出 `batch=<n> decided=<m>`；队列为空时输出 `batch=0` 且不调用决策器 |
| AC-002 | 决策只接受 `route` / `reply` / `ignore` / `escalate` 四种；任何其它值被拒绝并记 `decision_invalid`，队列条目标 `failed`，进程退出码仍为 0 |
| AC-003 | Router 的执行工具集在运行时枚举为空：`adbd route --list-tools` 输出 `[]`；实现里不得 import `os/exec`（静态检查：源码扫描断言 route 包不引用 os/exec），派活必须通过注入的 `Dispatcher` 接口完成 |
| AC-004 | 每个决定写一行 `.beacon/adbd-decisions.jsonl`，字段固定：`channel` / `thread` / `message_id` / `decision` / `reason_code` / `batch_size` / `untrusted` / `decided_at`；同一消息重复 drain 不产生第二行 |
| AC-005 | 派活失败时条目进入 `backed_off` 并记录 `attempts`；连续失败 3 次后自动转 `escalate`（reason_code=`dispatch_attempts_exhausted`），不再重试 |
| AC-006 | `route --once` 处理完成后把未 applied 的条目原子写回队列；进程在 drain 中途被杀，重启后这些条目仍在（测试：处理一半时模拟失败，断言队列仍含未处理项且已 applied 的不会被再次派活） |
| AC-007 | 所有入站文本标记 `untrusted=true` 并写入 trace；派给 worker 的文本以固定守卫行开头（`UNTRUSTED_INPUT`）；含高危动词（`delete` / `push` / `release` / `deploy`）的 untrusted 文本不得判为 `route`，必须 `escalate`（reason_code=`untrusted_high_risk`）；`reply` 文本超过 100 个字符被拒绝（reason_code=`reply_too_long`） |
| AC-009 | 派活前必须解析出**真实存在**的 worker：① 绑定里已有 agent id 且 `paseo inspect` 成功 → 复用；② 否则 `adbd_worker_agent_id` 非空且 inspect 成功 → 用它；③ 两者都不成立时，用 `paseo run --no-wait --json --provider <adbd_worker_provider> --model <adbd_worker_model> --title <thread> --cwd <root>` 新建，解析返回的 `agentId` 并写回绑定，供后续消息复用；④ 新建失败 → `dispatch_failed`，条目保留 attempts。任何情况下都不得把一个未经验证的固定 id（例如 `main`）当作真实 worker 发送 |
| AC-008 | `adbd_decider=llm` 时启用 LLMDecider（实现同一 `Decider` 接口）：一次 drain 只发一个 OpenAI 兼容 `/chat/completions` 请求携带整批；响应必须是 JSON 数组且每项含 `message_id` / `decision` / `reason_code`；校验失败的条目记 `decision_invalid` 并置 failed；HTTP 超时、非 2xx 响应时回退 RuleDecider 并在 trace 记 `decider=llm_fallback_rules`；LLM 不得产出四选一以外的决策，也不得把高危 untrusted 文本判成 route（代码复核，不依赖提示词） |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_worker_agent_id` | 空 | 指定的 paseo agent id；空则自动新建 |
| `adbd_worker_provider` | `opencode` | 新建 worker 时用的 provider |
| `adbd_worker_model` | `deepseek/deepseek-flash` | 新建 worker 时用的模型 |

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_decider` | `rule` | `rule` / `llm` |
| `adbd_llm_base_url` | `https://api.deepseek.com/v1` | OpenAI 兼容端点 |
| `adbd_llm_model` | `deepseek-chat` | 模型名 |
| `adbd_llm_api_key` | 空（读 `ADBD_LLM_API_KEY`） | 密钥，不落日志 |
| `adbd_llm_timeout` | `60s` | 单次请求超时 |

## 禁止的迁移（人工维护，内核不校验）

- illegal: decided -> claimed
- illegal: applied -> queued
- illegal: escalated -> dispatched

## Non-goals（冻结边界）

- 不接 IM/飞书信道；`reply` 只记账不投递（M4 接）
- 不做多模型路由与成本优化；LLM 决策器只有一个端点
- 不做 watchdog 进展判定（M3）
- 不 fork paseo，不 import 它的代码
- 不改 beacon 真相

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 四选一用什么决策器？ | 确定性规则 + 可注入 Decider 接口；M2.5 再换 LLM | answered | 确定性规则决策器 + 可注入 Decider 接口；M2.5 换 LLM 时不改上层。 | human |
| Q2 | reply 决策本期真发出去还是只记账？ | 只写 trace，不投递；信道接入留到 M4 | answered | reply 只写 decisions.jsonl，不投递；信道接入在 M4。 | human |
