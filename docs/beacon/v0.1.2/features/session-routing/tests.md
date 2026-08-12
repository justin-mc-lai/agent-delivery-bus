---
slug: session-routing
version: v0.1.2
materials_status: current
---

# Tests: session-routing

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-SR-001 | AC-SR-001 | `python3 -m pytest -q tests/test_session_routing.py -k 'registry or stale' --tb=short` | exit_code_0；bind/resolve/list/status；stale → session_stale |
| TC-SR-002 | AC-SR-002 | `python3 -m pytest -q tests/test_session_routing.py -k 'envelope or idempotency' --tb=short` | exit_code_0；v1.1 字段；六要素幂等；1.0 兼容 |
| TC-SR-003 | AC-SR-003 | `python3 -m pytest -q tests/test_session_routing.py -k 'intent_agent or candidates' --tb=short` | exit_code_0；--agent 解析；歧义/未知 blocked |
| TC-SR-004 | AC-SR-004 | `python3 -m pytest -q tests/test_session_routing.py -k 'context_inject or session_id' --tb=short` | exit_code_0；Session context 注入；pi --session-id 传递 |
| TC-SR-005 | AC-SR-005 | `python3 -m pytest -q tests/test_session_routing.py -k 'deliver or reply' --tb=short` | exit_code_0；hermes send 回发；失败不影响状态 |
| TC-SR-006 | AC-SR-006 | `python3 -m pytest -q tests/test_session_routing.py -k 'channel_actor' --tb=short` | exit_code_0；不匹配拒绝；未提供兼容 |
| TC-SR-007 | AC-SR-007 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0；全量回归 |
| TC-EXEC-001 | AC-SR-001..007 | `python3 -c "import agent_delivery_bus.session; print('ok')"` | output_ok |
| TC-EXEC-002 | AC-SR-001..007 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |

## Domain FSM QA Matrix

| Walk ID | AC ID | Path type | Assertion |
|---------|-------|-----------|-----------|
| W-SR-01 | AC-SR-005/007 | legal bound→active→dispatch→reconcile→回发 | covered by TC-SR-005/007 |
| W-SR-02 | AC-SR-002 | legal 同线程同意图幂等 | covered by TC-SR-002 |
| W-SR-03 | AC-SR-001 | legal stale→rebound | covered by TC-SR-001 |
| I-SR-01 | AC-SR-002 | illegal 无会话身份派发 | covered by TC-SR-ILL-001 |
| I-SR-02 | AC-SR-002 | illegal 幂等键丢会话轴 | covered by TC-SR-ILL-002 |
| I-SR-03 | AC-SR-003 | illegal 目标静默回落 | covered by TC-SR-ILL-003 |
| I-SR-04 | AC-SR-006 | illegal 渠道身份不匹配放行 | covered by TC-SR-ILL-004 |
| I-SR-05 | AC-SR-007 | illegal release 自动放行 | covered by TC-SR-ILL-005 |

## Illegal TC slots

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-SR-ILL-001 | AC-SR-002 | `python3 -m pytest -q tests/test_session_routing.py -k 'no_session' --tb=short` | exit_code_0；无身份 blocked |
| TC-SR-ILL-002 | AC-SR-002 | `python3 -m pytest -q tests/test_session_routing.py -k 'digest_axes' --tb=short` | exit_code_0；丢轴 digest 不等 |
| TC-SR-ILL-003 | AC-SR-003 | `python3 -m pytest -q tests/test_session_routing.py -k 'silent_fallback' --tb=short` | exit_code_0；未知目标拒绝 |
| TC-SR-ILL-004 | AC-SR-006 | `python3 -m pytest -q tests/test_session_routing.py -k 'actor_mismatch' --tb=short` | exit_code_0；不匹配拒绝 |
| TC-SR-ILL-005 | AC-SR-007 | `python3 -m pytest -q tests/test_session_routing.py -k 'auto_release' --tb=short` | exit_code_0；release 拒绝 |
