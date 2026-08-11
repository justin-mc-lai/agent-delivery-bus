# PRD: pi-executor (v0.1.0)

**Canonical truth**: [../features/pi-executor/truth.md](../features/pi-executor/truth.md)

## 目标

把 pi agent 接成 ADB 第二执行器：driver_pi ExecutorAdapter（SPI）、goal closure 契约、per-project 执行器路由、本机 smoke；hermes 兼容路径不变。

## 验收要点（AC 摘要）

- AC-PI-001：adapters/pi.py 实现 SPI；pi CLI 缺失 fail-closed。
- AC-PI-002：create_task 幂等，body 含 binding/evidence，回执含 task_id。
- AC-PI-003：goal closure 校验 manifest dispatch_id。
- AC-PI-004：per-project executor=pi 路由；未知执行器 fail-closed。
- AC-PI-005：本机 smoke（dry-run→dispatch→reconcile 闭环）。
- AC-PI-006：illegal 门（auto approve/dispatch/heartbeat/release 全拒）。
- AC-PI-007：hermes 兼容回归。

完整 AC/FSM/非法转移见 canonical truth。
