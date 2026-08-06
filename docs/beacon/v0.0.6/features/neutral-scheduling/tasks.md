---
slug: neutral-scheduling
version: v0.0.6
materials_status: current
task_source: acceptance_criteria
---

# Tasks: neutral-scheduling

## Task Ledger

- [x] TASK-001 Implement AC-NS-001: worker binding 改为 profile 制：内置 `beacon` profile 保留现有 `### Beacon work... · ac=AC-NS-001 · evidence=`.beacon/evidence/implement/neutral-scheduling/AC-NS-001.json`
- [x] TASK-002 Implement AC-NS-002: 派工单 body 必含 evidence spec（evidence_dir、glob、dispatch_id 绑定说明），envelop... · ac=AC-NS-002 · evidence=`.beacon/evidence/implement/neutral-scheduling/AC-NS-002.json`
- [x] TASK-003 Implement AC-NS-003: Project 支持 per-project `truth_gate` / `executor` / `binding_profile`，... · ac=AC-NS-003 · evidence=`.beacon/evidence/implement/neutral-scheduling/AC-NS-003.json`
- [x] TASK-004 Implement AC-NS-004: closure 校验证据归属：evidence manifest 中 dispatch_id 必须等于当前 dispatch；缺失、陈旧或... · ac=AC-NS-004 · evidence=`.beacon/evidence/implement/neutral-scheduling/AC-NS-004.json`
- [x] TASK-005 Implement AC-NS-005: 文档中立化：SKILL.md / usage-playbook / README / role-division 研究文档不再宣称 ADB... · ac=AC-NS-005 · evidence=`.beacon/evidence/implement/neutral-scheduling/AC-NS-005.json`
- [x] TASK-006 Implement AC-NS-006: 兼容性：现有 beacon profile 消费者（worker-beacon-binding v0.0.3 契约）关键字段与行为保持；仅... · ac=AC-NS-006 · evidence=`.beacon/evidence/implement/neutral-scheduling/AC-NS-006.json`
- [x] TASK-007 Implement AC-NS-007: 本机交付验收：hermes executor + 注册项目可完成 dispatch（plan 或等价非受限 stage）并 reconci... · ac=AC-NS-007 · evidence=`.beacon/evidence/implement/neutral-scheduling/AC-NS-007.json`
- [x] TASK-008 Implement AC-NS-008: illegal：未知 profile/适配器静默回落、派工单缺 evidence_spec、closure 用无 dispatch_id ... · ac=AC-NS-008 · evidence=`.beacon/evidence/implement/neutral-scheduling/AC-NS-008.json`
- [x] TASK-009 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/neutral-scheduling/GATES.json`
- [x] TASK-010 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/neutral-scheduling/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
