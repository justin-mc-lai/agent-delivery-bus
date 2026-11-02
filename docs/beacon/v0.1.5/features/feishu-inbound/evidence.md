---
slug: feishu-inbound
version: v0.1.5
materials_status: draft
---

# Evidence Index: feishu-inbound

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/feishu-inbound/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/feishu-inbound/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/feishu-inbound/tasks.md | present | truth |
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json | present | truth |

## 证据要求（冻结后由 implement 与 qa 填）

- AC-001..005：`go test -run ...` 的输出
- AC-001：用 fixture NDJSON 通过 stdin 喂入后的计数输出
