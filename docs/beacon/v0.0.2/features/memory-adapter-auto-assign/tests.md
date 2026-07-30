---
slug: memory-adapter-auto-assign
version: v0.0.2
materials_status: current
---

# Tests: memory-adapter-auto-assign

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-MEM-001 | AC-MEM-001 | `python3 -m pytest -q tests/test_memory_adapter.py -k recall_before_dispatch --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；dispatch 前调用 scoped recall，且 adapter 不在 core 模块内硬编码 agentmemory |
| TC-MEM-002 | AC-MEM-002 | `python3 -m pytest -q tests/test_memory_adapter.py -k writeback_after_reconcile --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；reconcile 后写回含 project/stage/feature/dispatch_id；写回失败不改变 reconcile status |
| TC-MEM-003 | AC-MEM-003 | `python3 -m pytest -q tests/test_memory_acl.py --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；项目 B scope 召回不含项目 A 记忆；越权 fail-closed |
| TC-MEM-004 | AC-MEM-004 | `python3 -m pytest -q tests/test_auto_assign.py -k candidates_only --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；评分器只产出 candidates，不创建 executor task |
| TC-MEM-005 | AC-MEM-005 | `python3 -m pytest -q tests/test_auto_assign.py -k approve_still_required --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；受限阶段无 token 仍 approval_required；有 token 才可 dispatch |
| TC-MEM-006 | AC-MEM-006 | `python3 -m pytest -q tests/test_pending_approvals.py --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；待拍板列表含 project/stage/feature/expires；可渲染飞书/文本通道载荷 |
| TC-MEM-007 | AC-MEM-007 | `python3 -m pytest -q tests/test_pending_approvals.py -k post_approve_dispatch --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；approve 后允许项目内 agent dispatch |
| TC-MEM-ILL-001 | AC-MEM-004, AC-MEM-005 | `python3 -m pytest -q tests/test_auto_assign.py -k illegal_skip_approve --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；candidates_ready→dispatched 与 score→consume_token 等 illegal 路径 fail-closed |
| TC-MEM-ILL-002 | AC-MEM-003, AC-MEM-002 | `python3 -m pytest -q tests/test_memory_acl.py -k illegal_cross_project --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；cross_project→recall_success 与 writeback_failure→erase_reconcile 均为 illegal 并被阻断 |
| TC-MEM-CLI | AC-MEM-004, AC-MEM-006 | `PYTHONPATH=src python3 -m agent_delivery_bus.cli --config config/projects.json --db :memory: assign candidates --json` | exit_code==0；输出 candidates JSON 数组且不含 task_id |
