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
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/ | present | truth |
| review | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/program-review-p2.md | present | truth |
| implement | support_advisory | .beacon/evidence/implement/requirement-index/ | pending | implement |
| qa | qa_verdict | .beacon/evidence/qa/v0.1.5/requirement-index/ | pending | qa |

## 证据要求（冻结后由 implement/qa 填）

- AC-001/002/003/004/006：pytest 单测输出 + 生成的 `requirement-index.json` 快照
- AC-005：拒绝 `truth_stage` 写入的 reason_code 与文件未变的哈希对比
