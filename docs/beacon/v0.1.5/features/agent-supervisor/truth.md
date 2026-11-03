---
slug: agent-supervisor
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
  truth: docs/beacon/v0.1.5/features/agent-supervisor/truth.md
  tests: docs/beacon/v0.1.5/features/agent-supervisor/tests.md
  tasks: docs/beacon/v0.1.5/features/agent-supervisor/tasks.md
  evidence: docs/beacon/v0.1.5/features/agent-supervisor/evidence.md
---

# Requirement Truth: agent-supervisor

## 人话

paseo 里堆了 176 个 agent：88 个已死、85 个停着、2 个报错、1 个在跑。
停着的 85 个里混着三种完全不同的东西——干完了的、卡在等权限的、干到一半被丢下的。
系统只告诉你"idle"，所以人只能靠记忆区分。

这个湖做四件事：给 agent 挂工牌（认领）、按四个事实判它到底什么状态（看住）、
该放行就放行该归档就归档（动作）、最后只把"在等你"的摆到你面前（收口）。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "目前我的核心关键问题就是我目前paseo 管理的多个会话agent ，如何稳定地使用Super Agent进行管理"
goal: "让 Super Agent 成为 paseo agent 的唯一管理者：归属显式、状态可判、动作确定、注意力收敛。"
non_goals: "不做 Web 面板；不迁移 DSH 的 Ticket/Mandate/Policy；不用模型判断状态；不删除 agent；不改 paseo 本身。"
constraints: "Go 放 cmd/adbd（stdlib only）；paseo 只走 CLI；只 archive 不 delete；无标签的 agent 绝不触碰。"
scope_mode: "lake"
```

## 管理不变量

每个 paseo agent 在任何时刻属于且只属于三类之一：

| 类别 | 判据 | 谁能动 |
|------|------|--------|
| owned + active | 有 `adbd.item` 标签且工作项未关闭 | Super Agent |
| owned + terminal | 有标签且工作项已关闭 | Super Agent（负责回收） |
| foreign | 无 `adbd.item` 标签 | 任何人都不动 |

工作项 id 等于 thread 字符串（例如 `skill:studio:probe`），与绑定同源，不新增状态。

## 状态判定（四个事实，按优先级）

1. `paseo inspect` 的 `PendingPermissions` 非空
2. `paseo inspect` 的 `Status`（running / idle / error）
3. 工作项报告文件（`.beacon/adbd-reports/<item>.json`）
4. 该 agent 距最近一次实质动作的时长

| 观察 | 状态 | 动作 |
|------|------|------|
| running 且 PendingPermissions 非空 | waiting_permission | 按策略放行、拒绝、升级 |
| running | working | 无动作 |
| idle 且报告 ready | done | 归档 agent，关闭工作项 |
| idle 且报告 attention | waiting_input | 进收口清单，等人 |
| idle 且无报告且未超宽限 | grace | 无动作 |
| idle 且无报告且超过宽限 | orphan | 发一次探针 |
| orphan 且已探过仍无响应 | failed | 归档 agent，记失败 |
| error | failed | 归档 agent，记失败 |

## 用户旅程

1. 存量 176 个 agent 先跑一次 `adbd adopt --dry-run`，看清哪些能对上工作项、哪些对不上。
2. `adbd adopt --apply` 落标；对不上的标成 foreign，从此不再出现在管理面。
3. 每两分钟 `adbd supervise --once` 按上表判定，dry-run 时只打印，加 `--apply` 才动手。
4. 卡在权限的按策略放行或升级；干完了的归档；发呆超时先探一次，再没反应记失败。
5. `adbd inbox` 只列在等权限和在等人回答的项，全都没有时输出 `NOTHING`。

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | goal：归属显式 | truth.md#Acceptance-Criteria |
| AC-002 | goal：先看清再动手 | truth.md#Acceptance-Criteria |
| AC-003 | goal：认领落地 | truth.md#Acceptance-Criteria |
| AC-004 | goal：状态可判 | truth.md#状态判定 |
| AC-005 | goal：动作确定 | truth.md#状态判定 |
| AC-006 | goal：不猜，先探 | truth.md#状态判定 |
| AC-007 | goal：收尸 | truth.md#Acceptance-Criteria |
| AC-008 | 稳定性：不被无限开 agent 拖垮 | truth.md#Acceptance-Criteria |
| AC-009 | goal：注意力收敛 | truth.md#Acceptance-Criteria |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 新建 worker 时 `paseo run` 必须携带 `--label adbd.item=<work_item_id>`；对已有 agent 用 `paseo agent update <id> --label adbd.item=<work_item_id>` 补标；`work_item_id` 取值等于该线程的 thread 字符串 |
| AC-002 | `adbd adopt --dry-run` 遍历 `paseo ls --json`，按 `adbd.item` 标签、agent 名称两者是否等于任一绑定 thread 分成 owned 与 foreign 两类，输出 `owned=<n> foreign=<n>` 及逐条明细；不写标签、不改 agent、不调 archive |
| AC-003 | `adbd adopt --apply` 仅对能对上工作项的 agent 补 `adbd.item` 标签，其余补 `adbd.foreign=1`；重复执行不产生重复标签，输出与首次一致 |
| AC-004 | `adbd supervise --dry-run` 对每个 owned 项按「状态判定」表得出唯一状态，输出 `item=<id> state=<state> because=<fact>`；不执行任何动作；不认识的 agent 状态输出 `state=unknown` 并给出原始 Status |
| AC-005 | `supervise --apply` 处理 waiting_permission：权限请求的 `tool` 名不含 `delete`、`push`、`release`、`deploy` 任一高危词时执行 `paseo permit allow <agent> --all` 并记审计；含任一高危词时不自动放行，输出 `action=escalate`；每条动作向 `.beacon/adbd-supervise.jsonl` 追加一行，字段为 `item`、`agent_id`、`action`、`reason`、`at` |
| AC-006 | orphan 首次出现时向该 agent 发送一次探针消息并写入 `probed_at`；下一轮仍为 orphan 且 `probed_at` 非空时状态转 failed，不再发第二次；同一工作项在一轮内最多一次探针 |
| AC-007 | done 与 failed 的工作项执行 `paseo archive <agent>` 并记审计；只 archive 不 delete；已归档的 agent 在后续 supervise 中不再产生动作 |
| AC-008 | owned 且 working 的数量达到 `adbd_max_active`（默认 8）时，`route` 不再新建 worker，返回 `reason_code=max_active_reached`，队列条目保留 |
| AC-009 | `adbd inbox` 输出三类工作项：waiting_permission、waiting_input、escalated；escalated 取自 `.beacon/adbd-decisions.jsonl` 中 `decision=escalate` 且 `decided_at` 落在 `adbd_inbox_escalation_window`（默认 24h）内的条目；三类合并后按等待时长降序；三类都为空时输出 `NOTHING` 且退出码 0 |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_grace_after` | `5m` | idle 且无报告多久后转 orphan |
| `adbd_max_active` | `8` | owned 且 working 的上限 |
| `adbd_report_dir` | `<root>/.beacon/adbd-reports` | 工作项报告目录 |

## 禁止的迁移（人工维护，内核不校验）

- illegal: foreign -> owned（必须显式 adopt，不能由 supervise 顺手改）
- illegal: terminal -> active（关闭的工作项只能新建）

## Non-goals（冻结边界）

- 不做 Web 面板
- 不迁移 DSH 的 Ticket、Mandate、Policy 三层
- 不用模型判断状态或放行权限
- 不删除任何 agent（只 archive）
- 不改 paseo 本身的实现

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 归属用什么表达？ | paseo 原生 label adbd.item=<thread>；工作项 id 复用 thread 字符串 | answered | paseo label adbd.item=<thread>，工作项 id 与 thread 同源。 | human |
| Q2 | 状态判定用模型还是规则？ | 四个事实的确定性规则；模型只在需要回答等待中的问题时介入 | answered | 确定性规则判状态；模型不参与状态判定与权限放行。 | human |
