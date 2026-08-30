# Beacon 全局约束

## 目标

这份文档只保留 Beacon 在跨版本、跨宿主、跨目标项目下都必须成立的原则性边界。

它不描述当前版本怎么实现，不承载临时 runtime 细节，也不记录某一条产品线或某一个内部角色的局部机制。

收敛标准：

> 凡是会随着版本、runtime、命令包装、内部编排方式变化而变化的内容，都不应进入 `global-boundaries`；只有不可越过的原则，才应留在这里。

## 1. 主流程与支持面边界

Beacon 的人类主流程必须保持收敛，不得因内部能力增强而持续膨胀。

默认主链：

```text
prd
→ user-story
→ test-case
→ implement
→ qa
→ release
```

补充规则：

- `prototype` 只可作为条件触发层插入 `test-case` 与 `implement` 之间，不得被默认抬升为所有 feature 的常驻主段。
- `ceo-review`、`brainstorm`、`design-review`、`deep`、`help`、`status`、`doctor`、`archive` 等都属于 support surface，不是新的 lifecycle stage。
- `archive` governs historical requirement isolation and version-truth catalog commits; it does not replace `change` for editing frozen feature truth.
- support surface 可以 challenge、explain、route、diagnose，但不能默认替代 truth surface、execution surface 或 release gate。

收敛原则：

> Support surface 可以增强判断，但不能扩张主链。

## 2. Truth、Gate 与 Requirement Closure 边界

Beacon 的 requirement closure 必须始终以统一 truth 为中心：

```text
prd
→ user-story
→ test-case
→ acceptance
```

全局规则：

- `prd` 定义问题、目标、边界与承诺面。
- `user-story` 把 `prd` 编译成角色目标与 acceptance criteria。
- `test-case` 把 `user-story` 编译成可验收场景。
- `implement / qa / release` 只能沿这条链闭环，不得反向改写 requirement truth。
- runtime mode、host 能力、内部协作形态只能改变执行方式，不能改变 truth、acceptance 或 gate 标准。
- requirement traceability 必须可检查；如果 `promised -> specified -> tested -> shipped -> verified` 断链，系统必须阻断或显式降级，不得静默滑过。

收敛原则：

> Beacon 的完成定义来自 requirement truth，不来自 runtime 状态、局部完成信号或代理自述。

## 2.1 需求包形状：不兼容旧形状（最高原则 · v1.6.10+）

对 **active 交付面**（`docs/beacon/<version>/` 且 version ≥ 规范生效线，当前为 **v1.6.10+**）的 feature package：

```text
features/<slug>/{truth,tests,tasks,evidence}.md
```

**禁止**以「历史包 / 旧项目 / 软兼容」为由降低门控或保留平行旧形状。

### 必须成立

1. **单一现行形状**：以当前 docs/runtime 规范的 package 形状为准（含人话/L0、Intent Coverage Matrix、旅程、AC、FSM 五元组与 illegal（域湖）、tests 的 Command+Assertion、**exec-layer TC**、tasks 的 `ac=` + `evidence=` 绑定等）。
2. **旧形状 = 不合格，不是可选方言**：缺字段、旧 ledger、无 evidence 绑定、仅 docs-only TC 冒充 exec 等，一律 **fail-closed**。
3. **升级路径只有一条**：  
   `plan（重拆意图）→ truth gen / change → Truth Review Gate → freeze（新 revision）`  
   不得在旧 truth 上打补丁式“兼容读取”冒充合规。
4. **引导话术**：旧项目不满足现行 package 规范时，系统与 skill 必须引导 **重新 plan 并冻结新 truth**，而不是静默沿用旧形状继续 implement/qa/release。
5. **实现后果**：不得为旧形状堆积长期兼容分支；发现仅服务旧形状的代码路径应删除或改为 **显式 upgrade/replan 阻断**（reason_code 指向 plan/truth）。

### 不是什么

- 不是禁止 archive 历史版本只读查阅。
- 不是禁止 `docs/beacon/archive/<version>/` 保留旧交付物。
- 是禁止 **active 面** 与 **现行门控** 对旧形状网开一面。

收敛原则：

> **Truth 形状只向前兼容规范，不向后兼容旧包。旧包要上船，就重 plan、重冻 truth。**

## 3. 内部复杂度边界

Beacon 必须默认把复杂度吸收到机器内部，而不是暴露为人类长期心智负担。

全局规则：

- 宿主能力差异、delegation、precheck、内部 lane 拆解、诊断编排、修复协作等复杂度，默认都应内收。
- `implement` 仍是唯一默认执行入口。
- `qa` 仍是唯一默认验收入口。
- `doctor` 是显式诊断与阻断修复面，不是主流程常驻步骤。

收敛原则：

> 复杂度不能消失，但必须默认留在机器面，而不是转嫁给人类主流程。

## 4. Canonical、事务一致性与 Anti-Drift 边界

Beacon 的多表面写入必须遵循单真值、多投影、事务一致的原则。

全局规则：

- canonical source 是唯一提交源，不允许多源并写。
- human-readable、machine-readable、runtime projection 等表面都必须由 canonical 单向投影生成。
- 不允许 human 文档反向驱动 machine truth。
- 同一批次跨表面写入必须具备可审计事务元数据，并能检查 drift。
- 发现跨面不一致时，不得继续判定 `freeze pass`、`qa pass` 或 `release pass`。
- 版本表面治理同样遵循这条原则：版本号真值必须集中管理，并对外表面做受控投影，而不是散落手改。

收敛原则：

> Beacon 可以有多个读面，但只能有一个提交真值。

### 4.1 MD-only catalog and archive plane (v1.5.2+ evolution line)

- Requirement-truth **version catalog** canonical source is Markdown under `docs/beacon/governance/version-truth-catalog.md`, not scattered JSON or hand-edited directory lists.
- **Active delivery truth** lives under `docs/beacon/<version>/` for at most the configured active window (default: 3 release lines).
- **Archived requirement truth** lives under `docs/beacon/archive/<version>/`; archive is relocation, not deletion of requirement evidence.
- Project **governance** (branch/topology) and **truth catalog** (delivery tree placement) are separate MD authorities; neither substitutes for `features/<slug>/truth.md`.
- `.machine/` and sqlite projections of the catalog are read/verify surfaces; drift against catalog MD must fail closed for freeze, qa, and release on the active plane.
- Default resolver, doctor, status, and truth-map bulk scans use the catalog active set; explicit `--version` may target archived roots for inspect-only commands.

### 4.2 Truth 分支策略（non-monorepo）

非 monorepo 项目（`topology_kind: single_repo`）的 truth 分支职责：

- **需求真相在主分支（main）编写**：`docs/beacon/<version>/` 与 `features/<slug>/{truth,tests,tasks,evidence}.md` 在 `main` 编写 + freeze + push `origin/main`。truth 是共享版本真相，**不进 per-feature worktree**——避免在 feature 分支 fork 后漂移、合并冲突。
- **implement 用 git worktree 隔离**：`beacon-gen-implement` / `beacon-eval-qa` / `beacon-eval-release` 在专用 worktree（`beacon/v<x>/<feature>` 分支）执行，不污染 main。
- **implement 前必 merge main**：开发分支（worktree）实现前，先从 `main` merge 到开发分支，确保实现基于主分支冻结的 truth 事实，而非开发分支的滞后副本。
- **truth 不隔离、实现隔离**：truth 在 main 积累单一真相源；实现隔离避免主分支被半成品污染。两者职责分离。
- monorepo 场景的 truth 分支策略另定（beacon 当前 single_repo）。

### 4.1 自动合入契约（auto-merge · v1.6.14+）

第一性原理：**合入（merge）与发布（release）是两层动作，控制强度必须匹配风险强度**。合入可逆（git revert / 分支保留），发布不可逆（对用户生效）。方法论 M4（Cursor / Lauren Tan）的 auto-merge 落在合入层，与 Beacon release 人类 gate 不冲突。

- **L1 自动合入（默认）**：实现经 worktree 内 QA 验收（AC↔TC 矩阵全绿 + exec-layer 测试 exit 0）后，agent 可自动 `git merge` 回 main。人类不需要对每个 PR 把关合入动作本身。
- **L2 合并前置强化（进行中）**：QA passed 后、merge 前，叠加「失败模式回归库命中 0 + 对抗式审查通过」作为自动合并的证据背书。
- **L3 发布判定（人类 gate 保留，永不动摇）**：`release` verdict 永远不可自动覆盖（`beacon-release` HARD GATE：Human gate required; auto-pass forbidden）。agent 可自动合入 + 构建 + 跑 release 门禁，但**不得自动打 tag / 自动部署 / 自动发布**——最后一步 `manual_confirmation_required` 保持不变。
- **前置条件（三条件缺一即退回人工）**：
  1. 验证前置：合入前已有自动化验证通过证据（QA passed / 测试全绿）。
  2. 可回滚：main 上变更可低成本撤销（git revert 可用、分支保留）。
  3. 可观测：合入后有 evidence / trace / scorecard 可审计；异常可被发现定位。
- **禁止**：无验证证据的自动合入（fake delivery）、自动发布到生产、自动部署。这些仍属人类 gate 保护范围。

## 5. Skill 哲学边界

Beacon 的通用 skill / prompt 默认不得退化成“替强模型编排操作剧本”的说明书。

默认哲学：

- 先校准决策边界与判断哲学
- 再暴露完成当前判断所需的最小完备工具集
- 最后声明容易遗忘但必须成立的事实、边界与约束

可执行标准：

> 一个合格的 Beacon Skill，必须先定义决策边界，再只暴露完成该判断所需的最小工具，最后补充容易遗忘但必须成立的事实；如果主体内容是在替强模型编排逐步操作剧本，则默认判为不合格。

补充规则：

- `ceo-review`、`brainstorm`、`design-review`、`deep` 这类 guidance/support surface 必须优先贯彻这套哲学。
- support surface 的主要职责是校准 framing、challenge scope、暴露关键工具、解释 route 与证据缺口，而不是把探索和判断过程硬编码成脚本。

**第一性原理（v1.6.4+）**

- 动手前回到根本问题：每个实现/验收决定必须能说明「为什么」，并拆到最小可验证单元。
- truth / implement / qa 入口须校准根本问题与最小可验证单元，不得把第一性原理降格为文档装饰。

**对抗式审查（v1.6.4+ QA release 前强制）**

- 交付前切换为最挑剔审查者：主动列出 3–5 个翻车点，并为每个翻车点提供验证证据。
- 不接受「看起来没问题」式放行；缺失翻车点清单或验证证据 → fail-closed block（reason_code `adversarial_review_missing`）。
- qa skill 与 planner adversarial lane 在 qa release 前强制触发，非可选信号触发。

## 6. 强 Gate / Fail-Closed 例外边界

Beacon 允许在强 gate 场景中使用比默认 skill 哲学更强的步骤约束，但必须有明确理由。

适用场景：

- `qa`
- `release`

### 6.1 Protected Path Denylist（v1.6.5+ cross-version）

Beacon 自动写入面必须把高风险路径视为 fail-closed 保护区；planner/research 等只读支持面可以报告风险，但不能把 advisory 当作允许自动修改。

受保护路径类别包括：

- environment: `.env`, `.env.*`
- secrets: `secrets/**`, `credentials/**`, `*.pem`, `*.key`, `*_key*`
- auth: `auth/**`, `oauth/**`, `sso/**`
- payments: `payments/**`, `billing/**`, `stripe/**`
- migrations: `migrations/**`, `schema_migrations/**`

写入 guard 命中这些路径时必须返回 `reason_code=protected_path_denylist`，并给出显式 human route recommendation；不得静默允许自动编辑。
- freeze / refreeze / canonical truth 提交
- 安全、权限、合规检查
- 多 sink 事务一致性
- 任何 fail-closed 的发布、校验、回滚、阻断面

全局规则：

- 这些场景可以显式写入顺序、事务、验证、回滚要求。
- 这样做的原因不是模型能力不足，而是 judgment 必须可审计、事务必须一致、错误代价必须硬性收口。

收敛原则：

> 只有当步骤约束是在保护强 gate、事务一致性或 fail-closed 边界时，Beacon 才应显式强化 how-to；否则默认优先写“怎么判断”，而不是“每一步怎么做”。

## 7. Learning 与记忆边界

Beacon 必须依赖可回放、可审计、可裁剪的记忆，而不是隐式上下文依赖。

全局规则：

- learning 可以影响判断、排序、route、风险 framing，但不能直接定义 truth 或 gate verdict。
- learning 必须可见：系统应能说明哪些 learning 影响了本轮判断。
- learning 必须可审计：系统应能说明这些 learning 是否仍然可信、是否冲突、是否需要剪枝。
- 多轮 evo 的连续性应来自 governed recall，而不是让模型被动“记住上文”。

可执行标准：

> Beacon 的 Skill 不应强依赖步骤脚本，但必须强依赖可回放记忆、显式边界与可累积演化的判断框架，确保同一问题在多轮 evo 中能持续收敛，而不是每轮重新发明判断方式。

收敛原则：

> 少规定步骤，多固定边界；少依赖即时上下文，多依赖可回放记忆；learning 只辅助判断，不定义真相与 gate。

补充规则：

- 在 learning / recall 尚未充分覆盖前，Beacon Skill 应保留少量高价值示例作为冷启动判断支架。
- 这些示例用于稳定边界判断、典型路由和反例辨识，不用于展开逐步操作剧本。

## 8. 默认行为边界

Beacon 默认行为边界必须保留以下四类轻量原则：

1. 显式假设与歧义管理
   - 不清楚时不得静默选择一种解释直接执行
   - 多种解释并存时必须显式记录假设、取舍或阻断原因
   - 关键需求不明时，应回到 `prd / user-story` 收口，而不是直接进入实现
2. 简单优先
   - 默认采用最小正确改动
   - 不为单次需求新增 speculative abstraction、额外配置或未请求的未来扩展
3. 外科手术式改动
   - 只改与当前请求、冻结材料或当前闭环直接相关的范围
   - 不做 drive-by refactor
   - 不删除无关代码、注释或格式
   - 只清理自己这次改动制造的 orphan
4. 目标驱动验证闭环
   - 非平凡任务必须绑定成功标准与验证信号
   - bug fix 要有复现或验证证据
   - feature 要有 acceptance/test 映射
   - refactor 要有 before/after 验证

收敛原则：

> 轻量不等于随意；简单任务可以少流程，但不能破坏边界、扩大范围或跳过验证闭环。

## 9. 人类面与机器面边界

Beacon 默认必须同时满足两条：

- 对人类，输出应保持极简、定向、可行动。
- 对机器与审计面，输出必须完整、可追溯、可检查。

全局规则：

- 人类面优先回答当前状态、阻断原因、下一步与回路方向。
- 机器面必须保留 reason codes、evidence refs、diagnostics、artifact paths、transaction metadata 等完整信息。

收敛原则：

> 复杂度不能消失，但必须被机器完整吸收，再以低摩擦方式投影给人类。

## 10. 外部能力与宿主边界

外部能力、外部规范、外部样本、外部运行时都只能作为研究或受治理输入，不能成为 Beacon 默认运行前提。

全局规则：

- 同一轮判断只基于当前宿主能力面，不把多宿主桥接选择暴露为默认人类心智负担。
- 外部能力可以被借鉴、治理、吸收、组合，但不应自动升级为 Beacon 的默认主叙事或强依赖。
- 目标项目接入 Beacon 后，默认也应继承这份边界，而不是私自扩张出更宽的默认主叙事。

收敛原则：

> Beacon 可以吸收外部能力，但不能把外部依赖升级成默认前提。

## 禁止事项

以下做法应被视为违背 Beacon 全局约束：

1. 因内部能力增强而持续扩张公开主流程或把 support surface 升格成新 stage
2. 让 runtime mode、host 差异或内部协作形态改写 requirement truth、acceptance 或 gate 标准
3. 在没有 canonical 提交与事务一致性的前提下进行多点写入
4. 允许 human、machine、runtime projection 出现同批次语义漂移却继续 freeze、qa 或 release
5. 把通用 skill 写成强模型的默认操作剧本，而不是边界、工具、事实说明
6. 让 learning 静默决定 truth 或 release verdict，而不是作为可审计 advisory input
7. 把内部复杂度、外部依赖或多宿主选择默认倾倒给人类主流程


## Dual-axis version (Runtime vs Project docs)

> **Permanent principle — do not collapse these axes.**

**Runtime is the screwdriver model; project docs version is the workpiece serial number.**

| Axis | Meaning | Typical source | Must not |
|------|---------|----------------|----------|
| **Beacon Runtime** | Installed toolkit / CLI capability line | `beacon --version`, `version-truth.runtime_version` | Become the default product version of a customer repo |
| **Project docs / delivery line** | Product version under `docs/beacon/<ver>/` | project onboarding, explicit `-v`, project release baseline | Be inferred from toolkit semver (e.g. v1.6.8 → v1.6.9) for foreign projects |

### Defaults when the user omits `-v`

| Project kind | Project docs default | Hotfix baseline |
|--------------|----------------------|-------------------|
| External **greenfield** | **`v0.0.1`** | n/a or first ship (prefer normal, not hotfix) |
| External **existing** | patch+1 of **project** baseline | pin **project** production line |
| **Beacon/Loom self** monorepo | monorepo planning / self patch rules only | `current_runtime_release_line` |

### Illegal (reason conceptual)

- Treating Beacon monorepo `docs/beacon/v1.6.9` as the default for an empty customer project
- Using toolkit runtime patch as the product version of an external greenfield project
- Silent major/minor bump without explicit user intent

### External project hard rule (enforced)

- `init` / `setup-context` / material-writing commands refuse `docs_version == runtime_version` on external (`external_greenfield` / `external_existing`) projects.
- Omitting `-v` must default external docs to `v0.0.1` (greenfield) or the project delivery baseline (existing), never the toolkit runtime semver.
- Doctor diagnostics/verify fail closed when an external project collapses the two axes or carries a runtime-named docs root.
- The only explicit force is the documented override `BEACON_ALLOW_RUNTIME_AS_DOCS_VERSION=1`; there is no silent fallback.

Runtime module: `beacon.utils.version_defaults` (`classify_project`, `default_target_version`, `default_hotfix_baseline`).
