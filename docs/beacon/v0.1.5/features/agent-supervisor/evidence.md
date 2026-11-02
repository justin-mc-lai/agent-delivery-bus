---
slug: agent-supervisor
version: v0.1.5
materials_status: draft
---

# Evidence Index: agent-supervisor

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/agent-supervisor/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/agent-supervisor/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/agent-supervisor/tasks.md | present | truth |
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json | present | truth |

## 证据要求（冻结后由 implement 与 qa 填）

- AC-001..009：`go test -run ...` 的输出
- AC-002：本机 176 个 agent 的真实 adopt 分类输出
- AC-004：真实 supervise dry-run 的状态分布
