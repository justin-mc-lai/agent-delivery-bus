---
slug: worker-beacon-binding
version: v0.0.3
materials_status: current
task_source: acceptance_criteria
---

# Tasks: worker-beacon-binding

## Task Ledger

- [x] TASK-001 Implement AC-WRK-001: dispatch 创建的 Hermes task body 含 stage→Beacon skill/命令绑定字段（至少 plan） · ac=AC-WRK-001 · evidence=`.beacon/evidence/implement/worker-beacon-binding/AC-WRK-001.json`
- [x] TASK-002 Implement AC-WRK-002: runner 约定使用 Hermes coding profile 或显式 Codex/等价本机 agent，不得假定云端集群调度器 · ac=AC-WRK-002 · evidence=`.beacon/evidence/implement/worker-beacon-binding/AC-WRK-002.json`
- [x] TASK-003 Implement AC-WRK-003: workspace admission/预检失败时不得创建成功派工；返回稳定 reason_code · ac=AC-WRK-003 · evidence=`.beacon/evidence/implement/worker-beacon-binding/AC-WRK-003.json`
- [x] TASK-004 Implement AC-WRK-004: implement/freeze 仍要求既有 approve token；绑定层不得绕过 · ac=AC-WRK-004 · evidence=`.beacon/evidence/implement/worker-beacon-binding/AC-WRK-004.json`
- [x] TASK-005 Implement AC-WRK-005: goal-stage-binding 默认 defer：未升格前 ENABLED_STAGES 不含 goal，或显式 blocked r... · ac=AC-WRK-005 · evidence=`.beacon/evidence/implement/worker-beacon-binding/AC-WRK-005.json`
- [x] TASK-006 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/worker-beacon-binding/GATES.json`
- [x] TASK-007 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/worker-beacon-binding/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
