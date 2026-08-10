# ADB + Hermes 派活使用手册（实操版）

**日期**：2026-08-04
**范围**：① vision-flywheel（v0.0.4 已冻结）实现后的定时任务用法；② 现在就能用的 hermes 派活开发流程。
**适用**：`agent-delivery-bus`（ADB 监工）+ `hermes`（工人，已装于 `~/.local/bin/hermes`）。

---

## Part 1 · vision-flywheel：定时任务管家（实现后可用）

愿景：**定时任务有登记、有配额、有证据**。触发交给 hermes cron，判定与记账交给 ADB。

### 1.1 完整链路（以「每天 10:00 跑日榜调研」为例）

```bash
# ① 在 ADB 登记这个定时任务（一次性）
adb schedule register \
  --slug daily-oss-pick \
  --command "bash scripts/daily-trending.sh --writeback" \
  --engine hermes \
  --cron "0 10 * * *" \
  --quota-limit 10          # 每天最多跑 10 次（防重试风暴）

# ② 在 hermes 建 cron 闹钟（到点喊 ADB）
hermes cron create "0 10 * * *" \
  --name "daily-oss-pick-tick" \
  --script adb-schedule-tick.sh \        # 脚本内容见下方
  --workdir $REPO_ROOT

# ③ 每天 10:00 自动发生的事：
#    hermes tick → 脚本跑 `adb schedule should-run daily-oss-pick`
#    → ADB 查配额：还有额度？→ 放行 → 走既有 dispatch 链路派活
#    → 干完 evidence 落盘 → ADB 记账（quota 扣 1 slot）
#    → 没额度？→ blocked(throttled)，等人工放行或调配额
```

`adb-schedule-tick.sh`（放 `~/.hermes/scripts/`）示例：

```bash
#!/bin/bash
# 到点只做两件事：问"该不该跑"；该跑就派
adb schedule should-run "$1" --json | grep -q '"action": "run"' || exit 0
adb dispatch --project "$2" --stage plan --feature "$3" --json
```

### 1.2 日常查看

```bash
adb schedule list                # 所有定时任务 + 配额状态
adb schedule show daily-oss-pick # 单条目详情
adb schedule should-run daily-oss-pick --json   # 手动试问：现在能跑吗
```

### 1.3 铁规矩（设计边界）

- ADB **不自己造闹钟**——定时触发永远走 hermes cron（`cron_owner: hermes`）
- 心跳**不自动派工/不自动发布**——配额用完就停，违规进 blocked
- 每次运行**必须留证据**（evidence 落盘才扣配额）——没证据不算干完

---

## Part 2 · hermes 派活给 agent 开发（现在就能用）

### 2.1 角色

| 角色 | 工具 | 干什么 |
|------|------|--------|
| 监工 | `adb` | 审批、幂等、证据对账（治理） |
| 工人 | `hermes kanban` + coding profile | 领任务、在隔离 worktree 干活、交回执 |
| 派工单 | hermes kanban task body | 内嵌 binding profile + evidence spec（Beacon 为内置参考 profile，可替换） |
| 验收 | `adb reconcile` | 对账证据（truth-gate closure），证据齐才 completed |

### 2.2 标准开发流程（以 beacon 项目加功能为例）

```bash
# ① 派发 plan（不需要审批）
adb dispatch --project beacon --stage plan --feature my-feature --json
#  → preflight 检查（目录存在/git/beacon truth 上下文）→ 建 hermes kanban 任务

# ② 审批 implement（受限阶段，需要一次性 token）
adb approve --actor you --project beacon --stage implement --feature my-feature
#  → 输出一次性 token（默认 900s 有效）

# ③ 派发 implement（带 token）
adb dispatch --project beacon --stage implement --feature my-feature \
  --approval-token <token> --json
#  → 创建 hermes kanban 任务，body 内嵌：
#     ### Beacon worker binding
#     stage: implement → beacon implement skill
#     runner_kind: local_agent · allowed_profiles: (coding, codex)
#     ### Evidence spec
#     evidence_dir: <repo>/.beacon/evidence/implement/my-feature
#     dispatch_id_binding: true → 证据 manifest 必须绑定本次 dispatch

# ④ 工人干活（hermes 侧自动或手动）
hermes kanban list --status todo          # 看待领任务
hermes kanban claim <task-id> --assignee coding   # 工人领活（coding profile）
hermes kanban dispatch <task-id>          # 派工人执行（worktree 隔离 + 超时 + 重试）
#  → coding profile 在 worktree:beacon 干活，按 body 里的 beacon implement 流程

# ⑤ 对账验收（证据齐才 completed）
adb reconcile <dispatch-id> --json
#  → truth-gate 查 .beacon/evidence/implement/my-feature/*.json（含 dispatch_id 归属校验）
#  → 证据齐 → completed；缺证据 → 保持 reconciling
```

### 2.2.1 中立接入：非 Beacon 项目/其他 agent

ADB 本身不要求项目使用 Beacon。项目在 `config/projects.json` 声明自己的
`truth_gate` / `executor` / `binding_profile`（未声明则回落全局配置），
派工单按项目 profile 生成 skill/command 与 evidence spec，closure 由该项目
的 truth gate 决定（Beacon 只是内置参考实现之一）：

```json
{
  "projects": [
    {
      "slug": "other-agent-project",
      "repo": "/path/to/project",
      "docs_root": "docs/truth",
      "docs_version": "v1.0.0",
      "truth_gate": "custom",
      "executor": "hermes",
      "binding_profile": "generic",
      "metadata": {
        "binding_profile": {
          "stages": {
            "implement": {"skill": "my-impl", "command": "run-impl {feature}", "public_harness": "implement"}
          },
          "runner": {"runner_kind": "local_agent", "hermes_assignee": "coding"},
          "evidence_spec": {
            "evidence_dir": ".adb/evidence/implement/{feature}",
            "glob": "*.json",
            "dispatch_id_binding": true
          }
        }
      }
    }
  ]
}
```

### 2.3 轻量路径：不经过 ADB 直接派（日常小活）

```bash
# 直接建任务让 hermes 工人干
hermes kanban create "修复 beacon doctor 的 timeout 解析" \
  --body "按仓库 AGENTS.md 规则修复；跑 pytest 验证；留 evidence" \
  --assignee coding \
  --project beacon \
  --workspace worktree \
  --max-runtime 2h \
  --max-retries 2 \
  --skill agent-delivery-bus

hermes kanban dispatch <task-id>      # 派发
hermes kanban watch <task-id>         # 看进度
```

### 2.4 现在就有的定时能力（不用等 v0.0.4）

```bash
# 定时跑脚本（不带 LLM，纯看门狗）
hermes cron create "0 10 * * *" --name "github-trending" \
  --script fetch-github-trending.sh --no-agent --workdir <repo>

# 定时派 agent 干活（带 LLM）
hermes cron create "every 2h" --name "digest" \
  --prompt "汇总项目看板状态并输出摘要" \
  --skill agent-delivery-bus --deliver local
```

### 2.5 多项目并行与看板

```bash
adb fleet --project beacon           # 看 beacon 项目健康度（active/idle/attention/error）
adb boards sync                      # 同步 hermes kanban 列到 ADB 视图
hermes kanban boards list            # 项目看板列表
```

---

## Part 3 · 常见问题

| 问题 | 回答 |
|------|------|
| 派了活没人干？ | 确认 hermes gateway 在线：`hermes status`；`hermes kanban dispatch` 手动触发 |
| 想撤回派工？ | `hermes kanban reclaim <task-id>` 释放工人占用 |
| 配额用完了？ | `adb schedule show <slug>` 看 next_eligible_at；或人工调配额 |
| 干完没标记完成？ | 先查 evidence 是否存在；`adb reconcile` 对账后再看 |
| 想要 release 自动发？ | 不行——release 硬禁，人工门是铁规矩 |
