---
slug: install
version: v0.1.5
materials_status: draft
---

# Evidence Index: install

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/install/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/install/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/install/tasks.md | present | truth |
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json | present | truth |

## 证据要求（冻结后由 implement 与 qa 填）

- AC-001..007：`go test -run ...` 的输出
- AC-002：一次真实 `adbd install --apply` 后的 `adbd install --status`
