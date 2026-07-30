---
slug: nl-intent-envelope
version: v0.0.3
materials_status: current
task_source: acceptance_criteria
---

# Tasks: nl-intent-envelope

## Task Ledger

- [x] TASK-001 Implement AC-INT-001: 提供 `adb intent parse`（及等价 Python API），输出 IntentEnvelope JSON，含 schema... · ac=AC-INT-001 · evidence=`.beacon/evidence/implement/nl-intent-envelope/AC-INT-001.json`
- [x] TASK-002 Implement AC-INT-002: 使用 registry slug/alias/path 解析项目；唯一命中写入 project_slug；零命中 `intent_proj... · ac=AC-INT-002 · evidence=`.beacon/evidence/implement/nl-intent-envelope/AC-INT-002.json`
- [x] TASK-003 Implement AC-INT-003: 多候选歧义时 fail-closed：`intent_project_ambiguous`，data.project_candidates... · ac=AC-INT-003 · evidence=`.beacon/evidence/implement/nl-intent-envelope/AC-INT-003.json`
- [x] TASK-004 Implement AC-INT-004: Hermes skill 合同：展示信封并取得确认前，不得调用 `adb dispatch`；可用文档/fixture 断言确认门 · ac=AC-INT-004 · evidence=`.beacon/evidence/implement/nl-intent-envelope/AC-INT-004.json`
- [x] TASK-005 Implement AC-INT-005: 可将 envelope 的 project/stage/feature 传入既有 `adb assign candidates`，仍只产出候选 · ac=AC-INT-005 · evidence=`.beacon/evidence/implement/nl-intent-envelope/AC-INT-005.json`
- [x] TASK-006 Implement AC-INT-006: parse/confirm 路径不得创建 executor task、不得消费 approve token · ac=AC-INT-006 · evidence=`.beacon/evidence/implement/nl-intent-envelope/AC-INT-006.json`
- [x] TASK-007 Implement AC-INT-007: 提供可执行 pytest 契约覆盖唯一解析、歧义拒绝、非法跳过确认 · ac=AC-INT-007 · evidence=`.beacon/evidence/implement/nl-intent-envelope/AC-INT-007.json`
- [x] TASK-008 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/nl-intent-envelope/GATES.json`
- [x] TASK-009 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/nl-intent-envelope/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
