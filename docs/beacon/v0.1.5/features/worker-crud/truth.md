---
slug: worker-crud
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
  truth: docs/beacon/v0.1.5/features/worker-crud/truth.md
  tests: docs/beacon/v0.1.5/features/worker-crud/tests.md
  tasks: docs/beacon/v0.1.5/features/worker-crud/tasks.md
  evidence: docs/beacon/v0.1.5/features/worker-crud/evidence.md
---

# Requirement Truth: worker-crud

## 人话

paseo 里 176 个 agent，现在要靠一堆零散命令去看、去改、去归档。
这个湖给一个统一入口：一条命令列出全部并标出它属于哪个项目，
一条命令看单条详情，一条命令在指定项目里新建，一条命令改名打标换模式，一条命令归档。
**没有硬删除**——归档就够，留痕可查。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "superagent 是把这些先管理起来，支持 paseo worker 增删改查管理吧，一码归一码"
goal: "给 paseo worker 一个统一管理面：查、增、改、归档，并把 worker 映射到项目目录。"
non_goals: "不管 aw 账号与出口；不做需求索引与进度；不做自动分派与看门狗；不提供硬删除；不做 Web 面板。"
constraints: "Go 放 cmd/adbd（stdlib only）；paseo 只走 CLI；只 archive 不 delete；无归属的 worker 不得被改或删。"
scope_mode: "lake"
```

## 动词边界（对着 paseo 实际能力）

| 动词 | 提供 | 不提供 | 原因 |
|------|------|--------|------|
| 增 | `create` 走 `paseo run` | — | paseo 支持 `--cwd/--provider/--model/--mode/--label` |
| 查 | `list` / `show` 走 `ls` / `inspect` | — | paseo 输出 JSON |
| 改 | `name` / `label` / `mode` | `cwd` / `provider` / `model` | paseo 没有这些参数，只能新建 |
| 删 | `archive`（软删） | 硬删除 | 避免一条命令清空一屏 |

## 项目归属

`cwd` → 项目 slug：用 `config/workspaces.json`，取**最长前缀匹配**（子目录也算同一项目）。
`worker create --project <slug>` 用同一张表解析绝对路径；解析不到即拒绝。

## 用户旅程

1. `adbd worker list` 一屏看到全部 worker 与所属项目，`--project studio` 只看一个项目。
2. 某条细节不清楚，`adbd worker show <id>` 看 cwd、状态、待批权限、标签、时间。
3. 需要一个新工人，`adbd worker create --project justin-ecommerce --item <工作项>`，它在项目目录里建好并带归属标签。
4. 名字不好认，`adbd worker update <id> --name ...`；要换模式，`--mode build`。
5. 干完了，`adbd worker archive <id>`，留痕不删除。
6. 想看历史，`paseo ls` 仍能看到归档记录。

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | goal：查 | truth.md#Acceptance-Criteria |
| AC-002 | goal：查详情 | truth.md#Acceptance-Criteria |
| AC-003 | goal：增 | truth.md#Acceptance-Criteria |
| AC-004 | goal：改 | truth.md#Acceptance-Criteria |
| AC-005 | goal：删（归档） | truth.md#Acceptance-Criteria |
| AC-006 | 用户决策：不提供硬删 | truth.md#动词边界 |
| AC-007 | goal：一码归一码，不越权 | truth.md#Acceptance-Criteria |
| AC-008 | goal：映射项目目录 | truth.md#项目归属 |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `adbd worker list [--project <slug>] [--state <state>] [--owned] [--json]` 输出每个 worker 的 `id`、`name`、`project`、`ownership`、`state`、`age`；`project` 由 cwd 最长前缀匹配得出，匹配不到输出 `-`；`--project`、`--state`、`--owned` 三个过滤器可同时生效 |
| AC-002 | `adbd worker show <id>` 输出 `id`、`name`、`cwd`、`project`、`status`、`pending_permissions`、`labels`、`created`、`updated`；id 不存在时输出 `reason_code=agent_not_found` 且退出码 2 |
| AC-003 | `adbd worker create --project <slug> --item <item-id> [--prompt] [--provider] [--model] [--mode]` 在 `adbd_projects_file` 指向的项目表解析出的绝对路径下创建 worker，并携带 `--label adbd.item=<item-id>`；项目 slug 解析不到、目录不存在时输出 `reason_code=project_not_registered` 且退出码 2，且不创建任何 agent |
| AC-004 | `adbd worker update <id> [--name <v>] [--label k=v] [--mode <m>] [--force]` 把这些字段映射到 `paseo agent update` 与 `paseo agent mode`；对 `--cwd`、`--provider`、`--model` 任一参数返回 `reason_code=field_not_updatable` 且退出码 2，并提示用 create 重建；worker 带 `adbd.foreign` 标签且未给 `--force` 时返回 `reason_code=ownership_unknown` 且不执行任何 paseo 写操作 |
| AC-005 | `adbd worker archive <id> [--force]` 调用 `paseo archive`；worker 无 `adbd.item` 标签且无 `adbd.foreign` 标签时返回 `reason_code=ownership_unknown` 且退出码 2；worker 带 `adbd.foreign` 标签且未给 `--force` 时同样返回 `ownership_unknown` 且不调用 paseo；已是归档状态的重复调用退出码 0；每次成功归档向 `.beacon/adbd-worker.jsonl` 追加一行审计 |
| AC-006 | 不存在 `adbd worker delete` 子命令；执行 `adbd worker delete <id>` 时退出码 2 且输出 `reason_code=delete_not_supported`；源码扫描断言 `cmd/adbd` 中不存在 `agent delete`、`delete <id>` 这两个调用串 |
| AC-007 | 对既无 `adbd.item` 也无 `adbd.foreign` 的 worker，`update` 与 `archive` 都返回 `reason_code=ownership_unknown` 且不执行任何 paseo 写操作；`list` 仍可展示它们，`ownership=unknown` |
| AC-008 | 新增 `config/workspaces.json`，结构为 `{"workspaces":[{"slug","path","root","aliases"}]}`；`cwd` 匹配时取路径前缀最长的一条；同一 slug 重复定义、路径不存在、文件非法均返回 `reason_code=workspaces_invalid` 且退出码 2 |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_projects_file` | `<root>/config/projects.local.json` | 项目登记表（供 create 解析路径） |
| `adbd_workspaces_file` | `<root>/config/workspaces.json` | worker 到项目的映射表 |

## 禁止的迁移（人工维护，内核不校验）

- illegal: archived -> active（归档后只能新建，不复活）
- illegal: unknown -> owned（必须先走 adopt）

## Non-goals（冻结边界）

- 不管 aw 的账号、出口、健康
- 不做需求索引与进度视图
- 不自动分派 worker（route 已有，不在本湖）
- 不提供硬删除
- 不改 paseo 本身

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 删除动词怎么提供？ | 只提供 archive 软删；不封装 paseo 的硬删（它支持 --all/--cwd） | answered | 只做 archive，不提供硬删除。 | human |
