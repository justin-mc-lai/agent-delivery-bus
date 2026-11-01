---
slug: skill-trigger
version: v0.1.5
materials_status: draft
---

# Evidence Index: skill-trigger

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v0.1.5/features/skill-trigger/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v0.1.5/features/skill-trigger/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v0.1.5/features/skill-trigger/tasks.md | present | truth |
| program | support_advisory | docs/beacon/v0.1.5/programs/superagent-router/feature-graph.json | present | truth |

## 证据要求（冻结后由 implement 与 qa 填）

- AC-001：skill 文档新增小节的实际内容
- AC-002..006：`go test -run ...` 的输出
- AC-004：一次真实 `adbd feed` 的 stdout 与 decisions.jsonl 对应行
