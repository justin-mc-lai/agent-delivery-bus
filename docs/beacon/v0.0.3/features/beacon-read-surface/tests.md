---
slug: beacon-read-surface
version: v0.0.3
materials_status: current
---

# Tests: beacon-read-surface

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-BCR-001 | AC-BCR-001 | `python3 -m pytest -q tests/test_beacon_read.py -k version_summary --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；版本摘要字段存在 |
| TC-BCR-002 | AC-BCR-002 | `python3 -m pytest -q tests/test_beacon_read.py -k latest_requirement --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；最新需求摘要只读 |
| TC-BCR-003 | AC-BCR-003 | `python3 -m pytest -q tests/test_beacon_read.py -k intent_action --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；beacon_status 可路由 |
| TC-BCR-004 | AC-BCR-004 | `python3 -m pytest -q tests/test_beacon_read.py -k read_only --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；无写 truth/freeze 副作用 |
| TC-BCR-ILL-001 | AC-BCR-004 | `python3 -m pytest -q tests/test_beacon_read.py -k illegal_write --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；写 truth/freeze 非法 |
