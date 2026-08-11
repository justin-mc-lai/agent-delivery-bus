---
slug: pi-curator
version: v0.1.1
materials_status: current
---

# Tests: pi-curator

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-PC-001 | AC-PC-001 | `python3 -m pytest -q tests/test_pi_curator.py -k 'approved_pool or empty_tick or request' --tb=short` | exit_code_0；approved 读取；空池 tick pass；request 含 anchors |
| TC-PC-002 | AC-PC-002 | `python3 -m pytest -q tests/test_pi_curator.py -k 'knowledge_scan or fallback' --tb=short` | exit_code_0；文件锚点命中；agentmemory 不可用降级 |
| TC-PC-003 | AC-PC-003 | `python3 -m pytest -q tests/test_pi_curator.py -k 'card_validate' --tb=short` | exit_code_0；缺锚点拒绝 |
| TC-PC-004 | AC-PC-004 | `python3 -m pytest -q tests/test_pi_curator.py -k 'write_card or outside_root' --tb=short` | exit_code_0；写卡成功；越界 fail-closed |
| TC-PC-005 | AC-PC-005 | `python3 -m pytest -q tests/test_pi_curator.py -k 'cli' --tb=short` | exit_code_0；list/request/apply/tick 可用；ledger 落账 |
| TC-PC-006 | AC-PC-006 | `python3 -m pytest -q tests/test_pi_beacon.py --tb=short` | exit_code_0；extension.ts 注册 adb_dispatch/prism；install.sh --dry-run 幂等；bounded body 追加 |
| TC-PC-007 | AC-PC-007 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0；全量回归 |
| TC-EXEC-001 | AC-PC-001..007 | `python3 -c "import agent_delivery_bus.pi_curator; print('ok')"` | output_ok |
| TC-EXEC-002 | AC-PC-001..007 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |

## Domain FSM QA Matrix

| Walk ID | AC ID | Path type | Assertion |
|---------|-------|-----------|-----------|
| W-PC-01 | AC-PC-004/005 | legal requested→anchored→filled→validated→written→recorded | covered by TC-PC-004/TC-PC-005 |
| W-PC-02 | AC-PC-001 | legal 空 approved 池空跑 | covered by TC-PC-001 |
| W-PC-03 | AC-PC-006 | legal 扩展安装幂等 | covered by TC-PC-006 |
| I-PC-01 | AC-PC-003 | illegal 无锚点直接写卡 | covered by TC-PC-ILL-001 |
| I-PC-02 | AC-PC-004 | illegal 越界写 | covered by TC-PC-ILL-002 |
| I-PC-03 | AC-PC-007 | illegal 自动 apply/审批 | covered by TC-PC-ILL-003 |
| I-PC-04 | AC-PC-004 | illegal 伪造/缺失 dispatch_id | covered by TC-PC-ILL-004 |
| I-PC-05 | AC-PC-007 | illegal release 自动放行 | covered by TC-PC-ILL-005 |

## Illegal TC slots

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-PC-ILL-001 | AC-PC-003 | `python3 -m pytest -q tests/test_pi_curator.py -k 'no_anchor' --tb=short` | exit_code_0；无锚点拒绝 |
| TC-PC-ILL-002 | AC-PC-004 | `python3 -m pytest -q tests/test_pi_curator.py -k 'outside_root' --tb=short` | exit_code_0；越界拒绝 |
| TC-PC-ILL-003 | AC-PC-007 | `python3 -m pytest -q tests/test_pi_curator.py -k 'auto_apply' --tb=short` | exit_code_0；自动 apply/审批拒绝 |
| TC-PC-ILL-004 | AC-PC-004 | `python3 -m pytest -q tests/test_pi_curator.py -k 'dispatch_id' --tb=short` | exit_code_0；缺失/伪造 dispatch_id 拒绝 |
| TC-PC-ILL-005 | AC-PC-007 | `python3 -m pytest -q tests/test_pi_curator.py -k 'auto_release' --tb=short` | exit_code_0；release 自动放行拒绝 |
