---
slug: thread-workdir
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
  truth: docs/beacon/v0.1.5/features/thread-workdir/truth.md
  tests: docs/beacon/v0.1.5/features/thread-workdir/tests.md
  tasks: docs/beacon/v0.1.5/features/thread-workdir/tasks.md
  evidence: docs/beacon/v0.1.5/features/thread-workdir/evidence.md
---

# Requirement Truth: thread-workdir

## 人话

派活时只有一个固定的工作目录。可一个 issue 属于 cli/cli，一个需求属于 studio，
工人全被扔进同一个仓库，一伸手就跨目录 → 又要权限审批 → 又卡住。

这个湖让系统按 thread 自己认出该去哪个仓库：先查已有项目登记表，
查不到就退回默认目录，并把"退回了"这件事记下来，不装作成功。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "顺序修复以上缺口、断点，不绕过不回避不拖延"
goal: "按 thread 解析目标仓库：skill 与 gh 线程各自落到对应项目目录；解析不到回落并留痕。"
non_goals: "不自动 clone；不改 paseo workspace；不改 router 决策；不做飞书。"
constraints: "复用 config/projects.json；Go 放 cmd/adbd（stdlib only）；解析失败不阻断派活。"
scope_mode: "lake"
```

## 解析规则

| thread 形态 | 取什么 | 到哪 | 判定 |
|---|---|---|---|
| `skill:<project>:<slug>` | `<project>` | 登记表里 slug 或 alias 等于它的项目 | `repo` 字段解析为绝对路径 |
| `gh:<owner>/<repo>#<n>` | `<owner>/<repo>` | 登记表里 `github` 字段等于它的项目，否则 repo 名等于 `<repo>` 的项目 | 同上 |
| 其它 | — | 默认工作目录 | 记 `workdir_fallback` |

登记表不可读、字段缺失、目录不存在，一律回落默认目录并留痕。

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `threadWorkdirKey(thread)` 解析出投影键：`skill:<p>:<s>` → `p`；`gh:<owner>/<repo>#<n>` → `<owner>/<repo>`；其它形态返回空串 |
| AC-002 | 登记表默认读 `<root>/config/projects.json`，可由 `adbd_projects_file` 覆盖；文件缺失、JSON 非法、`projects` 非数组时返回空映射且不报错（派活继续） |
| AC-003 | 解析到项目且其 `repo` 目录存在时，新建 worker 的 `--cwd` 为该绝对路径；相对 `repo` 以 ADB root 为基准解析 |
| AC-004 | 解析不到、目录不存在时，`--cwd` 回落 `adbd_worker_workdir`（再回落 root），并向 `.beacon/adbd-reports.jsonl` 记一条 `kind=workdir_fallback` 的记录，字段含 `thread`、`reason` |
| AC-005 | 线程已有可用绑定时直接复用，不重新解析目录，也不产生第二条 fallback 记录 |
| AC-006 | 测试用真实 fake paseo 断言：skill 线程的 `--cwd` 是目标项目路径；gh 线程按 owner/repo 命中；未登记线程的 `--cwd` 是默认目录且写了一条 `workdir_fallback` |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_projects_file` | `<root>/config/projects.json` | 项目登记表 |
| `adbd_worker_workdir` | root | 解析不到时的默认工作目录 |

## 禁止的迁移（人工维护，内核不校验）

- illegal: resolved -> fallback（同一线程一次派发内只能二选一）

## Non-goals（冻结边界）

- 不自动 clone 或创建工作树
- 不改 paseo 的 workspace 语义
- 不改 router 的四选一
- 不接飞书

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
| Q1 | 项目登记表复用 config/projects.json，还是新建一份 adbd 专用映射？ | 复用 config/projects.json（已有 slug/repo/aliases），缺字段再补 github | answered | 复用 config/projects.json；可用 adbd_projects_file 覆盖。 | human |
