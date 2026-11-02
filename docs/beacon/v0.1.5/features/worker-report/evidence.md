---
slug: worker-report
version: v0.1.5
materials_status: draft
---

# Evidence Index: worker-report

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/worker-report/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/worker-report/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/worker-report/tasks.md | present | truth |
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json | present | truth |

## 证据要求（冻结后由 implement 与 qa 填）

- AC-001..007：`go test -run ...` 的输出
- AC-003：一次真实 reconcile 后 `adbd-exec-state.json` 的内容
- AC-007：watchdog 对 done 绑定输出 NOTHING 的实测
