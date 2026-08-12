---
slug: channel-session-hardening
version: v0.1.3
materials_status: current
---

# Tests: channel-session-hardening

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-CH-001 | AC-CH-001 | `python3 -m pytest -q tests/test_channel_session.py -k 'channel_aware or unsupported' --tb=short` | exit_code_0；platform 感知；未知渠道 blocked |
| TC-CH-002 | AC-CH-002 | `python3 -m pytest -q tests/test_channel_session.py -k 'task_session or auto' --tb=short` | exit_code_0；并发任务独立会话；三态解析 |
| TC-CH-003 | AC-CH-003 | `python3 -m pytest -q tests/test_channel_session.py -k 'lease or busy' --tb=short` | exit_code_0；busy 拒绝；release 后可再 acquire |
| TC-CH-004 | AC-CH-004 | `python3 -m pytest -q tests/test_channel_session.py -k 'resolution' --tb=short` | exit_code_0；决议顺序 + source；文档含决议顺序 |
| TC-CH-005 | AC-CH-005 | `python3 -m pytest -q tests/test_channel_session.py -k 'deliver' --tb=short` | exit_code_0；按渠道回发目标 |
| TC-CH-006 | AC-CH-006 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0；全量回归 |
| TC-EXEC-001 | AC-CH-001..006 | `python3 -c "import agent_delivery_bus.session; print('ok')"` | output_ok |
| TC-EXEC-002 | AC-CH-001..006 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |

## Domain FSM QA Matrix

| Walk ID | AC ID | Path type | Assertion |
|---------|-------|-----------|-----------|
| W-CH-01 | AC-CH-002 | legal 同线程并发两任务独立会话 | covered by TC-CH-002 |
| W-CH-02 | AC-CH-003 | legal acquire→release→acquire | covered by TC-CH-003 |
| W-CH-03 | AC-CH-001/005 | legal 三渠道同脚本 | covered by TC-CH-001/005 |
| I-CH-01 | AC-CH-003 | illegal 并发同固定会话 | covered by TC-CH-ILL-001 |
| I-CH-02 | AC-CH-003 | illegal release 不匹配 | covered by TC-CH-ILL-002 |
| I-CH-03 | AC-CH-001 | illegal 渠道未知静默 feishu | covered by TC-CH-ILL-003 |
| I-CH-04 | AC-CH-006 | illegal release 自动放行 | covered by TC-CH-ILL-004 |

## Illegal TC slots

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-CH-ILL-001 | AC-CH-003 | `python3 -m pytest -q tests/test_channel_session.py -k 'busy' --tb=short` | exit_code_0；第二个 dispatch blocked |
| TC-CH-ILL-002 | AC-CH-003 | `python3 -m pytest -q tests/test_channel_session.py -k 'release_mismatch' --tb=short` | exit_code_0；不匹配拒绝 |
| TC-CH-ILL-003 | AC-CH-001 | `python3 -m pytest -q tests/test_channel_session.py -k 'unsupported' --tb=short` | exit_code_0；未知渠道拒绝 |
| TC-CH-ILL-004 | AC-CH-006 | `python3 -m pytest -q tests/test_channel_session.py -k 'auto_release' --tb=short` | exit_code_0；release 拒绝 |
