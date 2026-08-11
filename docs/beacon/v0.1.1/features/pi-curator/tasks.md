---
slug: pi-curator
version: v0.1.1
materials_status: current
task_source: acceptance_criteria
---

# Tasks: pi-curator

## Task Ledger

- [ ] TASK-001 Implement AC-PC-001: `pi_curator.py` CuratorService：读取 boundary approved 池；`tick` 对无提案空跑 p... · ac=AC-PC-001 · evidence=`.beacon/evidence/implement/pi-curator/AC-PC-001.json`
- [ ] TASK-002 Implement AC-PC-002: knowledge_scan 扫描 knowledge_root（ideas/daily/bases）按 topic token 命中文件... · ac=AC-PC-002 · evidence=`.beacon/evidence/implement/pi-curator/AC-PC-002.json`
- [ ] TASK-003 Implement AC-PC-003: 选题卡 schema（curator-card.v1）校验：topic/sources/knowledge_refs/market_sig... · ac=AC-PC-003 · evidence=`.beacon/evidence/implement/pi-curator/AC-PC-003.json`
- [ ] TASK-004 Implement AC-PC-004: 写回 personal-brain：`<knowledge_root>/ideas/<slug>-<date>.md` + YAML fr... · ac=AC-PC-004 · evidence=`.beacon/evidence/implement/pi-curator/AC-PC-004.json`
- [ ] TASK-005 Implement AC-PC-005: CLI：`adb curator list --status approved` / `request --proposal <id>` ... · ac=AC-PC-005 · evidence=`.beacon/evidence/implement/pi-curator/AC-PC-005.json`
- [ ] TASK-006 Implement AC-PC-006: pi-beacon 扩展包：`skills/pi-beacon/extension.ts`（registerTool adb_dispat... · ac=AC-PC-006 · evidence=`.beacon/evidence/implement/pi-curator/AC-PC-006.json`
- [ ] TASK-007 Implement AC-PC-007: 定时整合：复用 ScheduleService should_run/quota 与 boundary cron 形态；curator 不... · ac=AC-PC-007 · evidence=`.beacon/evidence/implement/pi-curator/AC-PC-007.json`
- [ ] TASK-008 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/pi-curator/GATES.json`
- [ ] TASK-009 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/pi-curator/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
