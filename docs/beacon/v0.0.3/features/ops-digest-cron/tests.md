---
slug: ops-digest-cron
version: v0.0.3
materials_status: current
---

# Tests: ops-digest-cron

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-DIG-001 | AC-DIG-001 | `python3 -m pytest -q tests/test_ops_digest.py -k render --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；digest 含 fleet/awaiting |
| TC-DIG-002 | AC-DIG-002 | `python3 -m pytest -q tests/test_ops_digest.py -k cron_template --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；cron 模板文档/fixture 存在 |
| TC-DIG-003 | AC-DIG-003 | `python3 -m pytest -q tests/test_ops_digest.py -k feishu_payload --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；飞书载荷字段齐全 |
| TC-DIG-004 | AC-DIG-004 | `python3 -m pytest -q tests/test_ops_digest.py -k idempotent --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；无派工副作用 |
| TC-DIG-005 | AC-DIG-005 | `python3 -m pytest -q tests/test_ops_digest.py -k no_auto_dispatch --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；不自动 approve/dispatch |
| TC-DIG-ILL-001 | AC-DIG-005 | `python3 -m pytest -q tests/test_ops_digest.py -k illegal_dispatch --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；digest→dispatch 非法 |
