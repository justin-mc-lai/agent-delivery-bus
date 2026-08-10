# Feishu 对话话术 Playbook（ADB 调度）

飞书聊天里用自然语言指挥 adb 的固定契约。编号 2026-08-10 冻结，只追加不改号。

## 固定编号速查卡（示例；以本地注册表为准）

| # | slug | 别名 |
|---|------|------|
| 1 | demo-app | app / demo |
| 2 | demo-content | content / creator |
| 3 | demo-web | web / site |
| 4 | demo-ai | ai / agent |

真实编号 = `adb projects list --numbered` 输出（机器强制，只追加不改号）。

## 触发词门

调度类意图 = 提到「adb」或 {派发, 派活, 派单, 分配, 调度, 审批, 同意, 验收, 对账, 待审, 看板, 任务, 状态}

- 查询类（状态/待审/列表/看板）→ 直接执行，无需确认。
- 写类（派发/审批）→ 必须先确认门。

## 动作词 → 命令族

| 动作词 | 命令 |
|--------|------|
| 派发 / 派活 / 派单 / 分配 | `adb dispatch` |
| 审批 / 同意 / 放行 | `adb approve` → `adb dispatch` |
| 验收 / 对账 / 结果 | `adb task show` / `adb reconcile` |
| 状态 / 待审 / 列表 / 看板 | `adb fleet` / `adb projects list` / `adb approvals awaiting --channel feishu` |
| 登记 / 注册 / 新增 / 立项 | `adb projects register --slug ... --class ... --repo ...` |
| 删除 / 移除 / 归档 | `adb projects delete <编号|slug> --yes`（需确认） |
| 恢复 | `adb projects restore <编号|slug>` |
| 安装 / 登记工作流 | `adb workflow install --name <名> --preset <superpowers|openspec>` |
| 列出工作流 / 预设 | `adb workflow list` |
| 删除工作流 | `adb workflow remove <名> --yes`（需确认） |
| 接入 / 适配开源库 | `adb workflow ingest --source <url|路径> --name <名>`（宿主 agent 回填后 draft apply） |
| 验证 / 排查工作流 | `adb workflow verify` / `adb workflow trace` / `adb workflow debug` |

## 项目管理状态机

```text
register（编号 = max+1，只追加不改号）
        ↓
    active（可派发，编号固定）
        │ delete（软删除，需确认 --yes）
        ↓
    archived（不可派发，编号保留）
        │ restore
        └─────────────→ active
```

- 编号唯一、不可变、不重用；`adb projects list --numbered` 输出即机器强制编号。
- delete 是软删除（归档），可 restore；未确认（无 --yes）一律拒绝。
- archived 项目 dispatch 被拒（project_not_dispatchable）。
- 工作流与调度解耦：预设只有开源第三方（aider/openhands）；私有工作流
  （如 beacon-goal）只放本地注册表，不进预设。派发会把绑定 skill
  force-load 进 worker，目标设备缺 skill 会 fail-closed。
- 关键词：渠道统一查 `adb intent keywords --json`（飞书/微信/Line 同表）。
- 接入开源库：adb 只盘点并出题，宿主 agent 回填 response，校验→草案→确认→verify→绑定。

## 固定流程

触发 → `adb intent parse --utterance "<原话>"` → 回显信封（编号/项目/stage/feature/动作）→ 用户确认 → dry-run → dispatch → 报 dispatch_id。

缺参数：只问缺的那个，或列候选编号；绝不猜项目。

## 示例对话

- 用户：`adb 派发 2 的 demo-feature implement` → 回显信封 → 确认 → approve + dispatch
- 用户：`1 的 demo-feature 派活 plan` → 回显信封 → 确认 → dispatch（plan 免审批）
- 用户：`待审有什么` → 飞书待审卡片（只读）
- 用户：`adb 验收刚才那个` → task show + reconcile
- 用户：`登记新项目 my-tool，class managed，repo /path/to/my-tool` → 回显待登记信息 → 确认 → register（自动分配编号）
- 用户：`删除项目 2` → 回显 `#2 demo-content 将归档` → 确认 → delete --yes
- 用户：`恢复项目 2` → restore
- 用户：`安装工作流 openspec，名字 my-spec` → 回显待安装 → 确认 → install
- 用户：`列出工作流` → 预设 + 本地工作流列表
- 用户：`删除工作流 my-spec` → 回显 → 确认 → remove --yes
- 用户：`接入工作流 https://github.com/xxx/yyy，名字 my-wf` → ingest → 宿主回填 → 确认 → 安装
- 用户：`验证工作流 my-wf` → verify 报告

## 护栏

- implement/freeze 必须一次性 approval token；release 永远禁用。
- 审批 token 绝不写入聊天或日志。
- 未确认信封绝不 dispatch。
