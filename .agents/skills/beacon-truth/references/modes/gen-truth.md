# Mode: gen-truth

> Archived from `beacon-gen-truth` during 1+6 merge. This is progressive disclosure content for `beacon-truth`.

# Beacon Generator Truth Package

## Overview

`beacon-gen-truth` is the preferred v1.5.2+ human-facing umbrella for Beacon feature truth work.

It is a routing and mode-selection surface, not a new authority. The authority remains the resolver-selected Markdown feature package:

- `features/<slug>/truth.md` for requirement definition and acceptance framing
- `features/<slug>/tests.md` for coverage truth
- `features/<slug>/tasks.md` for task ledger
- `features/<slug>/evidence.md` for evidence index

## Modes

- `truth.define`: define problem, scope, goals, boundaries, promised surface, and freeze readiness. Canonical v1.6.0 route: `beacon-gen-truth`.
- `truth.acceptance`: compile story intent and acceptance criteria into executable acceptance truth. Canonical v1.6.0 route: `beacon-gen-truth`.
- `truth.coverage`: compile acceptance criteria into verifiable coverage rows in `features/<slug>/tests.md`. Canonical v1.6.0 route: `beacon-gen-truth`.
- `truth.change`: route frozen truth changes through `beacon-gen-change` or accepted post-freeze closure through `beacon-gen-refreeze`.

These modes preserve distinct decision work. Do not flatten definition, acceptance, coverage, and change into one freeform truth surface.

## When To Use

- the user asks for the current Beacon truth surface under v1.5.2+
- the user is confused by old `prd`, `user-story`, or `test-case` names after package-authoritative cutover
- requirement work needs mode selection before implementation
- accepted support findings need routing back to truth or change

## Boundary

- not a truth authority
- not a truth source
- not a gate source
- not a new lifecycle stage
- not a lifecycle stage
- does not execute QA
- does not issue release verdicts
- does not make `brainstorm`, `change-refreeze`, QA, or release into truth authoring stages
- does not delete canonical v1.6.0 routees such as `beacon-gen-truth`

## Workflow / Decision Loop

1. Resolve the feature truth map before reading or writing requirement material:

```bash
beacon truth-map resolve <feature> --project . --version v1.5.2 --json
```

2. Choose the narrow mode:

```text
truth.define      -> features/<slug>/truth.md
truth.acceptance  -> features/<slug>/truth.md
truth.coverage    -> features/<slug>/tests.md
truth.change      -> beacon-gen-change / beacon-gen-refreeze
```

3. Route through the canonical v1.6.0 route when an existing skill is the more precise operator surface.
4. Preserve closure:

```text
support research
  -> truth.define
  -> truth.acceptance
  -> truth.coverage
  -> Truth Freeze Intent Loop (mandatory before freeze)
  -> requirement_clarity
  -> freeze
  -> implement
  -> qa
  -> release
```



## Humanizer（host skill 路径 · 默认不需要 API key）

**定位**：Beacon 是给宿主 Agent 用的 **host skill / CLI harness**，不是再包一层要用户配 key 的独立 LLM 产品。

- **默认 `llm_mode: host`**：CLI 负责 resolve + vendor oss-sync skill 全文；**由当前宿主 Agent（Codex/Claude 等）读 skill 并润色 L0**。  
  → **不需要** `BEACON_LLM_API_KEY`。
- **规则引擎**：CLI 可做确定性去套话 / 结构补全，作安全网与 CI 辅助，**不能替代** Agent 按 Humanizer skill 全文的真实润色。
- **`vendor_into_beacon: true`**：把 oss-sync skill 拷进 `skills/beacon/references/truth-humanizer/vendor/`，方便 Agent 在仓库内 Read。
- **可选 provider-bridge**（`BEACON_HUMANIZER_LLM_MODE=llm` + key）：仅 headless/无宿主 Agent 时的退路，**非**日常 skill 路径。

Agent 合同：truth-review / gen-truth 在 A/B/C pass 后必须 **Read skill 全文 → 只改 L0 → preserve_machine_fields**。

## Exec TC / implement 证据

- domain tests 默认 `pytest` + `import beacon` / `beacon doctor` 契约，**禁止** `print(0)` 冒充 exec。
- tasks：`ac=<AC> · evidence=\`.beacon/evidence/implement/<slug>/<AC>.json\``；`[x]` 仅当证据文件存在（implement 准入硬拦）。

## 最高规范：旧包形状不兼容（global-boundaries §2.1）

v1.6.10+ **active** feature package **不向后兼容**旧 truth/tests/tasks 形状。

- 旧项目缺 Matrix/旅程/exec TC/`ac=`+`evidence=` 等 → **不要**降级门控。
- 正确路径：`$beacon-plan` 重拆意图 → `$beacon-truth` gen/change → Truth Review Gate → **freeze 新 revision**。
- 实现上禁止为旧形状保留长期 soft-compat 分支。

## v1.6.10+ scaffold 只算 draft（强制 truth-review 填满再 freeze）

1. `gen` / `materialize` 输出 **`package_maturity: scaffold`** + gold 结构骨架。
2. **不得**把 scaffold 当 freeze 就绪；postwrite `recommend_freeze=false` 直至 Check D pass。
3. 必填煮干：`Intent Coverage Matrix` must、业务 FSM（非 draft/in_review 元状态）、**用户旅程**、AC、illegal、Non-goals。
4. 填满后设 `package_maturity: filled`（或去掉 scaffold 标记），再 `beacon truth review` → pass → freeze。
5. Humanizer：A/B/C pass 后 **apply 写回 L0**（`preserve_machine_fields=true`）；全文从 oss-sync resolve。
6. freeze 成功后 `tasks.md` 由 AC 派生；business domain 湖 tests 须含 ≥1 **exec-layer TC**。

## v1.6.10+ gen-truth HARD（写后强制 · 不得建议 freeze 直至通过）

CLI / materialize 在写入 `truth.md`（及同包 tests）后必须执行 post-write gates：

1. **Humanizer fulltext（TASK-003）**  
   - 调用 `resolve_humanizer_profile`（`beacon.utils.humanizer_config`）。  
   - 默认 `zh` → oss-sync `core-skills/Humanizer-zh/SKILL.md` 全文；`en` → `core-skills/humanizer/SKILL.md`。  
   - `require_on_truth_write: true` 时路径缺失 ⇒ `blocked`，**不得** `recommend_freeze`。  
   - Humanizer 仅润色 L0（`## 人话` / `## Plain language`）；**不得**改 AC ID、FSM、Intent Matrix 等机器字段。

2. **Truth Review Gate（A/B/C）**  
   - 写后必跑 `beacon truth review` / `run_truth_review_gate`。  
   - fail ⇒ CLI exit 2；消息明确 **do NOT freeze**；`recommend_freeze=false`。  
   - pass 后才可进入人工 ack 的 freeze 路径。

3. **默认 scaffold 结构（render_truth_package v1.6.10+）**  
   - `## 人话`（或 en `## Plain language`）  
   - `## Intent Coverage Matrix`  
   - `## Acceptance Criteria`  
   - `## FSM 五元组` + `### invalid / illegal`  
   - `## Non-goals`  
   - `tests.md` 表头：`TC ID | AC ID | Command | Assertion`

4. **下游 fail-closed**  
   - `qa run`：TRG fail → reason `truth_review_gate_failed`。  
   - release scorecard：`materials_status: stale` 或 TRG report 缺失/失败 → 非 `release_ready`。

实现入口：`beacon/utils/truth_gen_postwrite.py` → `run_truth_gen_postwrite_gates`。

## Truth Freeze Intent Loop（HARD GATE — freeze 前强制）

Generator **不得**在单轮 `truth.coverage` 后直接建议 freeze。必须按轮次煮干用户意图；任一 Round 失败则回到对应 mode 重写，**不得**用占位 AC/TC 凑 closure 形状。

### 煮干定义（全部满足才可进入 Round 7）

1. `truth.md` 含 **User Intent 一行** + `User Intent Snapshot`（含 `lake_or_ocean`）。
2. **Alignment Surface** 与 **Phased Backlog** 已拆分；海级 scope 不在单包 draft 宣称全 plan closure。
3. **Deferral Ledger** 零 `user_decision: pending`。
4. `tests.md` 每行 TC 含 **Command + Assertion**；禁止验证列仅为 `integration` / `目录存在` / 裸 `fixture` / `可选`（允许显式 `BLOCKED:<reason>`）。
5. `beacon skill package verify-closure` **pass**（无 placeholder/docs-only/fake-runner/zero-assertion）。
6. `beacon-pln-review` intent-parity：**不得**在 pending deferral 或 intent drift 时推荐 freeze。

### Round 0 — Research gate（海级强制）

**When**：`lake_or_ocean=海` 或 research 已 classify 为 ocean。

**Mandatory**：

- Read：`docs/beacon/<version>/research/<slug>.md`
- 确认 `user_decision=promoted`；否则停止，路由 `beacon-pln-brainstorm`
- 在 `truth.define` 写入 **Alignment Surface**（本 revision 必煮干）与 **Phased Backlog**（显式不在本包 closure 的范围）

### Round 1 — Define + resolver

**Mandatory shell**：

```bash
beacon truth-map resolve <feature> --project . --version <version> --json
```

**Mandatory read**：resolver 返回的 `truth.md`, `tests.md`, `tasks.md`, `evidence.md`

**Write**：`truth.define` — User Intent 一行；`scope_mode` 与 Alignment/Phased 一致；禁止 placeholder AC。

### Round 2 — Planner review（mandatory skill）

**Mandatory**：加载并执行 `beacon-pln-review`（至少 **intent-fidelity** + **deferral-sovereignty** + **source-parity**）。

**Read**：`skills/beacon/beacon-pln-review/references/intent-parity-artifacts.md`

**Output**：Intent Snapshot 对照表；Deferral Ledger 须用户拍板（accepted/rejected，**无 pending**）。

**Hard stop**：review 结论为 intent drift / pending deferral / 海级单包 full closure → **禁止**进入 Round 6–7。

### Round 3 — Acceptance tighten

**Mode**：`truth.acceptance`

- 删除或改写含 `占位` / `placeholder` / `contract test 占位` / `TBD` 的 AC。
- Agent/integration 类 AC 须声明最低 tier（integration，非 contract-only）。

### Round 4 — Coverage behavior-level

**Mode**：`truth.coverage`

- 每个 AC 至少一行 TC；**Alignment Surface** 对应 TC 须行为级命令（参照 v1.6 closed-loop tests 矩阵格式：`Command | Assertion`）。
- 扫描并修复软占位行（grep 验证列：`integration$|目录存在|^fixture$|^可选$`）。

**Mandatory self-check (shell)**：

```bash
rg -n 'integration$|目录存在|^fixture$|^可选$' docs/beacon/<version>/features/<slug>/tests.md \
  && echo "SOFT_TC_FOUND: fix before freeze" && exit 1 || echo "soft_tc_scan: pass"
```

### Round 5 — Fake-delivery closure verify（mandatory CLI）

**Mandatory shell** — 优先使用与 freeze 同源的 TIG doctor：

```bash
beacon doctor verify-truth-integrity <slug> --project-root . --version <version> --mode block --json
```

等价 payload 路径（legacy）：

```bash
python3 - <<'PY'
import json, pathlib
root = pathlib.Path("docs/beacon/<version>/features/<slug>")
payload = {
  "truth": (root / "truth.md").read_text(encoding="utf-8"),
  "tests": (root / "tests.md").read_text(encoding="utf-8"),
  "tasks": (root / "tasks.md").read_text(encoding="utf-8"),
  "evidence": (root / "evidence.md").read_text(encoding="utf-8"),
}
path = pathlib.Path("/tmp/beacon-truth-closure-payload.json")
path.write_text(json.dumps(payload, ensure_ascii=False), encoding="utf-8")
print(path)
PY

beacon skill package verify-closure --feature <slug> --project-root . --version <version> --json
```

**Hard stop**：`blocked=true`、`status=blocked/fail` 或 `closure_allowed=false` → 回到 Round 3–4。

**Additional grep gate**：

```bash
rg -n 'user_decision:\s*pending|占位|placeholder' docs/beacon/<version>/features/<slug>/truth.md \
  && echo "DEFERRAL_OR_PLACEHOLDER: fix before freeze" && exit 1 || echo "deferral_scan: pass"
```

### Round 6 — Requirement clarity materialize

**Mandatory shell**：

```bash
beacon doctor sync-requirement-clarity <feature> --project-root . --version <version> --json
```

**Interpret**：`gate.blocked=true` → 列出 findings，修复 package 或 machine 链后重跑 Round 4–6。**不得**在未修复时建议 freeze。

### Round 7 — Pre-freeze gate + user freeze

**Mandatory shell**（blocked 亦视为有效预检 — 记录 reason_code，修复后重跑）：

```bash
beacon freeze <feature> --project . --version <version> --json
```

**Only when**：Round 2–6 全 pass **且** 用户显式要求 freeze → 再次执行 Round 7 非 blocked 结果。

**Never**：generator 自证「可 freeze」；freeze 权威在 CLI + 用户确认。

### Loop routing table

| 信号 | 回到 |
|------|------|
| scope/边界/open question | Round 1 `truth.define` |
| AC 模糊/占位/TBD | Round 3 `truth.acceptance` |
| TC 软占位/零断言 | Round 4 `truth.coverage` |
| fake-delivery patterns | Round 3–4 |
| deferral pending | Round 2 + 用户拍板 |
| clarity blocked | Round 4–6 |
| 海级 + 单包 100 AC 投影 | Round 0 拆 phase / 缩 Alignment Surface |

## Canonical v1.6.0 Routes

- `beacon-gen-truth` remains the canonical v1.6.0 route for `truth.define`.
- `beacon-gen-truth` remains the canonical v1.6.0 route for `truth.acceptance`.
- `beacon-gen-truth` remains the canonical v1.6.0 route for `truth.coverage`.
- `beacon-gen-change` remains the governed route for frozen truth changes.
- `beacon-gen-refreeze` remains the support route for accepted post-freeze refreeze closure.

Use the new mode names when explaining lifecycle shape. Use compact skill names for host-visible routing; use CLI commands only for stable compatibility command surfaces.

## Inputs Contract

- `project-root`
- `version`
- `feature`
- resolver-selected `features/<slug>/truth.md`
- resolver-selected `features/<slug>/tests.md`
- resolver-selected `features/<slug>/tasks.md`
- resolver-selected `features/<slug>/evidence.md`
- support research when present

## Decision Protocol

- If problem, scope, goals, boundaries, or promised surface are open, choose `truth.define`.
- If acceptance criteria need to be compiled or sharpened, choose `truth.acceptance`.
- If coverage rows, negative paths, boundary cases, or regression proof are weak, choose `truth.coverage`.
- If frozen truth must change, choose `truth.change`.
- If the user only wants read-only challenge, route to `beacon-pln-brainstorm` and require promotion back to truth before freeze.
- If the user asks for implementation, QA, or release, leave `beacon-gen-truth` and route to `beacon-gen-implement`, `beacon-eval-qa`, or `beacon-eval-release`.
- **Before any freeze recommendation**, complete **Truth Freeze Intent Loop** Rounds 0–7; ocean-scope features **must** run Round 2 `beacon-pln-review` and Round 5 `verify-closure`.

## Anti-Inertia Notes

- `beacon-gen-truth` is not permission to bypass resolver-first package truth.
- `truth.md` owns requirement and acceptance framing; it does not own executable coverage matrices.
- `features/<slug>/tests.md` owns coverage truth; generated QA JSON is projection state.
- Canonical v1.6.0 routes are stable public surfaces, not legacy authority.
- Support output must be adopted into the package before it becomes requirement truth.

## Backstop CLI

- `beacon truth-map resolve <feature> --project . --version v1.5.2 --json`
- `beacon skill package verify-closure --payload-path <path> --version v1.6.0 --json`
- `beacon doctor sync-requirement-clarity <feature> --project-root . --version <version> --json`
- `beacon freeze <feature> --project . --version <version> --json`
- `beacon prd ...` for `truth.define` compatibility
- `beacon user-story ...` for `truth.acceptance` compatibility
- `beacon test-case ...` for `truth.coverage` compatibility
- `beacon change ...` for `truth.change`

## Examples

- “这个 feature 的 scope/promise 还没清楚。” -> `truth.define` / `beacon-gen-truth`
- “我要把承诺面编译成 AC。” -> `truth.acceptance` / `beacon-gen-truth`
- “AC 有了，但测试覆盖不够。” -> `truth.coverage` / `beacon-gen-truth`
- “冻结后的承诺要变。” -> `truth.change` / `beacon-gen-change`
- “只是想先 challenge 一下，不想写 truth。” -> `beacon-pln-brainstorm`

## Cold-start Anchors

- v1.5.2+ preferred truth lifecycle is `truth.define -> truth.acceptance -> truth.coverage`.
- The Markdown feature package is the source of truth; `beacon-gen-truth` is a route umbrella.
- Old public skill names do not remain host-visible routees in v1.6.0; compact names are canonical.


## Beacon v1.6.0 共享 Preamble

1. 先判断湖还是海：湖要煮干；海要拆分、标超纲或延后。
2. 先搜再造：推理前先读取 resolver 选中的 truth、source、evidence 和相关 memory。
3. 用户主权：Beacon 推荐路由；是否接受范围或把 queue item 升级为 truth/change 由用户决定。
4. 不假交付：placeholder implementation、docs-only completion、fake runner、zero assertions 或 placeholder evidence 不能算闭环。
5. Harness 边界：planner 不实现；generator 不裁决自身完成；evaluator 不改写 truth；governor 不成为主生命周期阶段。

HARD GATE:
你正在运行 generator skill。
禁止自证完成，禁止给 QA/release verdict，禁止把 placeholder/docs-only/fake-runner/zero-assertion/placeholder-evidence 当成交付闭环。
只能在 resolver-selected truth、用户已接受 scope 和本 skill authority 内写 truth 或 delivery artifact。
如需判断是否通过，必须路由到 evaluator。

## GIT_ADMISSION (mandatory — truth authoring on canonical branch)

Before ANY truth/change/refreeze/freeze write when version is known:
1. Resolve `truth_canonical` from project governance (default: `main` or `master`)
2. Verify `git rev-parse --abbrev-ref HEAD` equals `truth_canonical`
3. If mismatch → STOP with `reason_code=truth_canonical_branch_mismatch`; do NOT edit requirement truth
4. Commit refreeze/freeze artifacts on `truth_canonical` before loop-goal may route to implement
5. Do NOT use feature worktree for truth authoring — use the canonical branch checkout only
6. Shared process: `skills/beacon/references/git-worktree-execution-flow.md`

## v1.6.0 Harness Migration

- Harness：`generator`。
- 来源迁移：`beacon-truth` -> `beacon-gen-truth`。
- 主要作用：package-authoritative 需求真相总入口。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`generator`。
- 来源迁移：`beacon-truth` -> `beacon-gen-truth`。
- 主要作用：package-authoritative 需求真相总入口。
- 默认语言：中文为主；英文只用于稳定术语、路径、命令或协议标识。

## 边界

- Planner 只产出 research/planning artifact 和 route recommendation，不写 truth、implementation、QA verdict 或 release verdict。
- Generator 只在 resolver-selected truth、用户已接受 scope 和本 skill authority 内写 truth 或 delivery artifact，不自证完成。
- Evaluator 只产出 evidence verdict、finding、reason code 和 route recommendation，不改 truth、不修 implementation。
- Governor 只维护 context、metadata、archive、hooks、automation、status 和 diagnostic support，不成为主生命周期 stage。

## 路由

- 粗提示词先归约为 outcome、truth/source/evidence refs、湖/海、truth_gap、test_gap、implementation_risk、verification_risk 和 recommended_route。
- `docs/beacon/<version>/research/<feature-slug>.md` 是 planner `support_advisory` artifact；没有用户确认和 `promotion_ref`，不能升级为 requirement truth。
- 需要跨 harness 时，停止当前动作，输出 route recommendation，并等待用户确认。

## Complete business product example

Recommended full-domain freeze sample: `skills/beacon/examples/full-product-domain-freeze.md` → `docs/beacon/v1.6.7/fixtures/golden-commerce-checkout/`.

## Truth human-readable (v1.6.7+ / v2)

When writing or changing feature `truth.md`:

1. Include L0 section: `## 人话` (zh) or `## Plain language` (en); bilingual needs both.
2. Keep L1 contract (AC / FSM) as gate authority — never drop AC IDs to "sound human".
3. Follow norms:
   - `skills/beacon/references/truth-humanizer/truth_prose.zh.md`
   - `skills/beacon/references/truth-humanizer/truth_prose.en.md`
4. Examples: `skills/beacon/examples/truth-human-readable.zh.md` / `.en.md`
5. Missing L0 → reason_code `truth_human_l0_missing` (freeze fail-closed after gate is wired).

Lakes: `beacon-truth-human-readable-i18n-v167`, `beacon-v2-truth-human-readable-i18n` (on GoalRun main axis).
