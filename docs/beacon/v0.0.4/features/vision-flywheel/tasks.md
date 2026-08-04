---
slug: vision-flywheel
version: v0.0.4
materials_status: stale
task_source: acceptance_criteria
---

# Tasks: vision-flywheel

## Task Ledger

- [x] TASK-001 Implement AC-FLY-001: `adb schedule register` 登记定时任务（slug/command/engine/cron_expr/quota_li... · ac=AC-FLY-001 · evidence=`.beacon/evidence/implement/vision-flywheel/AC-FLY-001.json`
- [x] TASK-002 Implement AC-FLY-002: `adb schedule list` 输出全部注册条目（含 quota 状态）；`adb schedule show <slug>` 单条目 · ac=AC-FLY-002 · evidence=`.beacon/evidence/implement/vision-flywheel/AC-FLY-002.json`
- [x] TASK-003 Implement AC-FLY-003: `adb schedule should-run <slug>` 确定性门卫链（quota 门卫 → 健康门卫），输出 run/block... · ac=AC-FLY-003 · evidence=`.beacon/evidence/implement/vision-flywheel/AC-FLY-003.json`
- [x] TASK-004 Implement AC-FLY-004: quota 记账：被触发的执行按来源（heartbeat/controller 白名单）计 slot；配额耗尽 → throttled +... · ac=AC-FLY-004 · evidence=`.beacon/evidence/implement/vision-flywheel/AC-FLY-004.json`
- [x] TASK-005 Implement AC-FLY-005: 心跳运行追加写 dispatch ledger（事件流：entry_slug/status/evidence_refs/quota_spe... · ac=AC-FLY-005 · evidence=`.beacon/evidence/implement/vision-flywheel/AC-FLY-005.json`
- [x] TASK-006 Implement AC-FLY-006: 心跳证据对账：复用 truth-gate closure 逻辑，缺证据保持 reconciling；证据齐才 completed · ac=AC-FLY-006 · evidence=`.beacon/evidence/implement/vision-flywheel/AC-FLY-006.json`
- [x] TASK-007 Implement AC-FLY-007: 心跳不得自动 approve/dispatch（illegal）；违规进入 blocked · ac=AC-FLY-007 · evidence=`.beacon/evidence/implement/vision-flywheel/AC-FLY-007.json`
- [x] TASK-008 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/vision-flywheel/GATES.json`
- [x] TASK-009 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/vision-flywheel/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
