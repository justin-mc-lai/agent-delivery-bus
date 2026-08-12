# User Story: session-routing (v0.1.2)

**Canonical truth**: [../features/session-routing/truth.md](../features/session-routing/truth.md)

## 旅程

1. 飞书说"派发给 pi 实现 xxx"。
2. 宿主 bind 会话 → `adb intent parse --agent pi` → 回显 envelope。
3. 确认 → dispatch（六要素幂等）→ pi 固定 session-id 执行。
4. reconcile → 结果回发原线程。
