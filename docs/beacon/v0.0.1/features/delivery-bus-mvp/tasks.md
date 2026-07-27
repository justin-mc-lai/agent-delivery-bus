---
slug: delivery-bus-mvp
version: v0.0.1
materials_status: stale
task_source: acceptance_criteria
---

# Tasks: delivery-bus-mvp

## Task Ledger

- [x] TASK-001 Implement AC-ADB-001: `config/projects.json` 是唯一项目注册真值；CLI 能 list，并按 slug、唯一 alias 或 canoni... · ac=AC-ADB-001 · evidence=`.beacon/evidence/implement/delivery-bus-mvp/AC-ADB-001.json`
- [x] TASK-002 Implement AC-ADB-002: `doctor` 和每次 dispatch 前执行只读 strict preflight：repo/git、docs root、声明 do... · ac=AC-ADB-002 · evidence=`.beacon/evidence/implement/delivery-bus-mvp/AC-ADB-002.json`
- [x] TASK-003 Implement AC-ADB-003: implement/freeze/release scope 使用带 actor、project、stage、feature、expiry... · ac=AC-ADB-003 · evidence=`.beacon/evidence/implement/delivery-bus-mvp/AC-ADB-003.json`
- [x] TASK-004 Implement AC-ADB-004: SQLite 原子持久化 projects snapshot、approvals、dispatches、dispatch_events；事... · ac=AC-ADB-004 · evidence=`.beacon/evidence/implement/delivery-bus-mvp/AC-ADB-004.json`
- [x] TASK-005 Implement AC-ADB-005: dispatch 使用规范化请求生成稳定 SHA-256 idempotency key；相同请求返回同一 dispatch/Hermes... · ac=AC-ADB-005 · evidence=`.beacon/evidence/implement/delivery-bus-mvp/AC-ADB-005.json`
- [x] TASK-006 Implement AC-ADB-006: Hermes adapter 仅通过公开 JSON CLI 创建项目 board 和 task；task 必须绑定 coding assi... · ac=AC-ADB-006 · evidence=`.beacon/evidence/implement/delivery-bus-mvp/AC-ADB-006.json`
- [x] TASK-007 Implement AC-ADB-007: dispatch 状态机覆盖 draft、awaiting_approval、queued、dispatched、reconciling、... · ac=AC-ADB-007 · evidence=`.beacon/evidence/implement/delivery-bus-mvp/AC-ADB-007.json`
- [x] TASK-008 Implement AC-ADB-008: `task show/list` 与 `reconcile` 合并本地 ledger、Hermes JSON 状态和 Beacon 阶段证... · ac=AC-ADB-008 · evidence=`.beacon/evidence/implement/delivery-bus-mvp/AC-ADB-008.json`
- [x] TASK-009 Implement AC-ADB-009: 提供 `skills/agent-delivery-bus`，SKILL frontmatter 仅含 name/description，... · ac=AC-ADB-009 · evidence=`.beacon/evidence/implement/delivery-bus-mvp/AC-ADB-009.json`
- [x] TASK-010 Implement AC-ADB-010: MVP 不读取 Hermes SQLite、不直接修改目标 repo、不自动执行 Beacon 修复、不自动 release；releas... · ac=AC-ADB-010 · evidence=`.beacon/evidence/implement/delivery-bus-mvp/AC-ADB-010.json`
- [x] TASK-011 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/delivery-bus-mvp/GATES.json`
- [x] TASK-012 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/delivery-bus-mvp/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
