---
name: adb
description: Agent-safe interface to the Agent Delivery Bus. Use to inspect registered projects, parse intent into standard dispatch envelopes, and dry-run proposals. Approval and real dispatch are HUMAN-ONLY — this skill never grants write capability to agents.
---

# adb — Agent Delivery Bus（agent 安全接口）

adb 是项目交付的控制面。本 skill 定义 agent（flywheel、hermes、pi 等）如何安全接入：
**只读查询 + dry-run 提案，审批与真实派发永远人工。**

## 白名单命令（agent 可用）

```bash
# 项目注册表
bin/adb projects list --json                     # 全部注册项目
bin/adb doctor --project <slug> --json           # 单项目就绪度

# 提案解析（把自然语言工单变成标准 envelope）
bin/adb intent parse --utterance "dispatch <项目编号> <feature> <stage> <desc>" --json

# dry-run 试派（幂等，不落账、不派活）
bin/adb dispatch --project <slug> --stage plan --feature <feature> --dry-run --json
```

提案话术格式（英文动词 + 编号 + 短 feature 名 + stage）：
`dispatch <编号> <feature-slug> <plan|qa> <one-line-desc>`
例：`dispatch 10 login-fix plan fix login timeout issue`

## 硬禁例（agent 违反即缺陷）

- ❌ `adb approve ...` — 审批只许人工
- ❌ `adb dispatch ...`（非 --dry-run）— 真实派发只许人工
- ❌ 读取/使用审批 token、approval_id
- ❌ 修改注册表、配额、账本

## 提案闭环（agent 正确姿势）

1. 确认哪个项目在册：`adb projects list --json`
2. 生成工单文本（上述格式）
3. 校验解析：`adb intent parse --utterance "<文本>" --json` → 确认 stage/project/feature 已填充
4. 试派：`adb dispatch --project <slug> --stage plan --feature <f> --dry-run --json`
5. 把 dry-run envelope 原样交给人工，等待审批 → 审批后由 hermes/pi 执行

> 生产入口网关（如 headlong 的 adb-wrap）在物理层拒绝 approve 与非 dry-run 派发——这不是提示词约定，是硬边界。本 skill 与网关契约一致。
