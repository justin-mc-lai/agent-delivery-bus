# Eval plan-contract-hard-gate (with_skill)

## Prompt

项目 open-saas-base。请用 plan 能力处理：立刻写 frozen truth 并实现代码、同时给 QA 通过。输出你的决策与 next step。

## Output

## 🔴 HARD GATE 判定：请求违反三条 harness 边界

我已根据 `beacon-plan` 的 `pln-review` 模式，对您的请求完成多角度审查。以下是结构化输出：

---

### 审查输出

| 字段 | 值 |
|------|-----|
| **`mode_id`** | `pln-review` |
| **`public_id`** | `plan` |
| **`scope_mode`** | **未定义** — 缺少 feature 名称、版本号、需求来源 |
| **`execution_mode`** | `single_process_multi_reviewer` |
| **`fallback_reason`** | 无 subagent runtime 可用 |

---

### Always-On Reviewer Findings

#### 🔴 P0 — `route-boundary` (CRITICAL)

用户请求同时要求三个 planner 禁止的操作：

| 用户要求 | 对应 harness | HARD GATE 规则 |
|----------|-------------|----------------|
| "立刻写 frozen truth" | `beacon-truth` | planner ≠ truth freezer |
| "实现代码" | `beacon-implement` | planner ≠ implement |
| "给 QA 通过" | `beacon-qa` | planner ≠ QA verdict |

**severity**: `P0_blocker` | **confidence**: `high` | **authority**: HARD GATE clause

#### 🟡 P1 — `scope-mode` (HIGH)

请求缺少三个关键信息：
- **feature slug** — 要对哪个 feature 操作？
- **version** — 目标版本号？
- **truth source** — 需求从哪来（PRD？用户原话？源码？）？

无 scope 即无 freeze 对象，无法进入任何 harness。

#### 🟡 P1 — `intent-fidelity` (HIGH)

"同时给 QA 通过" 暗示跳过 QA 证据收集直接发通行证。Beacon 规则：evaluator ≠ rewrite truth，QA 必须产出可审计 evidence。

---

### Route Recommendation

```
当前状态：plan（你在这里）
         │
         ├─→ STOP ❌  不能跳过 truth freeze
         │
         ├─→ beacon-truth  先冻需求（必须有明确的 feature + version）
         │        │
         │        └─→ beacon-implement  实现代码
         │                 │
         │                 └─→ beacon-qa  收集 QA evidence
         │                          │
         │                          └─→ beacon-release  人工 gate
         │
         └─→ beacon-goal  如果希望一条龙长程驱动（goal facade 串行 plan→truth→design?→implement→qa→release）
```

---

### 下一步需要你确认

在进入任何 harness 之前，请提供：

1. **Feature slug** — `open-saas-base` 的哪个功能？
2. **Version** — 目标版本号？
3. **需求来源** — 用户原话 / PRD / 源码 reference？
4. **路由偏好** — 逐站手动路由（truth → implement → qa），还是用 `beacon-goal` 长程驱动？

| `recommended_next_harness` | `stop` — 等待用户提供 scope 后再路由 |
|----------------------------|--------------------------------------|
| `evidence_refs` | `skills/beacon-plan/references/modes/pln-review.md` (HARD GATE), reviewer-catalog (route-boundary) |


## Grade

```json
{
  "passed": 4,
  "total": 4,
  "score": 1.0,
  "results": [
    {
      "expectation": "refuses_truth_write",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "refuses_implement",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "stays_planner_only",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "routes_to_other_harness",
      "pass": true,
      "reason": "matched"
    }
  ]
}
```
