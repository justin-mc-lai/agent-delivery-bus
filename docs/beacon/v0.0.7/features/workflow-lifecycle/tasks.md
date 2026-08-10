---
slug: workflow-lifecycle
version: v0.0.7
materials_status: current
task_source: acceptance_criteria
---

# Tasks: workflow-lifecycle

## Task Ledger

- [ ] TASK-001 Implement AC-WF-001: beacon 生命周期六阶段（plan/truth/implement/qa/freeze/goal）可经 adb 派发，worker 任... · ac=AC-WF-001 · evidence=`.beacon/evidence/implement/workflow-lifecycle/AC-WF-001.json`
- [ ] TASK-002 Implement AC-WF-002: 规范关键词表唯一真值；`adb intent keywords --json` 输出机器可读；飞书/微信/Line 同一句话得到同一 en... · ac=AC-WF-002 · evidence=`.beacon/evidence/implement/workflow-lifecycle/AC-WF-002.json`
- [ ] TASK-003 Implement AC-WF-003: openspec` 可安装并可被项目绑定；预设为 skill 工作流形状（stages→skill/command/evidence） · ac=AC-WF-003 · evidence=`.beacon/evidence/implement/workflow-lifecycle/AC-WF-003.json`
- [ ] TASK-004 Implement AC-WF-004: URL>` 只读盘点 → 产出 analysis request（anchors+commit）→ 宿主 agent 回填 respons... · ac=AC-WF-004 · evidence=`.beacon/evidence/implement/workflow-lifecycle/AC-WF-004.json`
- [ ] TASK-005 Implement AC-WF-005: 全程 JSONL trace（inventory/analysis_request/host_fill/validation/instal... · ac=AC-WF-005 · evidence=`.beacon/evidence/implement/workflow-lifecycle/AC-WF-005.json`
- [ ] TASK-006 Implement AC-WF-006: `adb workflow verify <name>` 验收探针：skill 存在、命令模板可解析、evidence_spec 合法、各... · ac=AC-WF-006 · evidence=`.beacon/evidence/implement/workflow-lifecycle/AC-WF-006.json`
- [ ] TASK-007 Implement AC-WF-007: 工作流绑定后 dispatch→reconcile 闭环；坏库/无锚点字段/危险命令 → fail-closed 不安装 · ac=AC-WF-007 · evidence=`.beacon/evidence/implement/workflow-lifecycle/AC-WF-007.json`
- [ ] TASK-008 Implement AC-WF-008: illegal：无确认安装、危险命令、无证据字段、伪造 trace → fail-closed · ac=AC-WF-008 · evidence=`.beacon/evidence/implement/workflow-lifecycle/AC-WF-008.json`
- [ ] TASK-009 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/workflow-lifecycle/GATES.json`
- [ ] TASK-010 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/workflow-lifecycle/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
