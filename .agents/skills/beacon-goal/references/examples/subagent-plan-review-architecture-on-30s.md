# 子代理 + plan 架构/代码审查（plan-review pack）

```text
$beacon-plan / $beacon-goal
目标：启用子代理，对方案做架构设计与代码审查全面分析
版本：v1.6.10
```

```bash
beacon doctor install-subagent-pack -p . --host-runtime codex --pack plan-review
beacon doctor install-subagent-pack -p . --host-runtime claude --pack plan-review
beacon doctor verify-subagent-runtime -p . --host-runtime codex --require-agents --json
```

预期：

- `evaluate_host_subagent_gate` → `pack-review`，**默认 spawn_count=3**：`architect-reviewer` / `code-reviewer` / `knowledge-synthesizer`（catalog 仍 10；`开N个子代理` / `subagent-count=N` / `BEACON_SUBAGENT_MAX` 可调）
- install 写入 **10** 角色到 `.codex/agents` 或 `.claude/agents`
- 无 host spawn 时 planner-review 仍为 `single_process_multi_reviewer` + `fallback_reason`
- 子代理不可 complete/release；Plan→Truth Gate 仍由 truth harness 负责 freeze
