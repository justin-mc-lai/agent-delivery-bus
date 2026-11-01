---
slug: requirement-index
version: v0.1.5
materials_status: current
task_source: acceptance_criteria
plan_mode: feature_projection_from_program
program_ref: docs/beacon/v0.1.5/programs/superagent-router
program_phase: P1
lake_priority: P0
ac_source: provisional_plan_ids
upstream_truth_readonly: true
---

# Tasks: requirement-index (v0.1.5)

> **这份文档是什么**：program `superagent-router` 的第 1 个湖（`requirement-index`）的
> **功能级实施计划**。program 层的图、P2 评审、模块边界已经冻结，这里不重规划，
> 只把湖内部切到「可一个一个写代码 + 可一条一条验收」的粒度。
> **不是什么**：不是 truth（AC 正式措辞在 truth 阶段落地），不是实现，不是 QA 结论。

## 人话：这个湖干什么

ADB 要当常驻前台，就必须在看一眼的地方知道「公司现在有哪些需求、各自走到哪一步」。
现在这些信息散在 5 个仓库的 beacon 文档里，人要看 5 遍、口径还不一样。

这个湖做一件事：把这些已经写死的事实**汇总成一个 JSON 文件**，让 Router 只读这一个文件。

- **能做**：读 beacon 冻结物 + studio 需求池，生成/校验/查询 `requirement-index.json`；
  记录「执行到哪一步」并单独存在 ADB 自己的 SQLite 里；超过 10KB 自动分层。
- **不能做**：改 beacon 的真相（`truth_stage` 只读）、改 studio 需求池、做 UI、
  自己判断「这条需求该不该继续」（那是 router-loop 的事）。
- **怎样算完**：6 条 AC 各有行为测试；同一输入跑两遍输出一致；缺数据全部有记录不静默；
  越权改真相被明确拒绝并给出 reason_code；单文件可被 `python3 -c "json.load(...)"` 直接读。

## 一句话边界（本湖最硬的一条）

> **真相只能读，执行态只能写自己家的。**

来自 program P0 `R-P0-02` 与用户决策 `UD-SAR-007`。落到代码就是两个字段、两种写权：

| 字段 | 谁能写 | 从哪来 | 越权后果 |
|------|--------|--------|----------|
| `truth_stage` | ❌ 谁都不可写，只做**证据推导** | beacon 冻结物（见下方阶梯） | `reason_code=truth_stage_readonly`，整笔写入拒绝 |
| `exec_state` | ✅ ADB 写 ADB 的 SQLite | worker 汇报 / 人工 | 枚举与迁移必须合法，否则 `exec_state_invalid` |

## 上游输入（只读，已在源码里确认存在）

| # | 源 | 具体路径（相对 `project.repo`） | 用途 | 缺失时 |
|---|----|--------------------------------|------|--------|
| S1 | 项目注册表 | `config/projects.json` / `config/projects.local.json` → `ProjectRegistry.load()` | 项目枚举（`repo`/`docs_root`/`docs_version`/`status`） | 已由 registry 现有门控阻断（`registry_missing`） |
| S2 | 版本 feature 清单 | `<docs_root>/<docs_version>/.machine/manifest/version-features.json` | 该版本有哪些 feature | 回退扫 `features/*/truth.md`，记 `manifest_missing` |
| S3 | 需求包机器投影 | `<docs_root>/<version>/.machine/requirement/<slug>.requirement-package.json` | `truth_frontmatter.status`、`acceptance_criteria[]`、`task_rows[].status`、`source_hashes` | 回退读 `truth.md` frontmatter，记 `machine_projection_missing` |
| S4 | 冻结修订 | `<docs_root>/<version>/.machine/execution/<slug>.revision.json` | `revision_id`/`doc_status`/`frozen_at` | 记 `revision_missing`（注意：`version-features.json` 里声明的 `freeze_state` 在本仓 v0.1.3 实际不存在，属已知缺口） |
| S5 | QA 证据 | `<docs_root>/<version>/.machine/qa/<slug>*.json` | 是否有 `pass` 的 qa 矩阵 | 记 `qa_missing` |
| S6 | Studio 需求池 | `<studio>/requirements/pool.md`（本机实际路径见 `data/` 之外的配置，**不进 git**） | 池内 `REQ-xxx` 的需求行、状态节、绑定、证据 | 记 `studio_pool_missing`，不静默跳过 |

**S3 的关键约束**：该文件名里写着 `machine_json_role: generated_state_not_truth`。
所以它只是**缓存**。读它必须先校验 `source_hashes.truth` 与 `truth.md` 实算 sha256：
不一致 → 记 `stale_machine_projection` 并改用 `truth.md` frontmatter 为准（真相永远赢缓存）。

## `truth_stage` 推导阶梯（只读、只用证据存在性、单调）

| 判据（按序取第一个命中） | `truth_stage` |
|---|---|
| `truth.md` 不存在 | `unknown` |
| frontmatter `status != frozen` | `plan` |
| `status == frozen`，无 implement 证据目录 | `truth` |
| `.beacon/evidence/implement/<slug>/manifest.json` 存在且 `pass` | `implement` |
| `.machine/qa/<slug>*.json` 中任一 `pass: true` | `qa` |
| `docs/<version>/release/` 下有该 feature 的放行物且人工门已记 | `release` |

规则：
1. **只能从 `exec_state` 之外的信息推导**——`exec_state` 永远不许反向抬高 `truth_stage`。
2. 每一级都写 `truth_stage_basis[]`（命中了哪个文件），人不用猜。
3. 任一判据文件不可读 → `unknown` + `missing[]` 记账，**不猜、不兜底成 `truth`**。

## 输出契约（`requirement-index.v1`，草案，truth 阶段定稿）

```json
{
  "schema_version": "requirement-index.v1",
  "generated_at": "2026-11-01T07:41:10+00:00",
  "content_hash": "sha256:...",
  "format": "full",
  "budget_bytes": 10240,
  "stats": {"requirements": 12, "projects": 5, "unknown_truth_stage": 2, "conflicts": 1},
  "missing": [{"scope": "project:studio", "kind": "pool_missing",
               "reason_code": "studio_pool_missing", "detail": "..."}],
  "conflicts": [{"req_id": "req:studio:REQ-003", "kind": "duplicate_req_id",
                 "sections": ["Routed", "Done"]}],
  "requirements": [{
    "req_id": "req:<project_slug>:<feature_slug>",
    "source": "beacon",
    "project_slug": "agent-delivery-bus",
    "title": "...",
    "version": "v0.1.5",
    "truth_stage": "plan",
    "truth_stage_basis": [{"artifact": "docs/beacon/v0.1.5/features/requirement-index/truth.md",
                           "present": true}],
    "exec_state": "unknown",
    "task_progress": {"checked": 0, "total": 11},
    "refs": {"truth": "...", "tasks": "...", "revision": "", "qa": ""}
  }]
}
```

- **`req_id` 命名**：`req:<project_slug>:<feature_slug>`（beacon 源）、`req:studio:<REQ-ID>`（池源）。
  版本不进 id——版本升级后 `exec_state` 必须还能接上，不因 v0.1.5→v0.1.6 而丢。
  **本命名要与 `intake-fastpath` 共用**（program 依赖声明），签名在本湖冻结。
- **确定性**：`requirements[]` 按 `req_id` 升序；`missing[]`/`conflicts[]` 按
  (`reason_code`, `scope`) 升序；所有 dict 键固定顺序输出。
  `generated_at` **不参与** `content_hash`，否则做不到幂等。
- **默认落盘**：`data/requirement-index.json`（`data/` 已 gitignore，属派生状态，不是真相）。

## 10KB 分层规则（AC-RI-006）

1. 计算 `format=full` 序列化后的字节数。
2. `<= budget_bytes`（默认 10240，可由配置覆盖）→ 只写单文件。
3. `>` → 写三个面：`requirement-index.summary.json`（计数 + 每项目指针 + 全部 `missing`/`conflicts`）、
   `projects/<slug>.json`（该项目全量行）、保留 `requirements` 为空数组。
4. 分层判定必须是**同一个输入同一个结论**（纯函数：bytes → format），不允许「跑两次一次分层一次不分层」。

## Task Ledger

- [ ] TASK-001 定义 `requirement-index.v1` 契约与骨架：新增 `src/agent_delivery_bus/requirement_index.py`（`SCHEMA_VERSION`、`TRUTH_STAGES`/`EXEC_STATES` 枚举、`req_id` 构造与解析、稳定排序键、`content_hash`），配套 `tests/test_requirement_index_contract.py` · ac=AC-RI-001 · evidence=`.beacon/evidence/implement/requirement-index/AC-RI-001.json`
- [ ] TASK-002 beacon 源读取器：`_read_version_features()`(S2) + `_read_requirement_package()`(S3，含 `source_hashes.truth` 实算 sha256 比对与 `truth.md` frontmatter 回退) + `_read_revision()`(S4) + `_read_qa()`(S5)，全部只读、不写 `.machine/` · ac=AC-RI-001 · evidence=`.beacon/evidence/implement/requirement-index/AC-RI-001.json`
- [ ] TASK-003 缺失与冲突台账：所有读不到/读不动的源写 `missing[]`（`reason_code` + `scope` + `detail`）；重复 `REQ` id（studio 池实测 `REQ-003` 在 `Routed` 与 `Done` 各出现一次）写 `conflicts[]` 且**不替用户选一个**；`--strict` 下 `missing`/`conflicts` 非空即 blocked · ac=AC-RI-003 · evidence=`.beacon/evidence/implement/requirement-index/AC-RI-003.json`
- [ ] TASK-004 双列写权：`storage.py` 升 `SCHEMA_VERSION=4` + `MIGRATIONS` 追加 `(4, "requirement_exec_state", ...)` 建 `requirement_exec_state`（`req_id` PK、`exec_state`、`actor`、`updated_at`）与 `requirement_exec_events`（append-only 审计：`from→to`、`reason_code`）；`set_exec_state()` 拒收 `truth_stage`/beacon 字段（`truth_stage_readonly`），非法迁移拒收（`exec_state_invalid`）· ac=AC-RI-005 · evidence=`.beacon/evidence/implement/requirement-index/AC-RI-005.json`
- [ ] TASK-005 `truth_stage` 只读推导阶梯实现（上文表格逐级命中 + `truth_stage_basis[]`）；断言 `exec_state` 变化不改变 `truth_stage`；断言缺文件得 `unknown` 而非猜测值 · ac=AC-RI-005 · evidence=`.beacon/evidence/implement/requirement-index/AC-RI-005.json`
- [ ] TASK-006 幂等生成 + 原子写：同输入两次运行除 `generated_at` 外字节一致（`content_hash` 相同）；写入走同目录临时文件 + `os.replace`；并发两进程写同一路径不留半文件；已存在文件内容相同则跳过重写 · ac=AC-RI-002 · evidence=`.beacon/evidence/implement/requirement-index/AC-RI-002.json`
- [ ] TASK-007 CLI 面（`argparse` subparser + 现有 `envelope` 约定）：`adb requirement index [--project] [--out] [--format full|summary] [--strict] [--json]`、`adb requirement show <req_id>`、`adb requirement check [--strict]`（只校验不写）、`adb requirement exec-state <req_id> --state <s> [--actor]` · ac=AC-RI-004 · evidence=`.beacon/evidence/implement/requirement-index/AC-RI-004.json`
- [ ] TASK-008 10KB 分层：纯函数 `choose_format(payload_bytes, budget) -> full|summary`，按上文三面落盘，summary 带每项目指针与计数；超限/未超限各一条测试 · ac=AC-RI-006 · evidence=`.beacon/evidence/implement/requirement-index/AC-RI-006.json`
- [ ] TASK-009 只读副作用证明（反造假）：跑 `index` 两遍后，`docs/beacon/**/.machine/**` 与 studio `pool.md` 的 sha256 不变、`git status --porcelain` 为空；`set_exec_state` 只动 `data/*.sqlite3`；把这条写成可重跑脚本测试而非人工核对 · ac=AC-RI-002 · evidence=`.beacon/evidence/implement/requirement-index/AC-RI-002.json`
- [ ] TASK-010 回归：既有 44 个测试文件全绿（`python3 -m pytest -q`），schema 迁移 v3 库可升到 v4 且幂等 · ac=AC-RI-001 · evidence=`.beacon/evidence/implement/requirement-index/AC-RI-001.json`
- [ ] TASK-011 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/requirement-index/GATES.json`
- [ ] TASK-012 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/requirement-index/QA-MATRIX.json`

### M0 增量（change `chg_17fc1e57ef36` → AC-007 / AC-008）

> **人话**：TASK-001..008 只证明了「给一批数据，能生成 JSON」。
> 谁去把这批数据找出来？没人写。`adb index ...` 这条命令？也没有。
> 所以现在缺两块：**自己找包的枚举器**（AC-007）+ **一条命令跑完的入口**（AC-008）。
> truth 与 tests 已按这两条冻结，本文件只补台账，不重写 AC 措辞。

- [ ] TASK-013 枚举器：`requirement_index.py` 新增 `EnumerationResult` + `enumerate_sources(project_roots, *, version="")`——扫 `<root>/docs/beacon/<version>/features/<slug>/truth.md`，只收 frontmatter `status: frozen` 的包，产出 `RequirementSource`（`req_id=req_id_for(project, feature)`；`project` 取 root 目录名，允许 CLI 传 `slug=path` 显式覆盖；`payload_path` 优先指向 `<feature>/.machine/requirement/<slug>.requirement-package.json`，该文件不存在时仍产出 source，由 `build_index` 按 AC-003 记 `source_missing`，**不伪造 lifecycle**）。版本选择：显式 `--version` 优先，否则取 `docs/beacon/` 下最高 `vX.Y.Z`；同一 root 重复传入按 `Path.resolve()` 去重只枚举一次；未冻结 / 无 frontmatter / frontmatter 不可读 / truth.md 缺失 / root 不存在 → 记入 `skipped[]`（`reason` + `project` + `path`）后继续，**不抛栈**；输出按 `(project, feature)` 固定排序（幂等前提）。配套 `tests/test_requirement_index.py -k enumerator`（TC-RI-007）与 `-k enumerator_missing_root`（TC-RI-ILL-003）· ac=AC-007 · evidence=`.beacon/evidence/implement/requirement-index/AC-007.json`
- [ ] TASK-014 `adb index` CLI：`cli.py` 新增 `index` 子命令组，复用 `envelope()` 与 `main()` 的 blocked→exit 2 约定。① `adb index generate --projects a,b[,slug=path] --out <path> [--version] [--budget] [--strict] [--json]`：枚举 → `generate_index(storage=Storage)` 合并 ADB `exec_state` → `write_index()` 原子落盘 → `status=pass` + exit 0，`skipped[]`/`missing[]` 计数进 `data`（`--strict` 下非空即 blocked，沿用 TASK-003 语义）。② `adb index status --out <path> [--json]`：打印 `requirements` 条数、`format`、`missing` 计数；文件不存在 → `DeliveryBusError("index_file_missing")` 经 `main()` 返回 **exit 2**，JSON 解析失败 → `index_file_invalid`（同样 exit 2）。③ 时钟走 `requirement_index.CLOCK_ENV`（`ADB_REQUIREMENT_INDEX_NOW`）：同一输入两次 generate **字节一致**。④ 副作用边界：generate 不写 `docs/beacon/**`、不碰 `truth_stage`（TASK-009 复跑校验）。配套 `tests/test_requirement_index.py -k cli`（TC-RI-008）· ac=AC-008 · evidence=`.beacon/evidence/implement/requirement-index/AC-008.json`

## 依赖顺序（一个方向，不绕圈）

```text
TASK-001 契约
   ├─▶ TASK-002 读 beacon 源 ─▶ TASK-003 缺失台账
   ├─▶ TASK-004 写权/迁移 ─▶ TASK-005 truth_stage 阶梯
   ├─▶ TASK-006 幂等/原子写 ─▶ TASK-008 10KB 分层
   └──────────────────────▶ TASK-007 CLI（依赖 002/003/005/006/008 的入口）
TASK-009 副作用证明（依赖 007）
TASK-010 回归（全绿后）
TASK-011 门 ─▶ TASK-012 QA

M0 增量（change chg_17fc1e57ef36）：
TASK-001/002 已有 ─▶ TASK-013 枚举器 ─▶ TASK-014 adb index CLI ─▶ TASK-009 副作用证明复跑
```

先做 001（契约），是因为 002–008 都在写同一个 schema；契约返工会全返工。
007 放最后，因为 CLI 只是入口，逻辑全在 001–008。
M0 里 013 先于 014：CLI 的 `--projects` 只是把参数交给枚举器，找包的逻辑只有一份。

## AC 映射（program parity_subset → 本湖 AC）

| program 能力键 | 本湖 AC（truth 阶段定稿措辞） | 主要 TC 方向（`tests.md` 在 truth 阶段写） |
|----------------|------------------------------|------------------------------------------|
| `requirement-index-schema` | AC-RI-001 | 字段必填/枚举/排序/`req_id` 唯一；缓存过期回退 |
| `projection-idempotent` | AC-RI-002 | 两次运行 `content_hash` 相同；并发写无半文件；零副作用 |
| `missing-not-silent` | AC-RI-003 | 缺源全进 `missing[]`；重复 REQ 进 `conflicts[]`；`--strict` blocked |
| `single-file-parseable` | AC-RI-004 | `adb requirement index --json` 输出可直接 `json.load`；`check` 不写盘 |
| `truth-stage-readonly-exec-state-writable` | AC-RI-005 | 越权 `truth_stage` 被拒；`exec_state` 与 `truth_stage` 不互相污染 |
| `index-budget-layering-at-10kb` | AC-RI-006 | 超限分层三面齐全；阈值边界（恰好 10240 / 10241） |

> AC 编号是**计划期占位**，truth 阶段必须以这 6 个编号落定（改名则本台账同步重生成）。
> 上表 AC-RI-001..006 现已在 truth 中落成 **AC-001..AC-006**（本文件历史行保留旧占位名，执行时按 AC-00x 记账）。

| program 能力键（M0 补强） | 落定 AC | 主要 TC 方向 |
|---------------------------|---------|--------------|
| `requirement-index-schema` + `missing-not-silent` | AC-007 | TC-RI-007：两个假项目树只收 frozen 包、`req_id=req:<project>:<feature>`、未冻结/无 frontmatter 进 `skipped[]`、重复 root 只枚举一次；TC-RI-ILL-003：不存在的 root 进 `skipped[]` 不抛栈 |
| `single-file-parseable` + `missing-not-silent` | AC-008 | TC-RI-008：`adb index generate` 落盘并 exit 0；`status` 打印条数/格式/missing 计数；文件缺失 exit 2 + `index_file_missing`；固定 `ADB_REQUIREMENT_INDEX_NOW` 后两次生成字节一致 |

## 本湖失败矩阵（program 失败矩阵的湖内投影）

| 失败 | 现象 | 本湖处置 | 不做的兜底 |
|------|------|----------|------------|
| `.machine/` 缓存与 `truth.md` 不一致 | index 显示过时阶段 | 记 `stale_machine_projection`，改读 `truth.md` | 不「取新者」猜测 |
| 项目仓库不存在 / 被 archive | 某项目整块缺失 | 按 registry `status` 标 `archived`，不进 `missing` 噪音 | 不静默删行 |
| studio 池路径未配置 | 池源整体缺失 | `studio_pool_missing` 一条，其余源照常出 | 不伪造空池 = 健康 |
| 池内 REQ id 重复 | 一条需求两个状态 | `conflicts[]` 记录，两处都不采信 | 不按节顺序选第一个 |
| index 越写越大 | 撑爆 Router 上下文 | 10KB 分层，summary 常驻 | 不靠「以后再说」 |
| 有人想直接改 `truth_stage` | 出现第二真相源 | 写入拒绝 + `truth_stage_readonly` | 不留后门开关 |
| 并发两个 CLI 生成 | 文件损坏 | 临时文件 + `os.replace` | 不加全局锁（无必要） |

## Open Questions（交 truth 阶段一问一答，计划不代答）

| Q | 问题 | 计划倾向 | 影响 |
|---|------|----------|------|
| Q-RI-1 | index 是全公司一个文件，还是 per-project 一个再汇总？（program 遗留项） | 一个文件 + 10KB 分层（分层天然解决规模） | 影响 CLI 默认路径与 summary 结构 |
| Q-RI-2 | studio 池路径放 `config/projects.json` 顶层 `requirement_index` 块，还是 CLI `--studio-pool`？ | 配置块（可被 `registry.raw` 保留，`save()` 不丢） | 影响配置契约 |
| Q-RI-3 | `exec_state` 取值集合是否与 `router-loop` 完全共用？ | 共用一套枚举，本湖先冻结 | 影响跨湖契约 |
| Q-RI-4 | 池内 `绑定: <slug> (vX feature <slug>)` 的解析规则（正则 vs 结构化字段） | 正则 + 解析失败进 `missing` | 影响池源可用性 |
| Q-RI-5 | 10KB 预算是 `full` 还是 `summary` 的字节数？ | `full`（否则分层阈值失效） | 影响 AC-RI-006 断言口径 |

## Non-goals（本湖明确不做）

- 不做 studio 只读 index 页面（program 已 defer）。
- 不写 beacon `.machine/`、不改 `truth.md`/`tests.md`、不动 studio `pool.md`。
- 不做 HTTP 服务、不做定时守护（那是 router-loop / watchdog）。
- 不判断「需求该不该继续」，不产生 dispatch，不碰审批。
- 不为旧形状的历史包做兼容分支（global-boundaries §2.1：旧包要上船就重 plan）。

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).

- 每个任务行必须同时有 `ac=` 与 `evidence=`；允许一 AC 多任务（TASK-001/002/010 同属 AC-RI-001）。
- `AC-RI-00x` 编号由 truth 阶段签认；签认后本文件的 AC 列同步重生成，不留双份编号。
- 本文件不构成 freeze、不构成 QA 结论；`status: draft` 时不得作为实现准入依据。
