---
slug: github-issue
version: v0.1.5
materials_status: draft
---

# Evidence Index: github-issue

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/github-issue/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/github-issue/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/github-issue/tasks.md | present | truth |
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json | present | truth |

## 证据要求（冻结后由 implement 与 qa 填）

- AC-001..006：`go test -run ...` 的输出
- AC-006：一次真实轮询的 stdout（polled/new/deduped）与 decisions.jsonl 对应行
