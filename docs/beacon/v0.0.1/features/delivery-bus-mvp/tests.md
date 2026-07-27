---
slug: delivery-bus-mvp
version: v0.0.1
materials_status: current
---

# Tests: delivery-bus-mvp

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-ADB-001 | AC-ADB-001 | `python3 -m unittest tests.test_registry -v` | exit_code==0；覆盖 list、slug/alias/path resolve、重复 slug、alias 冲突和 repo 缺失 |
| TC-ADB-002 | AC-ADB-002 | `python3 -m unittest tests.test_preflight -v` | exit_code==0；strict check 失败返回稳定 reason_code/resume_action 且无写目标仓命令 |
| TC-ADB-003 | AC-ADB-003 | `python3 -m unittest tests.test_approvals -v` | exit_code==0；覆盖 hash-only、expiry、scope、reserve/finalize/release、重放与并发 reservation |
| TC-ADB-004 | AC-ADB-004 | `python3 -m unittest tests.test_storage -v` | exit_code==0；事务回滚、重启恢复、事件 append-only 与 sequence 单调成立 |
| TC-ADB-005 | AC-ADB-005 | `python3 -m unittest tests.test_service.DispatchIdempotencyTests -v` | exit_code==0；重复请求复用 binding，payload 冲突阻断且 Hermes create 仅一次 |
| TC-ADB-006 | AC-ADB-006 | `python3 -m unittest tests.test_adapters.HermesAdapterTests -v` | exit_code==0；JSON CLI argv 含 board/worktree/skill/runtime/retry/idempotency 且不访问 Hermes DB |
| TC-ADB-007 | AC-ADB-007 | `python3 -m unittest tests.test_service.DispatchStateMachineTests -v` | exit_code==0；合法转换留事件，非法转换返回 `invalid_transition` |
| TC-ADB-008 | AC-ADB-008 | `python3 -m unittest tests.test_reconcile -v` | exit_code==0；worker success 仅进入 reconciling，Beacon closure pass 后才 completed |
| TC-ADB-009 | AC-ADB-009 | `python3 -m unittest tests.test_skill_contract -v` | exit_code==0；frontmatter/openai.yaml/symlink 安装契约及 quick_validate 全部通过 |
| TC-ADB-010 | AC-ADB-010 | `python3 -m unittest tests.test_boundaries -v` | exit_code==0；release、target mutation、Hermes DB 和自动修复路径均 fail-closed |
| TC-ADB-CLI | AC-ADB-001, AC-ADB-002, AC-ADB-004 | `PYTHONPATH=src python3 -m agent_delivery_bus.cli --config config/projects.json --db :memory: projects list --json` | exit_code==0；输出合法 JSON，包含 Beacon 源码仓和全部 dispatchable managed 项目 |
