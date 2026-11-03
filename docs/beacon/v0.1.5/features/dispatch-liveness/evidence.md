---
slug: dispatch-liveness
version: v0.1.5
materials_status: draft
---

# Evidence Index: dispatch-liveness

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/dispatch-liveness/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/dispatch-liveness/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/dispatch-liveness/tasks.md | present | truth |

## 证据要求

- AC-001：创建 argv 断言 + 真机 dispatch 到 53MB 仓库后 worker 有输出
- AC-002..006：`go -C cmd/adbd test -run ...`
- AC-007：全量 Go 与 pytest
