---
slug: search-boundary-curation
version: v0.0.5
materials_status: current
---

# Evidence: search-boundary-curation

## Authority Table

| surface | authority |
|---------|-----------|
| docs/beacon/v0.0.5/features/search-boundary-curation/truth.md | requirement_truth |
| docs/beacon/v0.0.5/features/search-boundary-curation/tests.md | test_truth |
| docs/beacon/v0.0.5/features/search-boundary-curation/tasks.md | support_advisory |
| .beacon/evidence/implement/search-boundary-curation/*.json | acceptance_truth |
| .beacon/qa/features/search-boundary-curation/scorecard.json | qa_verdict |

## Sources

- User utterance: 定时整理新的网络搜索边界 → 进待审 → 你拍板
- Prior lake: `docs/beacon/v0.0.4/features/vision-flywheel/truth.md`
- Related: `docs/beacon/v0.0.3/features/knowledge-curation-digest/truth.md`
- Hermes cron: script-only tick → ingest

## Implement evidence

已填充：`.beacon/evidence/implement/search-boundary-curation/AC-SBC-00*.json` + QA scorecard；junit=`.beacon/junit-search-boundary-curation.xml`
