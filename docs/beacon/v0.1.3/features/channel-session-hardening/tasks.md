---
slug: channel-session-hardening
version: v0.1.3
materials_status: current
task_source: acceptance_criteria
---

# Tasks: channel-session-hardening

## Task Ledger

- [x] TASK-001 Implement AC-CH-001: 入站桥 channel-aware：payload 取 platform（feishu/weixin/line/telegram 等）；b... · ac=AC-CH-001 · evidence=`.beacon/evidence/implement/channel-session-hardening/AC-CH-001.json`
- [x] TASK-002 Implement AC-CH-002: fixed:<id> · ac=AC-CH-002 · evidence=`.beacon/evidence/implement/channel-session-hardening/AC-CH-002.json`
- [x] TASK-003 Implement AC-CH-003: SessionRegistry.acquire/release lease：同 fixed 会话并发第二个未完成 dispatch → s... · ac=AC-CH-003 · evidence=`.beacon/evidence/implement/channel-session-hardening/AC-CH-003.json`
- [x] TASK-004 Implement AC-CH-004: 目标决议顺序固化：显式 --target-executor > 线程绑定 target > 项目 executor_policy.stag... · ac=AC-CH-004 · evidence=`.beacon/evidence/implement/channel-session-hardening/AC-CH-004.json`
- [x] TASK-005 Implement AC-CH-005: 按渠道回发：dispatch 账本记录 channel；reconcile deliver 目标 `<channel>:<thread>`... · ac=AC-CH-005 · evidence=`.beacon/evidence/implement/channel-session-hardening/AC-CH-005.json`
- [x] TASK-006 Implement AC-CH-006: 兼容回归：旧绑定（共享 target_session）仍可解析；既有 177 测试全绿 · ac=AC-CH-006 · evidence=`.beacon/evidence/implement/channel-session-hardening/AC-CH-006.json`
- [x] TASK-007 Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/channel-session-hardening/GATES.json`
- [x] TASK-008 Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/channel-session-hardening/QA-MATRIX.json`

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists (completion ≠ self-tick).
