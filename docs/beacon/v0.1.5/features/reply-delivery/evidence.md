---
slug: reply-delivery
version: v0.1.5
materials_status: draft
---

# Evidence Index: reply-delivery

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/reply-delivery/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/reply-delivery/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/reply-delivery/tasks.md | present | truth |
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json | present | truth |

## 证据要求（冻结后由 implement 与 qa 填）

- AC-001..006：`go test -run ...` 的输出
- AC-002：一条真实 trace 行（含 delivered / delivery_reason）
