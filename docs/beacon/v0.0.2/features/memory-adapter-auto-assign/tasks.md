---
slug: memory-adapter-auto-assign
version: v0.0.2
materials_status: current
task_source: acceptance_criteria
---

# Tasks: memory-adapter-auto-assign

## Task Ledger

- [x] TASK-001 Implement AC-MEM-001: 提供位于 ADB 核心外的 MemoryAdapter SPI；dispatch 前按 project_slug 做 scoped rec... · ac=AC-MEM-001 · evidence=`.beacon/evidence/implement/memory-adapter-auto-assign/AC-MEM-001.json`
- [x] TASK-002 Implement AC-MEM-002: reconcile 进入 completed/blocked 终态后调用 MemoryAdapter.writeback，写入含 proj... · ac=AC-MEM-002 · evidence=`.beacon/evidence/implement/memory-adapter-auto-assign/AC-MEM-002.json`
- [x] TASK-003 Implement AC-MEM-003: 跨项目 ACL：以项目 B scope 召回不得返回项目 A 写入的记忆；越权命中必须 fail-closed 并有可执行测试断言 · ac=AC-MEM-003 · evidence=`.beacon/evidence/implement/memory-adapter-auto-assign/AC-MEM-003.json`
- [x] TASK-004 Implement AC-MEM-004: 自动分配：规则+评分器仅产出 dispatch candidates（含 score/reason），不得直接创建 executor ta... · ac=AC-MEM-004 · evidence=`.beacon/evidence/implement/memory-adapter-auto-assign/AC-MEM-004.json`
- [x] TASK-005 Implement AC-MEM-005: 受限阶段派工仍必须经现有 ApprovalService.issue/reserve/finalize；候选转派工路径只能消费有效 app... · ac=AC-MEM-005 · evidence=`.beacon/evidence/implement/memory-adapter-auto-assign/AC-MEM-005.json`
- [x] TASK-006 Implement AC-MEM-006: 提供 awaiting_approval / 待拍板列表（CLI/JSON），并可经 Hermes 飞书通道渲染待拍板事项（项目/阶段/f... · ac=AC-MEM-006 · evidence=`.beacon/evidence/implement/memory-adapter-auto-assign/AC-MEM-006.json`
- [x] TASK-007 Implement AC-MEM-007: 拍板签发有效令牌后，允许对该 project/stage/feature 执行 in-project agent dispatch；无令牌... · ac=AC-MEM-007 · evidence=`.beacon/evidence/implement/memory-adapter-auto-assign/AC-MEM-007.json`
- [x] TASK-008 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/memory-adapter-auto-assign/GATES.json`
- [x] TASK-009 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/memory-adapter-auto-assign/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
