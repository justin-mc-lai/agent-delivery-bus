---
slug: gh-bug-binding
version: v0.1.5
materials_status: draft
---

# Evidence Index: gh-bug-binding

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/gh-bug-binding/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/gh-bug-binding/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/gh-bug-binding/tasks.md | present | truth |

## 证据要求

- AC-001..007：`go -C cmd/adbd test -run ...`
- AC-003：真机一条未打标签的 gh 线程被拒并留在队列
- AC-007：真机 change→refreeze 后同一条线程可通过
