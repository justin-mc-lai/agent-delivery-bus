<!-- BEACON:START -->
<!-- BEACON:VERSION:v1.6.12 -->
<!-- BEACON:DOCS_VERSION:v0.1.4 -->
# AGENTS.md

This file is Beacon-managed agent operation guidance.

## Agent behavior
- Keep scope tight and implement the smallest correct change.
- Verify diagnostics/tests before claiming completion.
- Preserve user-authored sections outside Beacon managed blocks.

## Beacon requirement-material usage
- Runtime target version: `v1.6.12`
- Docs target version: `v0.1.4`
- Use `docs/beacon/global-boundaries.md` as the canonical Beacon-wide global constraint source.
- Use `docs/beacon/<version>/` as the canonical delivery tree.
- Read progressively in this order:
  1. `docs/beacon/global-boundaries.md`
  2. `docs/beacon/<version>/SUMMARY.md`
  3. `docs/beacon/<version>/execution/index.md`
  4. `docs/beacon/<version>/execution/architecture-blueprint.md`
  5. `docs/beacon/<version>/prd/`, `user-story/`, `qa/test-cases/`
  6. `docs/beacon/<version>/.machine/`
- For greenfield projects: start with architecture blueprint, then enter think → user-story → prd.
- For takeover projects: document current-state architecture, service map, constraints, and version timeline before new implementation.
- If context blocks are missing/corrupted, run:
  - `beacon doctor setup-context --project-root .`
  - `beacon doctor verify-context --project-root . --strict`
- After runtime/version upgrades on a real project, sync machine requirement materials before QA:
  - `beacon doctor sync-materials --project-root . --version <version> --all-features`
<!-- BEACON:END -->


<!-- USER:CUSTOM-START -->
## 用户规则（大白话汇报）

- 每次交付的「这次做了什么」必须让不懂本项目的人看懂：说人话、少术语、必要术语一句话解释。
- 复杂批次按 `/eli5`（本仓 `skills/eli5` + 全局已装，Apache-2.0）的大图少字思路输出。
- 参考实现：`selfmedia/prism` 仓 `contracts/human-card.md` 大白话规则 + `vendor/skills/eli5`。
<!-- USER:CUSTOM-END -->
