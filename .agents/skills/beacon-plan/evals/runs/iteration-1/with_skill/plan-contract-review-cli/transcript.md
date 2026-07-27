# Eval plan-contract-review-cli (with_skill)

## Prompt

feature=demo version=v0.0.1，需要可审计的多角度 review。说明如何跑 runtime evidence，execution_mode 与 artifact 路径。不要假装 parallel subagent。

## Output

我能访问的已加载 SKILL.md 内容包含了 `pln-review` 模式的完整内容以及 `reviewer-catalog`。但由于文件系统权限限制，我无法直接读取嵌套引用文件或项目代码。下面基于已加载的模式规范，输出符合合约的 planner-review 产物。

---

## planner-review: feature=demo, version=v0.0.1

### 1. Runtime Evidence：如何跑

根据 `pln-review` 模式的 Review mode audit path，runtime evidence 的标准命令是：

```bash
beacon planner-review run demo \
  --project-root . \
  --version v0.0.1 \
  --prompt "feature=demo version=v0.0.1，需要可审计的多角度 review" \
  --json
```

这条命令会：
1. 启动 deterministic runtime builder
2. 按 reviewer-catalog 加载 always-on + conditional reviewers
3. 各 reviewer 产出独立 findings
4. 合成一份结构化 artifact 写入 `.beacon/state/planner-review/demo/<review_id>.json`

---

### 2. execution_mode 与 fallback_reason

根据 `multi-agent-runtime.md`（已嵌入 SKILL.md 的引用说明）的要求：

> 无 subagent runtime 时必须为 `single_process_multi_reviewer` + `fallback_reason`；禁止声称 parallel subagent execution。

当前环境：

| 项目 | 状态 |
|------|------|
| `beacon` CLI 是否安装 | **未验证**（无法执行 Bash） |
| subagent runtime（parallel）是否可用 | **不可用**（当前为单进程 Claude Code session，无 `beacon planner-review run` 的 subagent 分发能力） |
| 文件系统权限 | **受限**（无法读取项目文件和嵌套引用） |

**因此：**

```
execution_mode: single_process_multi_reviewer
fallback_reason: >
  No subagent runtime available. The beacon CLI (`beacon planner-review run`) 
  is not verified as installed in this session. File-system reads for nested 
  references (multi-agent-runtime.md, finding-schema.md, intent-parity-artifacts.md, 
  state-machine-review.md) are blocked by permission settings. All reviewer 
  analyses are performed sequentially within a single Claude Code process. 
  No parallel subagent execution is claimed.
```

---

### 3. Artifact 路径

```
.beacon/state/planner-review/demo/<review_id>.json
```

其中 `<review_id>` 由 `beacon planner-review run` 自动生成（通常为时间戳或 UUID）。如果手动构造，建议格式：`pln-review-demo-v0.0.1-<ISO-timestamp>.json`。

---

### 4. Always-On Reviewer 分析（单进程内执行）

根据 reviewer-catalog，以下 always-on reviewers 每个请求都必须运行：

#### 4.1 `intent-fidelity` — 意图保真

| 维度 | 内容 |
|------|------|
| **用户原话** | "feature=demo version=v0.0.1，需要可审计的多角度 review" |
| **核心诉求** | ① 对 demo 特性做多角度审查；② 审查必须是可审计的（有 runtime evidence、有 artifact 路径、有 reviewer 输出）；③ 不允许假装 parallel subagent |
| **finding** | 用户明确要求"可审计"（auditable），这意味着每个 reviewer 的发现必须可追溯到具体证据、具体 reviewer、具体 severity。当前由于无法读取项目文件，`evidence_refs` 只能引用模式文件本身。 |
| **severity** | `medium` |
| **confidence** | `high` |

#### 4.2 `scope-mode` — 范围模式

| 维度 | 内容 |
|------|------|
| **判定** | `research` — 用户首要目标是理解如何运行 review 流程（runtime evidence 命令、execution_mode、artifact 路径），而非对 demo 特性本身做完整的 parity review。 |
| **finding** | scope_mode 与用户意图一致：这是一个关于"如何审查"的元请求，包含对 demo 特性的初步 always-on 审查。 |
| **severity** | `info` |
| **confidence** | `high` |

#### 4.3 `coverage-shape` — 覆盖形态

| 维度 | 内容 |
|------|------|
| **判定** | 当前审查的证据来源仅限于 SKILL.md 中嵌入的模式规范。无法访问：① 项目源代码；② `.beacon/` 目录下的 truth package；③ 嵌套引用文件（finding-schema.md 等）。 |
| **finding** | 覆盖形态受限于文件系统权限。实际 behavior evidence 需要 `beacon planner-review run` 产出 runtime artifact 后才能获得。 |
| **severity** | `high` |
| **confidence** | `high` |

#### 4.4 `route-boundary` — 路由边界

| 维度 | 内容 |
|------|------|
| **判定** | 本审查保持在 planner 边界内。未写入 truth、代码、QA verdict、release verdict、`.machine/`。 |
| **finding** | 无违规。 |
| **severity** | `info` |
| **confidence** | `high` |

#### 4.5 `state-machine` — 状态机审查

| 维度 | 内容 |
|------|------|
| **触发原因** | feature=demo 无具体 lifecycle/workflow 描述，无法判定是否涉及状态机风险。 |
| **finding** | 无足够信息触发深度 state-machine review。若 demo 特性涉及多步骤流程、状态转换、或异步操作，需要在可读取项目文件后重新评估。 |
| **severity** | `info`（暂不适用） |
| **confidence** | `low` |

---

### 5. Conditional Reviewer 分析

| Reviewer | 是否触发 | 原因 |
|----------|---------|------|
| `source-parity` | **否** | 无 source project / OSS / parity 诉求 |
| `deferral-sovereignty` | **否** | 无 MVP cut / defer 信号 |
| `security-boundary` | **否** | 无 auth / permission / secrets 关键词 |
| `performance-boundary` | **否** | 无 long task / concurrency 关键词 |
| `design-flow` | **否** | 无 UI/UX 关键词 |
| `brownfield-contract` | **否** | 无 takeover / legacy 关键词 |
| `evidence-proof-shape` | **是** | 用户明确要求"可审计"的 evidence |
| `simplification-risk` | **否** | 无简化诉求 |
| `founder-business` | **否** | 无 business outcome 诉求 |
| `adversarial-doubt` | **否** | 无 challenge assumption 诉求 |
| `adr-documentation` | **否** | 无 ADR 诉求 |
| `context-engineering` | **是** | 文件系统权限受限，context 不完整 |

---

### 6. 综合 Findings

| # | Reviewer | Severity | Finding |
|---|----------|----------|---------|
| F1 | `coverage-shape` | **HIGH** | 无法读取项目文件（`.beacon/`、源代码、嵌套引用），审查覆盖不完整。需在 `beacon planner-review run` 可执行或文件系统权限开放后重新审查。 |
| F2 | `evidence-proof-shape` | **HIGH** | 用户要求"可审计"，但当前无 runtime artifact（`.beacon/state/planner-review/demo/<review_id>.json`）。审计链断裂——无法证明 reviewers 已按 catalog 完整执行。 |
| F3 | `context-engineering` | **MEDIUM** | Context 不完整：missing `finding-schema.md`、`multi-agent-runtime.md`、`intent-parity-artifacts.md`、`state-machine-review.md` 的完整内容。可能导致 reviewer 输出偏离 schema 规范。 |
| F4 | `intent-fidelity` | **MEDIUM** | 用户明确要求"不要假装 parallel subagent"。已遵守：`execution_mode=single_process_multi_reviewer` + 明确 `fallback_reason`。 |
| F5 | `state-machine` | **INFO** | 无足够信息判断 demo 特性是否涉及状态机风险。若涉及，需加载 `state-machine-review.md` 并产出 State Model / Diagram Truth Layer。 |

---

### 7. 输出合约

```yaml
mode_id: pln-review
public_id: plan
evidence_refs:
  - "skills/beacon-plan/SKILL.md (embedded pln-review mode + reviewer-catalog)"
  - "无法访问: references/pln-review/multi-agent-runtime.md"
  - "无法访问: references/pln-review/finding-schema.md"
  - "无法访问: references/pln-review/intent-parity-artifacts.md"
  - "无法访问: references/pln-review/state-machine-review.md"
  - "无法访问: 项目 .beacon/ 目录、源代码"
scope_mode: research
execution_mode: single_process_multi_reviewer
fallback_reason: >
  No subagent runtime available. beacon CLI not verified. 
  File-system reads blocked. Single-process sequential reviewer execution only.
  No parallel subagent execution claimed.
findings:
  - id: F1
    severity: high
    reviewer: coverage-shape
    summary: 无法读取项目文件，审查覆盖不完整
  - id: F2
    severity: high
    reviewer: evidence-proof-shape
    summary: 无可审计的 runtime artifact
  - id: F3
    severity: medium
    reviewer: context-engineering
    summary: 嵌套引用文件未加载，可能偏离 schema
  - id: F4
    severity: medium
    reviewer: intent-fidelity
    summary: 已遵守"禁止假装 parallel subagent"
  - id: F5
    severity: info
    reviewer: state-machine
    summary: 无足够信息判断状态机风险
recommended_next_harness: plan
```

---

### 8. 下一步操作指南

要获得真正可审计的 review，需要按以下顺序解除阻塞：

1. **确认 beacon CLI 安装**：
   ```bash
   which beacon && beacon --version
   ```

2. **确认项目根目录存在且 `.beacon/` 已初始化**：
   ```bash
   ls -la .beacon/
   ```

3. **运行 runtime evidence**：
   ```bash
   beacon planner-review run demo \
     --project-root . \
     --version v0.0.1 \
     --prompt "feature=demo version=v0.0.1，需要可审计的多角度 review" \
     --json
   ```

4. **读取生成的 artifact**：
   ```bash
   cat .beacon/state/planner-review/demo/<review_id>.json
   ```

5. **如果 beacon CLI 不可用**：需要在当前 session 中手动加载 `references/pln-review/` 下的全部嵌套引用文件，然后逐 reviewer 执行分析。此时的 `execution_mode` 始终为 `single_process_multi_reviewer`。

---

**推荐下一跳**: `plan`（继续在当前 harness 内，待权限/CLI 就绪后重新执行完整 review；或路由到 `goal` 进入长程流水线 `plan→truth→design?→implement→qa→release`）


## Grade

```json
{
  "passed": 3,
  "total": 3,
  "score": 1.0,
  "results": [
    {
      "expectation": "mentions_planner_review_cli",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "mentions_artifact_path",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "no_fake_parallel_subagent",
      "pass": true,
      "reason": "matched"
    }
  ]
}
```
