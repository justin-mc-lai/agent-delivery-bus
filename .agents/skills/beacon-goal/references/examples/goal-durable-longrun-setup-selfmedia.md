# SidePilot：Durable Goal 长跑配置（Beacon v1.6.7）

## 本机已具备

- CLI：`beacon` v1.6.7（`pip install -e` + `beacon doctor setup`）
- Skill：`~/.agents/skills/beacon` → 产品库 skills（含 `beacon-goal` longrun）
- 项目 env 模板：`.beacon/goal-durable.env.example`
- 项目本地 env：`.beacon/goal-durable.env`（可 source）

## 稳定用法（推荐）

```bash
cd /Users/apple/Developer/Personal/products/selfmedia-sync-ai
set -a; source .beacon/goal-durable.env; set +a

# 一句话目标 + 本 run 挂 supervisor + driver（arming）
beacon goal run "..." -p . -v v0.0.13 --longrun --json

# 查看 / 心跳 / 恢复包
beacon goal status --run-id <id> -p . --json
beacon goal heartbeat --run-id <id> -p . --summary "implemented X"
beacon goal checkpoint --run-id <id> -p . --next-action "run tests"
beacon goal resume-pack --run-id <id> -p . --json
beacon goal supervise --run-id <id> -p . --interval-sec 120 --once

# 真自动续轮（需要 bridge）
beacon goal driver start --run-id <id> -p . --loop --max-turns 8
# 或创建时：
beacon goal run "..." -p . -v v0.0.13 --longrun --driver-loop --max-driver-turns 8
```

## 默认 vs longrun

| 模式 | supervisor | driver |
|------|------------|--------|
| 普通 `goal run` | 不挂 | idle |
| `--longrun` | 本 run attach | start（arming） |
| `--longrun --driver-loop` | attach | 进程内多轮 spawn |

## 硬规则

- complete 必须 verifier refs（`beacon goal attach-verifier` / QA scorecard）
- release 永远人工
- resume 只用 digest，不把 full chat 当权威
