---
slug: pi-executor
version: v0.1.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: pi-executor

## Task Ledger

- [ ] TASK-001 Implement AC-PI-001: adapters/pi.py ExecutorAdapter SPI + pi CLI fail-closed preflight · ac=AC-PI-001 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-001.json`
- [ ] TASK-002 Implement AC-PI-002: create_task 幂等 + body 含 binding/evidence + 回执解析 · ac=AC-PI-002 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-002.json`
- [ ] TASK-003 Implement AC-PI-003: BeaconAdapter.closure(goal) manifest dispatch_id 校验 · ac=AC-PI-003 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-003.json`
- [ ] TASK-004 Implement AC-PI-004: per-project executor=pi 路由 + 阶段策略 + 未知 fail-closed · ac=AC-PI-004 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-004.json`
- [ ] TASK-005 Implement AC-PI-005: 本机 smoke（无 pi CLI blocked / 有 pi CLI 闭环） · ac=AC-PI-005 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-005.json`
- [ ] TASK-006 Implement AC-PI-006: illegal 门（auto approve/dispatch/heartbeat/release） · ac=AC-PI-006 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-006.json`
- [ ] TASK-007 Validate AC-PI-007: hermes 兼容回归 · ac=AC-PI-007 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-007.json`
- [ ] TASK-008 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/pi-executor/GATES.json`
- [ ] TASK-009 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/pi-executor/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists.
