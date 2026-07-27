# 30s: one-shot longrun goal

```bash
export BEACON_HOST_GOAL_BRIDGE_CMD='codex exec "$(cat {prompt_file})"'
# or any host CLI that reads the prompt file

beacon goal run "交付灵感池 + 定时任务 + 多平台填充，直到验收" \
  --project . --version v0.0.13 --longrun --json
```

Then:
- host turns auto-requeue via AgentDriver
- check: `beacon goal driver status --run-id <id>`
- stop: `beacon goal driver stop --run-id <id>`
- complete still needs QA verifier; release is human
