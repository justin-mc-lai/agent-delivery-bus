---
slug: router-loop
version: v0.1.5
materials_status: draft
---

# Evidence Index: router-loop

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/router-loop/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/router-loop/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/router-loop/tasks.md | present | truth |
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json | present | truth |

## 证据要求（冻结后由 implement 与 qa 填）

- AC-001..007：`go test -run ...` 的输出
- AC-003：`adbd route --list-tools` 输出 `[]`，以及 route 实现不引用 `os/exec` 的源码扫描结果
