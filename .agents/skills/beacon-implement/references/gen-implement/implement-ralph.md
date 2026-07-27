# Implement Ralph Mode

## Purpose

Use `ralph` as an **implement-only closure controller** when team or complex implementation evidence needs multi-round repair routing toward review, QA, or release **checks**.

Ralph is a **sub-controller under `beacon-gen-implement`**, not a product goal.

## Admission

- Requirement truth is frozen.
- Team or complex implementation has produced evidence, but implement closure is not finished.
- Review quality, QA, or release *next action* must be routed.
- Ralph can record blocker, next action, round trace, and evidence refs.

## Command

```bash
beacon implement run "<feature>" --project . --version <version> --mode ralph
```

Product Goal may delegate implement rounds:

```bash
beacon goal delegate-ralph --run-id <id> --feature <slug> --project .
```

## Evidence

- Ralph round trace.
- Current blocker and next action.
- Recommended command for implement repair, QA, or release **routing**.

## Boundary（硬边界）

- **范围**：仅 `implement --mode ralph`
- **不得写 truth** / 不得改冻结需求
- **不得发 QA verdict / release verdict**；不得设置 `qa_verdict`、`release_verdict`、`product_complete` 权威字段
- **Ralph finalize ≠ product complete**；产品完成权威在 Product Goal `done_when` + evaluator
- Handoff 仅允许：implement / qa / review_quality / verify / test-case / human_review / repair / release(route only)
- Never replaces QA, release, gate, or frozen truth

## Anti-patterns

- 用 Ralph 多轮跑完后口头宣布「产品交付完成」
- 在 Ralph state 里写入 release_ready 权威
- 把 `$beacon-gen-implement --mode ralph` 当作 `$beacon-goal` 的替代入口
