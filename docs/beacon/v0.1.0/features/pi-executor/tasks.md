---
slug: pi-executor
version: v0.1.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: pi-executor

## Task Ledger

- [x] TASK-001 Implement AC-PI-001: `adapters/pi.py` 实现 ExecutorAdapter SPI（name=pi；preflight/board/works... · ac=AC-PI-001 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-001.json`
- [x] TASK-002 Implement AC-PI-002: create_task 通过 pi CLI 创建任务，任务 body 含 binding manifest + evidence spec... · ac=AC-PI-002 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-002.json`
- [x] TASK-003 Implement AC-PI-003: goal closure 契约：`BeaconAdapter.closure(stage="goal")` 校验 `<repo>/.bea... · ac=AC-PI-003 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-003.json`
- [x] TASK-004 Implement AC-PI-004: per-project 执行器路由：`executor=pi` 按项目解析；阶段→执行器策略（默认 hermes 兼容）；未知执行器 fa... · ac=AC-PI-004 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-004.json`
- [x] TASK-005 Implement AC-PI-005: 本机 smoke：无 pi CLI → dry-run blocked（pi_cli_unavailable）；有 pi CLI → dr... · ac=AC-PI-005 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-005.json`
- [x] TASK-006 Implement AC-PI-006: illegal：pi 自动 approve/auto-dispatch 拒绝；heartbeat 不得自动派发 pi；release 永远人工门 · ac=AC-PI-006 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-006.json`
- [x] TASK-007 Implement AC-PI-007: 兼容性：hermes 默认路径与 AdapterResolver 全局默认不变；既有 145 测试保持全绿 · ac=AC-PI-007 · evidence=`.beacon/evidence/implement/pi-executor/AC-PI-007.json`
- [x] TASK-008 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/pi-executor/GATES.json`
- [ ] TASK-009 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/pi-executor/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
