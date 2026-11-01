---
slug: requirement-index
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
  truth: docs/beacon/v0.1.5/features/requirement-index/truth.md
  tests: docs/beacon/v0.1.5/features/requirement-index/tests.md
  tasks: docs/beacon/v0.1.5/features/requirement-index/tasks.md
  evidence: docs/beacon/v0.1.5/features/requirement-index/evidence.md
---

# Requirement Truth: requirement-index

## 人话

需求现在散在多个仓库的 beacon 文档里，想知道「有哪些需求、各自走到哪一步」只能靠人一个个翻会话。
这个湖把这些**已经冻结的事实**汇总成**一个 JSON 文件**，让常驻前台只读这一处就能看清全局。
它只**看**真相，不改真相。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "其实本质就是 人机交互中我 beacon 的真实需求 持续的 可多session 读取 并行。 super agent 怎么实现。"
goal: "把 beacon 已冻结的真相与 studio 需求池投影成单一 requirement-index.json，让 Router 只读一个文件就知道全部需求所处阶段与执行进度，且不反写真相。"
non_goals: "不改 beacon 真相；不改 studio 需求池；不做 UI/看板；不做 Router 派活；不做 watchdog；不合并仓库。"
constraints: "truth_stage 只读、exec_state 可写且只落 ADB SQLite；单文件必须能被 json.load 直接解析；超过 10KB 自动分层；缺失数据必须显式记录不得静默补全；生成幂等（同输入两遍输出一致）。"
scope_mode: lake
program_ref: docs/beacon/v0.1.5/programs/superagent-router
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | 使用者是 Router 与运维者；问题是需求散在多个仓库的 beacon 文档里；完成标准是一命令生成、两遍一致、缺数据有记录、越权改 truth_stage 被拒 | human |
| Q2 | index 粒度：全公司一个文件，还是每项目一个再汇总？ | 全公司一个；超 10KB 才分层 | answered | 全公司一个文件；超过 10240 字节时自动分层为 summary + requirements/<project>.json 分片。 | human |
| Q3 | exec_state 存哪：同一 JSON，还是 ADB SQLite 生成时合并？ | 只存 ADB SQLite，JSON 是派生只读视图 | answered | exec_state 只存 ADB SQLite；requirement-index.json 是派生只读视图，每次生成时合并，不回写。 | human |
| Q4 | truth_stage 从哪读：冻结包 lifecycle，还是解析 truth.md 的 status？ | 读冻结包 lifecycle（store），不解析 Markdown 正文 | answered | 读 beacon 冻结包的 lifecycle（store）；不解析 truth.md 的 status 字段。 | human |

## 数据契约（本湖的核心）

一条索引记录必含以下字段，缺一不可：

| 字段 | 写权 | 来源 | 说明 |
|------|------|------|------|
| `req_id` | — | beacon 冻结包 | 稳定主键，跨仓库唯一 |
| `project` | — | beacon 冻结包 | 仓库 slug |
| `truth_stage` | **只读** | beacon 冻结包 lifecycle | plan/truth/implement/qa/release |
| `exec_state` | **可写** | ADB SQLite | 执行进度（queued/running/reported/done/failed） |
| `thread_id` | 可写 | ADB | 关联的事件线程（L1 使用） |
| `worker` | 可写 | ADB | 当前执行者标识 |
| `blockers` | 可写 | ADB | 阻塞项列表 |
| `evidence_ref` | — | beacon 冻结包 | 证据文件路径 |
| `truth_ref` | — | beacon 冻结包 | truth.md 的锚点，供链式召回 |
| `updated_at` | — | 生成时刻 | ISO8601 |
| `source_status` | — | 生成器 | `ok` / `unknown`，缺失必须标 `unknown` |

**不可逆规则**：任何写入 `truth_stage` 的尝试必须被拒绝并返回 `truth_stage_readonly`。
真相只能由 `beacon truth change` + `refreeze` 改变。

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | goal：单一文件看清全局 | truth.md#数据契约 |
| AC-002 | constraints：生成幂等 | truth.md#Acceptance-Criteria |
| AC-003 | constraints：缺数据不静默 | truth.md#Acceptance-Criteria |
| AC-004 | constraints：json.load 直接可读 | truth.md#Acceptance-Criteria |
| AC-005 | goal + non_goals：不反写真相 | truth.md#数据契约 |
| AC-006 | constraints：超 10KB 分层 | truth.md#Acceptance-Criteria |
| AC-007 | goal：单一文件看清全局（前提是能自己找到包） | truth.md#枚举规则 |
| AC-008 | 用户原话："只用一条命令" | truth.md#Acceptance-Criteria |
| AC-009 | goal：单一文件看清全局（缺了阶段就等于没看清） | truth.md#生命周期来源 |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 生成 `requirement-index.json`；每条记录包含「数据契约」表中全部字段；文件可被 `python3 -c "import json;json.load(open(p))"` 直接解析；顶层含 `schema_version` 与 `generated_from[]` |
| AC-002 | 同一输入连续生成两次，输出**字节一致**（稳定键排序 + 固定时区/可注入时间源）；再次生成不产生重复记录 |
| AC-003 | 覆盖三种来源异常（文件缺失 / 文件不可读 / JSON 解析失败）：对应记录 `source_status=unknown`，并在顶层 `missing[]` 逐条记录 `source`、`reason`、`req_id`；**不删除记录、不填任何默认值**；生成仍以 exit_code_0 结束 |
| AC-004 | 单文件自洽：只把 `requirement-index.json` 拷到别处，`json.load` 仍成功且字段完整；分层模式下 summary 文件同样自洽，并可凭 `truth_ref` 定位到分片 |
| AC-005 | 提供写接口接受 `exec_state`/`thread_id`/`worker`/`blockers` 并成功落 ADB SQLite；任何携带 `truth_stage` 的写请求被拒绝，返回 `truth_stage_readonly`，且索引内容不变 |
| AC-006 | 索引体积超过 10240 字节时自动分层：`requirement-index.json`（summary）+ `requirements/<project>.json`（分片）；summary 每条仍保留 `req_id`/`truth_stage`/`exec_state`/`truth_ref`；未超阈值时不产生分片 |
| AC-007 | 给定一组 project root，枚举器发现其中 `docs/beacon/<version>/features/<slug>/truth.md` 且 frontmatter `status: frozen` 的包，产出 `RequirementSource` 列表；`req_id = req:<project>:<feature>`；未冻结、无 frontmatter、不可读的包记入 `skipped[]` 并继续；同一 project root 重复传入只算一次 |
| AC-008 | `adb index generate --projects a,b --out <path>` 一条命令完成枚举→生成→落盘并 exit 0；`adb index status --out <path>` 打印条数、格式、missing 计数；文件不存在时 exit 2 且 reason_code 为 `index_file_missing`；`ADB_REQUIREMENT_INDEX_NOW` 可固定时钟使两次运行字节一致 |
| AC-009 | `truth_stage` 只取自真实生命周期源，优先级：① `beacon-go truth list --project <root> --json` 返回的 `lifecycle_stage`；② 取不到时标 `unknown` 并写入 `missing[]`（reason=`lifecycle_source_missing`）。禁止用 frontmatter 的 `status`、目录名推断阶段；枚举器接受可注入的 lifecycle provider，测试不依赖真实内核 |

## Parity Matrix

不适用：`scope_mode: lake`，无上游 OSS 需要对齐。能力清单见 `feature-graph.json` 的 `parity_subset`。

## 生命周期来源

阶段只从内核拿，不从文件猜：

| 优先级 | 来源 | 取什么 |
|--------|------|--------|
| 1 | `beacon-go truth list --project <root> --json` | 该 slug 的 `lifecycle_stage` |
| 2 | （无） | 标 `unknown` + `missing[]`，reason=`lifecycle_source_missing` |

`status: frozen` 只用于**筛选**是否收录，不用于推断阶段：冻结只说明真相定了，不代表交付到哪一步。
项目未在内核注册时，枚举器报 `store_not_registered` 并提示先 `beacon-go projects register`。

## 枚举规则

谁进索引、谁不进：

| 情况 | 处置 |
|------|------|
| `docs/beacon/<version>/features/<slug>/truth.md` 存在且 `status: frozen` | 收，`req_id=req:<project>:<feature>` |
| frontmatter 缺失 / 解析失败 / `status` 不是 frozen | 跳过，记 `skipped[]`（reason 说明是哪一种） |
| 目录存在但 truth.md 不存在 | 跳过，记 `skipped[]` |
| 同一 project root 传入两次 | 只枚举一次 |
| 同一 project 下多个 version | 取 `projects.json` 或 `docs/beacon/` 的最高版本；显式 `--version` 覆盖 |

项目列表由调用方显式给出（`--projects`），不读隐式全局状态。

## Non-goals（冻结边界）

- 不写 beacon 真相（`truth_stage` 只读）
- 不改 studio 需求池文件
- 不做 UI / dashboard
- 不做 Router 派活（router-loop 湖）
- 不做 watchdog 判定（watchdog 湖）
- 不扫描未注册/未冻结的包（只读已冻结真相）
- 不做守护进程（那是 adbd 的事）
