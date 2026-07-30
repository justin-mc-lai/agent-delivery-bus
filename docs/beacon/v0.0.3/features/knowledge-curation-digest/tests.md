---
slug: knowledge-curation-digest
version: v0.0.3
materials_status: current
---

# Tests: knowledge-curation-digest

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-KNW-001 | AC-KNW-001 | `python3 -m pytest -q tests/test_knowledge_curation.py -k curation_entry --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；curation 入口/合同存在 |
| TC-KNW-002 | AC-KNW-002 | `python3 -m pytest -q tests/test_knowledge_curation.py -k cron_trigger --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；cron 触发合同 |
| TC-KNW-003 | AC-KNW-003 | `python3 -m pytest -q tests/test_knowledge_curation.py -k no_adb_sqlite_body --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；不写 ADB SQLite 正文 |
| TC-KNW-004 | AC-KNW-004 | `python3 -m pytest -q tests/test_knowledge_curation.py -k no_auto_truth --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；禁止灵感直接变 truth |
| TC-KNW-ILL-001 | AC-KNW-003, AC-KNW-004 | `python3 -m pytest -q tests/test_knowledge_curation.py -k illegal_persist_or_freeze --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；非法持久化/freeze 被阻断 |
