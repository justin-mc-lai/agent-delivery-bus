---
slug: search-boundary-curation
version: v0.0.5
materials_status: current
---

# Tests: search-boundary-curation

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-SBC-001 | AC-SBC-001 | `python3 -m pytest -q tests/test_boundary.py -k ingest --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；ingest→pending；缺 topic 拒绝 |
| TC-SBC-002 | AC-SBC-002 | `python3 -m pytest -q tests/test_boundary.py -k pending_show --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；pending 列表与 show 单条 |
| TC-SBC-003 | AC-SBC-003 | `python3 -m pytest -q tests/test_boundary.py -k decide --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；approve→approved；reject→rejected |
| TC-SBC-004 | AC-SBC-004 | `python3 -m pytest -q tests/test_boundary.py -k list_status --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；list 默认 active；status 过滤 |
| TC-SBC-005 | AC-SBC-005 | `python3 -m pytest -q tests/test_boundary.py -k awaiting_view --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；待拍板含 boundary_pending |
| TC-SBC-006 | AC-SBC-006 | `python3 -m pytest -q tests/test_boundary.py -k schedule_tick --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；tick fixture 只 ingest 不 approve |
| TC-SBC-007 | AC-SBC-007 | `python3 -m pytest -q tests/test_boundary.py -k no_auto --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；禁止 auto-approve |
| TC-SBC-ILL-001 | AC-SBC-007 | `python3 -m pytest -q tests/test_boundary.py -k illegal_activate --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；ingest→active 非法 |
| TC-SBC-ILL-002 | AC-SBC-007 | `python3 -m pytest -q tests/test_boundary.py -k skip_pending --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；跳过 pending 启用非法 |
