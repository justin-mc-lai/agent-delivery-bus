---
slug: control-plane-hardening
version: v0.1.4
materials_status: current
task_source: acceptance_criteria
---

# Tasks: control-plane-hardening

## Task Ledger

- [x] TASK-001 Implement AC-CPH-001: version-truth-catalog + version_truth 模块 + 校验脚本 · ac=AC-CPH-001 · evidence=`.beacon/evidence/implement/control-plane-hardening/AC-CPH-001.json`
- [x] TASK-002 Implement AC-CPH-002: --check-tag 支持 · ac=AC-CPH-002 · evidence=`.beacon/evidence/implement/control-plane-hardening/AC-CPH-002.json`
- [x] TASK-003 Implement AC-CPH-003: schema_version 迁移框架 + schema_migrations · ac=AC-CPH-003 · evidence=`.beacon/evidence/implement/control-plane-hardening/AC-CPH-003.json`
- [x] TASK-004 Implement AC-CPH-004: legacy 库升级迁移（executor_board/channel_actor 等） · ac=AC-CPH-004 · evidence=`.beacon/evidence/implement/control-plane-hardening/AC-CPH-004.json`
- [x] TASK-005 Implement AC-CPH-005: `adb backup` + manifest + 备份文档 · ac=AC-CPH-005 · evidence=`.beacon/evidence/implement/control-plane-hardening/AC-CPH-005.json`
- [x] TASK-006 Implement AC-CPH-006: adapter capabilities + resolver 签名协商，移除 TypeError 回退 · ac=AC-CPH-006 · evidence=`.beacon/evidence/implement/control-plane-hardening/AC-CPH-006.json`
- [x] TASK-007 Implement AC-CPH-007: .github/workflows/ci.yml（仅测试） · ac=AC-CPH-007 · evidence=`.beacon/evidence/implement/control-plane-hardening/AC-CPH-007.json`
- [x] TASK-008 Implement AC-CPH-008: 兼容回归 + 新增验收测试 · ac=AC-CPH-008 · evidence=`.beacon/evidence/implement/control-plane-hardening/AC-CPH-008.json`
- [x] TASK-009 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/control-plane-hardening/GATES.json`
- [x] TASK-010 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/control-plane-hardening/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
