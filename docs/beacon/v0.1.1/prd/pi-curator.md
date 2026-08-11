# PRD: pi-curator (v0.1.1)

**Canonical truth**: [../features/pi-curator/truth.md](../features/pi-curator/truth.md)

## 目标

approved 选题池 → 知识检索 → 证据化选题卡 → personal-brain 写回 → 定时 tick；pi-beacon 扩展 + driver_pi 有界正文。

## 验收要点

- AC-PC-001：CuratorService + request 构建。
- AC-PC-002：knowledge_scan + agentmemory 降级。
- AC-PC-003：选题卡 schema 校验。
- AC-PC-004：写回 + 越界 fail-closed。
- AC-PC-005：curator CLI + ledger。
- AC-PC-006：pi-beacon 扩展 + install.sh + 有界正文。
- AC-PC-007：定时整合 + 全量回归。
