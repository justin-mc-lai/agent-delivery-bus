---
slug: projects-import
version: v0.1.5
status: draft
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: draft
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v0.1.5/features/projects-import/truth.md
  tests: docs/beacon/v0.1.5/features/projects-import/tests.md
  tasks: docs/beacon/v0.1.5/features/projects-import/tasks.md
  evidence: docs/beacon/v0.1.5/features/projects-import/evidence.md
---

# Requirement Truth: projects-import

## 人话

有真相的项目有 25 个，router 能找到的只有 1 个。
这个湖做一次扫描登记：把某个目录下所有"有 beacon 真相"的仓库，
写进 router 用的那张表，顺带上报给内核。默认只看不写。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "现在继续补充安装，管理缺口，确保整个superagent闭环"
goal: "扫描目录自动登记项目：router 映射一份、内核注册可选；默认 dry-run，已存在不覆盖。"
non_goals: "不 clone、不迁移真相、不改项目自身；不删已有登记。"
constraints: "Python 侧实现；只写 config/projects.json；内核注册失败不阻断 JSON。"
scope_mode: "lake"
```

## 扫描规则

含 `docs/beacon/<version>/features/` 的直接子目录算一个项目。
`slug` = 目录名；`repo` = 绝对路径；`docs_version` = 最高版本目录。

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `adb projects import --scan <dir>` 扫描 `<dir>` 的直接子目录，仅收含 `docs/beacon/<v>/features/` 的目录；输出 `found=<n>` |
| AC-002 | 默认 dry-run：打印将新增的 `slug` / `repo` / `docs_version`，**不写任何文件** |
| AC-003 | `--apply` 时才写 `config/projects.json`；保留原有 `schema_version`/`adapters`/`notes` 等字段 |
| AC-004 | 已存在同名 `slug` 的条目默认跳过并计入 `existing`；`--overwrite` 时更新其 `repo` 与 `docs_version` |
| AC-005 | `--register-kernel` 时对每个新增项执行 `beacon-go projects register --project <repo> --version <v>`；单项失败不阻断其余，也不阻断 JSON 写入，结果记 `kernel_failed=<n>` |
| AC-006 | `<dir>` 不存在、无符合项、`config/projects.json` 非法时给出明确 `reason_code`（`scan_dir_missing` / `no_projects_found` / `registry_invalid`），退出码 0 |

## 禁止的迁移（人工维护，内核不校验）

- illegal: apply -> dry_run（同一次调用内）

## Non-goals（冻结边界）

- 不 clone 仓库、不迁移真相包
- 不删除任何已有登记
- 不改被扫描项目内的文件

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
| Q1 | 登记到哪一层？ | 先写 config/projects.json（router 用），内核注册做成可选开关 | answered | config/projects.json 必写；--register-kernel 时另调 beacon-go。 | human |
