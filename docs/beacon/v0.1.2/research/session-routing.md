# Research: session-routing（v0.1.2）

**性质**: planner support_advisory（非 requirement truth）

## 一句话定位

四轴会话身份（channel_thread / host_session / target_executor / target_session / actor）+ SessionRegistry + envelope v1.1，让飞书聊天里的自然语言调度稳定路由到 codex/claude/pi 并回发结果。

## 关键事实（source-anchored）

- hermes sessions store 提供宿主会话 id（`20260812_124240_8cd0f6dd`）；`hermes send --to feishu:oc_xxx[:topic]` 支持回发。
- pi 会话按 cwd 持久化 JSONL，`--session-id` 可固定 resume —— pi 是现成目标会话载体。
- ADB 现有幂等 digest 无会话字段；approval actor 为自由文本。
