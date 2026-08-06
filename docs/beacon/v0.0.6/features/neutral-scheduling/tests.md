---
slug: neutral-scheduling
version: v0.0.6
materials_status: current
---

# Tests: neutral-scheduling

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-NS-001 | AC-NS-001 | `python3 -m pytest -q tests/test_worker_binding.py -k 'profile or generic or neutral' --tb=short` | exit_code_0 |
| TC-NS-002 | AC-NS-002 | `python3 -m pytest -q tests/test_worker_binding.py -k 'evidence_spec or schema' --tb=short` | exit_code_0 |
| TC-NS-003 | AC-NS-003 | `python3 -m pytest -q tests/test_registry_routing.py --tb=short` | exit_code_0 |
| TC-NS-004 | AC-NS-004 | `python3 -m pytest -q tests/test_reconcile_evidence_ownership.py --tb=short` | exit_code_0 |
| TC-NS-005 | AC-NS-005 | `rg -n '参考实现|reference implementation' README.md docs/usage-playbook.md skills/agent-delivery-bus/SKILL.md docs/beacon-adb-role-division.md` | matches_found |
| TC-NS-006 | AC-NS-006 | `python3 -m pytest -q tests/test_worker_binding.py -k beacon --tb=short` | exit_code_0 |
| TC-NS-007 | AC-NS-007 | `python3 -m pytest -q tests/test_local_delivery_smoke.py --tb=short` | exit_code_0 |
| TC-NS-ILL-001 | AC-NS-008 | `python3 -m pytest -q tests/test_registry_routing.py -k 'unknown or fail_closed' --tb=short` | exit_code_0；未知 profile/适配器 fail-closed |
| TC-NS-ILL-002 | AC-NS-008 | `python3 -m pytest -q tests/test_worker_binding.py -k 'missing_evidence_spec' --tb=short` | exit_code_0；缺 evidence spec 拒绝 emit |
| TC-NS-ILL-003 | AC-NS-008 | `python3 -m pytest -q tests/test_reconcile_evidence_ownership.py -k 'mismatch or stale' --tb=short` | exit_code_0；dispatch_id 不匹配保持 reconciling |
| TC-NS-ILL-004 | AC-NS-008 | `python3 -m pytest -q tests/test_reconcile_evidence_ownership.py -k 'no_evidence' --tb=short` | exit_code_0；无证据不判完成 |
| TC-EXEC-001 | AC-NS-002 | `python3 -c "import agent_delivery_bus, beacon; print('ok')"` | output_ok |
| TC-EXEC-002 | AC-NS-001..008 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |

## Domain FSM QA Matrix

| Walk ID | AC ID | Path type | Assertion |
|---------|-------|-----------|-----------|
| W-NS-01 | AC-NS-001 | legal profile_requested→profile_resolved→binding_emitted | covered by TC-NS-001/TC-NS-002（beacon 与自定义 profile 均 emit） |
| W-NS-02 | AC-NS-004 | legal evidence_pending→closure_verified | covered by TC-NS-004（manifest.dispatch_id 匹配） |
| W-NS-03 | AC-NS-003 | legal project 路由（显式优先 + 全局回落） | covered by TC-NS-003 |
| I-NS-01 | AC-NS-008 | illegal 未知 profile/适配器静默回落 | covered by TC-NS-ILL-001 |
| I-NS-02 | AC-NS-008 | illegal 无 evidence_spec 直接 emit | covered by TC-NS-ILL-002 |
| I-NS-03 | AC-NS-008 | illegal dispatch_id 不匹配判完成 | covered by TC-NS-ILL-003 |
| I-NS-04 | AC-NS-008 | illegal 无证据直接判完成 | covered by TC-NS-ILL-004 |

## Validation Note

v1.6.10 TC layout：每个 AC 至少一行行为级 Command+Assertion；TC-EXEC-* 为 exec-layer 域测试。
