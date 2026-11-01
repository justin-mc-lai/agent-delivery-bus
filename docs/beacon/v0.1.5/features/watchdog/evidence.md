---
slug: watchdog
version: v0.1.5
materials_status: draft
---

# Evidence Index: watchdog

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/watchdog/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/watchdog/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/watchdog/tasks.md | present | truth |
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json | present | truth |

## 证据要求（冻结后由 implement 与 qa 填）

- AC-001..007：`go test -run ...` 的输出
- AC-006：`adbd agent plist` 的实际输出
- AC-007：`adbd agent status` 在未安装状态下的实际输出
