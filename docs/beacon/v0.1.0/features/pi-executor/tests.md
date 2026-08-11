---
slug: pi-executor
version: v0.1.0
materials_status: current
---

# Tests: pi-executor

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-PI-001 | AC-PI-001 | `python3 -m pytest -q tests/test_pi_executor.py -k 'spi or cli_missing' --tb=short` | exit_code_0；SPI 方法齐全；pi CLI 缺失 → pi_cli_unavailable |
| TC-PI-002 | AC-PI-002 | `python3 -m pytest -q tests/test_pi_executor.py -k 'create_task or idempotency' --tb=short` | exit_code_0；body 含 binding/evidence；同 key 复用 task |
| TC-PI-003 | AC-PI-003 | `python3 -m pytest -q tests/test_pi_executor.py -k 'goal_closure or manifest' --tb=short` | exit_code_0；goal manifest dispatch_id 校验；缺失/不匹配保持 reconciling |
| TC-PI-004 | AC-PI-004 | `python3 -m pytest -q tests/test_pi_executor.py -k 'routing or unknown' --tb=short` | exit_code_0；executor=pi 按项目解析；未知执行器 fail-closed |
| TC-PI-005 | AC-PI-005 | `python3 -m pytest -q tests/test_pi_executor.py -k 'smoke or reconcile' --tb=short` | exit_code_0；dry-run→dispatch→reconcile 闭环 |
| TC-PI-006 | AC-PI-006 | `python3 -m pytest -q tests/test_pi_executor.py -k 'illegal or auto' --tb=short` | exit_code_0；auto approve/auto dispatch/heartbeat 自动派发全拒 |
| TC-PI-007 | AC-PI-007 | `python3 -m pytest -q tests/test_worker_binding.py tests/test_registry_routing.py --tb=short` | exit_code_0；hermes 兼容路径不变 |
| TC-EXEC-001 | AC-PI-001..007 | `python3 -c "import agent_delivery_bus.adapters.pi; print('ok')"` | output_ok |
| TC-EXEC-002 | AC-PI-001..007 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |

## Domain FSM QA Matrix

| Walk ID | AC ID | Path type | Assertion |
|---------|-------|-----------|-----------|
| W-PI-01 | AC-PI-005 | legal submitted→pi_running→artifact_ready→evidence_pending→closure_verified | covered by TC-PI-005 |
| W-PI-02 | AC-PI-003 | legal goal dispatch→manifest 校验→completed | covered by TC-PI-003 |
| W-PI-03 | AC-PI-007 | legal hermes 兼容 | covered by TC-PI-007 |
| I-PI-01 | AC-PI-006 | illegal 无 pi CLI 静默回落 | covered by TC-PI-ILL-001 |
| I-PI-02 | AC-PI-003 | illegal dispatch_id 不匹配判完成 | covered by TC-PI-ILL-002 |
| I-PI-03 | AC-PI-003 | illegal 伪造/缺失 goal manifest | covered by TC-PI-ILL-003 |
| I-PI-04 | AC-PI-006 | illegal pi auto approve/dispatch | covered by TC-PI-ILL-004 |
| I-PI-05 | AC-PI-006 | illegal release 自动放行 | covered by TC-PI-ILL-005 |

## Illegal TC slots

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-PI-ILL-001 | AC-PI-006 | `python3 -m pytest -q tests/test_pi_executor.py -k 'silent_fallback' --tb=short` | exit_code_0；无 pi CLI 不静默回落 |
| TC-PI-ILL-002 | AC-PI-003 | `python3 -m pytest -q tests/test_pi_executor.py -k 'manifest_mismatch' --tb=short` | exit_code_0；dispatch_id 不匹配保持 reconciling |
| TC-PI-ILL-003 | AC-PI-003 | `python3 -m pytest -q tests/test_pi_executor.py -k 'fake_manifest' --tb=short` | exit_code_0；伪造/缺失 manifest 拒绝 |
| TC-PI-ILL-004 | AC-PI-006 | `python3 -m pytest -q tests/test_pi_executor.py -k 'auto_approve' --tb=short` | exit_code_0；pi 自动审批/派发拒绝 |
| TC-PI-ILL-005 | AC-PI-006 | `python3 -m pytest -q tests/test_pi_executor.py -k 'auto_release' --tb=short` | exit_code_0；release 自动放行拒绝 |
