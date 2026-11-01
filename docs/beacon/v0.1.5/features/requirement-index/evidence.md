---
slug: requirement-index
version: v0.1.5
materials_status: draft
---

# Evidence Index: requirement-index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/requirement-index/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/requirement-index/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/requirement-index/tasks.md | present | truth |
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json | present | truth |
| review | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/program-review-p2.md | present | truth |

## 证据要求（冻结后由 implement 与 qa 填）

- AC-001/002/003/004/006：pytest 单测输出，外加生成的 `requirement-index.json` 快照
- AC-005：拒绝 `truth_stage` 写入的 reason_code，以及写入前后文件哈希一致的对比

这两类证据在 implement 与 qa 阶段各自落到证据仓库的对应目录；冻结时它们尚不存在，因此不在本表登记。
