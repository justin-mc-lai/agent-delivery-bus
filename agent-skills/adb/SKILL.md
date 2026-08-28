---
name: adb
description: Agent-safe interface to the Agent Delivery Bus. Inspect registered projects, parse intent into standard dispatch envelopes, dry-run and dispatch REVERSIBLE work autonomously (f3 R4). Irreversible actions and real publish stay human-gated. Never fabricate; check the queue before dispatching.
---

# adb — Agent Delivery Bus（agent 安全接口）

adb 是项目交付的控制面。本 skill 定义 agent（flywheel、hermes、pi 等）如何安全接入：
**L0 只读查询 + L2 可逆自主派发 + L3 不可逆人工。**

## 白名单命令（agent 可用）

```bash
# 项目注册表 / 就绪度 / 队列（L0 只读）
bin/adb projects list --json                     # 全部注册项目
bin/adb doctor --project <slug> --json           # 单项目就绪度
bin/adb task list --project <slug> --json        # 该项目当前任务队列（派发前必查）

# 提案解析（L0）
bin/adb intent parse --utterance "dispatch <编号> <feature> <stage> <desc>" --json

# dry-run 试派（幂等，不落账、不派活）
bin/adb dispatch --project <slug> --stage plan --feature <feature> --dry-run --json

# 可逆自主派发（f3 R4，L2）—— 写文件+commit到feature分支，不 push main
bin/adb dispatch --project <slug> --stage implement --feature "<任务描述>" --reversible --json
```

提案话术格式：`dispatch <编号> <feature> <plan|qa|implement> <one-line-desc>`
例：`dispatch 12 l1-identity implement add GET /api/me/sessions`

## 派发纪律（f3 R4，防僵尸派发）—— 每次 dispatch 前必做

1. `adb task list --project <slug> --json` 查队列
2. **若同 feature 已在队列/派发中 → 不重复派发**，改为检查它的状态或等它完成
3. 只有队列干净时才派发新任务
4. 同一时刻对同一项目只保留 1-2 个 in-flight 任务，不要堆积

## 可逆 vs 不可逆（f3 R4 硬边界）

- ✅ **可逆自主**：写文件、改代码、commit 到 feature 分支、跑测试、生成内容（`--reversible`）
- 🔒 **永远人工**：真实上架/发布、DELETE/DROP、push main、生产配置、freeze/release 阶段
- 任务描述里**不要写不可逆动作**（gate 会拦：`真实上架`/`push main`/`delete` 等触发 L3 deny）

## 硬禁例（agent 违反即缺陷）

- ❌ `adb approve ...` — 审批只许人工
- ❌ dispatch 不带 `--reversible` 的 implement — 人工门（除非确实需要人工）
- ❌ 读取/使用审批 token、approval_id
- ❌ 修改注册表、配额、账本
- ❌ 同一任务重复派发（先查队列）

## 提案闭环（agent 正确姿势）

1. 查项目在册：`adb projects list --json`
2. **查队列**：`adb task list --project <slug> --json` → 无重复再继续
3. 生成工单：`dispatch <编号> <feature> <stage> <desc>`
4. 校验解析：`adb intent parse --utterance "<文本>" --json`
5. 可逆任务直接派发：`adb dispatch --project <slug> --stage implement --feature "<desc>" --reversible --json`
6. 结果写回 deliverables/dispatch-results.jsonl，下次唤醒读它复盘

> 生产入口网关（headlong 的 adb-wrap）在物理层拒绝不可逆动作与人工专属操作——这不是提示词约定，是硬边界。本 skill 与网关契约一致。
