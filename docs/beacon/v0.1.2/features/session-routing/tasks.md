---
slug: session-routing
version: v0.1.2
materials_status: current
task_source: acceptance_criteria
---

# Tasks: session-routing

## Task Ledger

- [x] TASK-001 Implement AC-SR-001: thread\ · ac=AC-SR-001 · evidence=`.beacon/evidence/implement/session-routing/AC-SR-001.json`
- [x] TASK-002 Implement AC-SR-002: dispatch envelope v1.1：normalized_request 增 channel/channel_thread/ac... · ac=AC-SR-002 · evidence=`.beacon/evidence/implement/session-routing/AC-SR-002.json`
- [x] TASK-003 Implement AC-SR-003: codex · ac=AC-SR-003 · evidence=`.beacon/evidence/implement/session-routing/AC-SR-003.json`
- [x] TASK-004 Implement AC-SR-004: 任务 body 注入 `### Session context`；pi create_task 支持 session_id → `--se... · ac=AC-SR-004 · evidence=`.beacon/evidence/implement/session-routing/AC-SR-004.json`
- [x] TASK-005 Implement AC-SR-005: reconcile 回发：dispatch 账本记录 channel_thread；HermesAdapter.deliver() 用 `... · ac=AC-SR-005 · evidence=`.beacon/evidence/implement/session-routing/AC-SR-005.json`
- [x] TASK-006 Implement AC-SR-006: 审批渠道身份：approve 支持 `--channel-actor <open_id>`；reserve 校验绑定；不匹配 → appr... · ac=AC-SR-006 · evidence=`.beacon/evidence/implement/session-routing/AC-SR-006.json`
- [x] TASK-007 Implement AC-SR-007: 自然语言全流程：bind → intent parse --agent → envelope 确认 → dispatch → pi 执行 ... · ac=AC-SR-007 · evidence=`.beacon/evidence/implement/session-routing/AC-SR-007.json`
- [x] TASK-008 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/session-routing/GATES.json`
- [x] TASK-009 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/session-routing/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
