# User Story: pi-executor (v0.1.0)

**Canonical truth**: [../features/pi-executor/truth.md](../features/pi-executor/truth.md)

## 旅程

1. 用户登记项目 `executor=pi` 或阶段策略指定 goal/长程走 pi。
2. 用户对承载 adb 的 agent 说派发意图 → intent parse → 确认。
3. `adb dispatch --dry-run`：pi CLI 缺失 → pi_cli_unavailable blocked；就绪 → pass。
4. `adb dispatch`：pi 执行绑定 skill，产出带 dispatch_id 的证据 manifest。
5. `adb reconcile`：goal closure 校验 manifest；匹配 → completed；缺失/不匹配 → reconciling。

## 边界

- pi 不自动审批/派发；release 永远人工门。
- hermes 短任务路径与既有行为不变。
