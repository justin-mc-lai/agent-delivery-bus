# 子代理 + 全面审查

```text
$beacon-goal
目标：启用子代理对该方案做全面审查与 challenge
版本：v1.6.8
```

```bash
beacon doctor install-subagent-pack -p . --host-runtime codex --pack minimal
beacon doctor verify-subagent-runtime -p . --host-runtime codex --require-agents --json
```
