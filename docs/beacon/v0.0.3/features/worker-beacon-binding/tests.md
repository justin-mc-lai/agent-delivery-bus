---
slug: worker-beacon-binding
version: v0.0.3
materials_status: current
---

# Tests: worker-beacon-binding

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-WRK-001 | AC-WRK-001 | `python3 -m pytest -q tests/test_worker_binding.py -k task_body_plan --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；task body 含 plan skill 绑定 |
| TC-WRK-002 | AC-WRK-002 | `python3 -m pytest -q tests/test_worker_binding.py -k runner_profile --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；runner profile 显式 |
| TC-WRK-003 | AC-WRK-003 | `python3 -m pytest -q tests/test_worker_binding.py -k admission_fail_closed --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；admission 失败 blocked |
| TC-WRK-004 | AC-WRK-004 | `python3 -m pytest -q tests/test_worker_binding.py -k approve_still_required --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；受限阶段仍要 token |
| TC-WRK-005 | AC-WRK-005 | `python3 -m pytest -q tests/test_worker_binding.py -k goal_deferred --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；goal 未升格时拒绝或 defer reason |
| TC-WRK-ILL-001 | AC-WRK-004 | `python3 -m pytest -q tests/test_worker_binding.py -k illegal_skip_approve --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；非法绕过 approve fail-closed |
| TC-WRK-ILL-002 | AC-WRK-005 | `python3 -m pytest -q tests/test_worker_binding.py -k illegal_goal_enable --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；未升格启用 goal 非法 |
