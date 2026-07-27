# Eval plan-mode-selection-source (baseline)

## Prompt

对着仓库源码做需求调研，别瞎编。请给出 mode_id 与 recommended_next_harness。

## Output

我已经完成了对 Beacon 仓库源码的系统调研。以下是基于源码锚定的输出：

---

## 输出契约

### `mode_id`: `pln-source`

### `public_id`: `plan`

### 分类依据

| 信号 | 来源 | 匹配 |
|------|------|------|
| "对着仓库源码" | 用户 utterance | → `source` subintent |
| "别瞎编" | 用户 utterance | → source-anchored discipline |
| "需求调研" | 用户 utterance | → research (非 review/brainstorm/program/friction) |

按 `skills/beacon/beacon-plan/SKILL.md` L63-71 的 subintent→mode 映射表：

```
source → pln-source → references/modes/pln-source.md
```

### `evidence_refs`

| 证据 | 路径 |
|------|------|
| Plan SKILL 定义 | `skills/beacon/beacon-plan/SKILL.md` |
| Modes 索引 | `skills/beacon/beacon-plan/references/modes/INDEX.md` |
| pln-source 模式文件 | `skills/beacon/beacon-plan/references/modes/pln-source.md` |
| 1+6 公共面定义 | `skills/beacon/references/public-surface-1plus6.md` |
| CLI plan start 入口 | `beacon/commands/plan.py` |
| 运行时分类逻辑 | `beacon/utils/program_plan_runtime.py` (L28-45 `classify_plan_mode`) |
| 渐进式映射 | `skills/beacon/references/public-surface-progressive-map.v1.json` |

---

## 源码调研结论

### 1. `mode_id` 体系（7 个模式，全部锚定源码）

从 `beacon-plan/SKILL.md` L42-51 和 `references/modes/INDEX.md` 确认，plan harness 下有 **7 个 mode**：

| mode_id | 模式文件 | 用途 |
|---------|---------|------|
| `pln-review` | `pln-review.md` | 多角度审查 |
| `pln-brainstorm` | `pln-brainstorm.md` | 冻结前头脑风暴 |
| **`pln-source`** | `pln-source.md` | **源码锚定调研** ← 当前 |
| `pln-program` | `pln-program.md` | 海→湖程序规划 |
| `pln-program-auto` | `pln-program-auto.md` | 自动程序模式 |
| `pln-program-interactive` | `pln-program-interactive.md` | 交互程序模式 |
| `pln-friction` | `pln-friction.md` | 摩擦录入 |

CLI 层通过 `beacon/utils/program_plan_runtime.py:classify_plan_mode()` 做自动分类（L28-45）：
- `INTERACTIVE_SIGNALS` → `interactive`
- `FULL_PARITY_SIGNALS` / source URL → `auto`
- 其他 → `auto-detect`（低置信度，需追问）

### 2. `recommended_next_harness` 体系（1+6 公共面）

从 `public-surface-1plus6.md` 和 `SKILL.md` L95 确认，`recommended_next_harness` 合法值为：

```
goal | plan | truth | design | implement | qa | release | stop
```

默认流水线顺序（`public-surface-1plus6.md` L14-16）：

```
plan → truth(freeze) → design? → implement → qa → release
```

其中 `goal` 是横切面（long-run facade），不计入 6 个 harness ability；`design` 仅在 `ux_required` / visual delivery 时触发。

### 3. `pln-source` 模式的边界（来自 pln-source.md）

- **不是** truth 源、gate 源、新生命周期阶段
- **产出**：source anchor summary + unsupported-claim warnings + route recommendation
- **不产出**：frozen truth、implementation、QA/release verdict
- 若 source 与 frozen truth 冲突 → 显式 route back to `change`

---

## `recommended_next_harness`: `stop`

**理由**：当前请求为对 Beacon plan harness 自身的源码调研，未指定具体 feature/version/实现目标。调研结论已产出。若用户后续需要：

- 对具体 feature 做程序规划 → 路由到 `plan`（mode=`pln-program`）
- 冻结需求 truth → 路由到 `truth`
- 进入实现 → 路由到 `implement`

所有路由需用户确认后执行（harness boundary 规则，`pln-source.md` L93-98 HARD GATE）。


## Grade

```json
{
  "passed": 3,
  "total": 3,
  "score": 1.0,
  "results": [
    {
      "expectation": "mode_pln_source_or_review",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "has_recommended_next",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "no_implement",
      "pass": true,
      "reason": "matched"
    }
  ]
}
```
