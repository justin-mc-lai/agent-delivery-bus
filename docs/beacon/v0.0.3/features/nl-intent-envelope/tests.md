---
slug: nl-intent-envelope
version: v0.0.3
materials_status: current
---

# Tests: nl-intent-envelope

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-INT-001 | AC-INT-001 | `python3 -m pytest -q tests/test_intent_parse.py -k envelope_schema --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；输出含 schema_version/status/blocked/reason_code/data.envelope |
| TC-INT-002 | AC-INT-002 | `python3 -m pytest -q tests/test_intent_parse.py -k unique_alias_resolve --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；唯一 alias 解析为 project_slug |
| TC-INT-003 | AC-INT-003 | `python3 -m pytest -q tests/test_intent_parse.py -k ambiguous_project --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；歧义返回 intent_project_ambiguous 且 candidates>1 |
| TC-INT-004 | AC-INT-004 | `python3 -m pytest -q tests/test_intent_confirm_gate.py --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；未确认不得进入 dispatch 调用路径 |
| TC-INT-005 | AC-INT-005 | `python3 -m pytest -q tests/test_intent_parse.py -k assign_bridge --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；envelope 字段可驱动 assign candidates 且无 task_id |
| TC-INT-006 | AC-INT-006 | `python3 -m pytest -q tests/test_intent_parse.py -k no_side_effects --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；parse 不创建 executor task、不消费 approve token |
| TC-INT-007 | AC-INT-007 | `python3 -m pytest -q tests/test_intent_parse.py tests/test_intent_confirm_gate.py --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；契约套件全绿 |
| TC-INT-ILL-001 | AC-INT-004, AC-INT-006 | `python3 -m pytest -q tests/test_intent_confirm_gate.py -k illegal_skip_confirm --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；skip confirm / parse→task / parse→token 均 fail-closed |
| TC-INT-ILL-002 | AC-INT-003 | `python3 -m pytest -q tests/test_intent_parse.py -k illegal_silent_pick --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；禁止静默挑选第一候选 |
| TC-INT-CLI | AC-INT-001 | `PYTHONPATH=src python3 -m agent_delivery_bus.cli --config config/projects.json --db :memory: intent parse --utterance "beacon plan demo" --json` | exit_code==0 或 blocked 带稳定 reason_code；JSON 含 schema_version |
