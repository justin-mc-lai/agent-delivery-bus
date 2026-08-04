---
slug: vision-flywheel
version: v0.0.4
materials_status: current
---

# Evidence: vision-flywheel

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.0.4/features/vision-flywheel/truth.md | draft→freeze | truth |
| tests | test_truth | docs/beacon/v0.0.4/features/vision-flywheel/tests.md | current | qa |
| implement | support_advisory | .beacon/evidence/implement/vision-flywheel/ | planned | implement |
| qa | qa_verdict | .beacon/evidence/qa-feature/vision-flywheel/ | planned | qa |

## Source Refs

- `docs/vision-first-principles.md`（愿景，2026-08-03 修订，源码级现状核查）
- `docs/beacon/research/beacon-longrun-stability-analysis-2026-08.md`（S4/S7 缺口）
- `docs/beacon/research/beacon-longrun-pi-agent-evolution-2026-08.md`（loopx quota/should_run 借鉴）
- `docs/beacon/v0.0.3/features/ops-digest-cron/`（参照包：cron_owner=hermes、不内嵌调度器）
- `docs/beacon/v0.0.4/research/vision-flywheel.md`（本包 research）

## Implement Evidence

- `.beacon/evidence/implement/vision-flywheel/AC-FLY-*.json`（每个 AC 一条，implement 时产生）
- `.beacon/evidence/implement/vision-flywheel/GATES.json`（包门）
- `.beacon/evidence/implement/vision-flywheel/QA-MATRIX.json`（QA 矩阵）

## Evidence Invariant

- 每个 evidence 文件含：ac_id / command / assertion_result / timestamp / runner。
- `[x]` 仅当证据文件存在（implement 准入硬拦）。
