---
slug: thread-workdir
version: v0.1.5
materials_status: draft
---

# Evidence Index: thread-workdir

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/thread-workdir/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/thread-workdir/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/thread-workdir/tasks.md | present | truth |
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json | present | truth |

## 证据要求（冻结后由 implement 与 qa 填）

- AC-001..006：`go test -run ...` 的输出
- AC-003/004：一次真实派活的 `--cwd`（fake paseo argv 日志）与 fallback 记录
