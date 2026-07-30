---
slug: kanban-ops-nl
version: v0.0.3
materials_status: current
---

# Tests: kanban-ops-nl

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-KAN-001 | AC-KAN-001 | `python3 -m pytest -q tests/test_kanban_ops.py -k fleet_table --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；fleet 表字段稳定 |
| TC-KAN-002 | AC-KAN-002 | `python3 -m pytest -q tests/test_kanban_ops.py -k boards_status --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；boards status 契约 |
| TC-KAN-003 | AC-KAN-003 | `python3 -m pytest -q tests/test_kanban_ops.py -k intent_actions --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；NL action 路由 |
| TC-KAN-004 | AC-KAN-004 | `python3 -m pytest -q tests/test_kanban_ops.py -k awaiting_table --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；待拍板表可合并反馈 |
| TC-KAN-005 | AC-KAN-005 | `python3 -m pytest -q tests/test_kanban_ops.py -k no_private_db --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；无 sqlite 直读 |
| TC-KAN-ILL-001 | AC-KAN-005 | `python3 -m pytest -q tests/test_kanban_ops.py -k illegal_db_read --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；私库读取非法并阻断 |
