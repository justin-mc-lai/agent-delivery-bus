# Feishu 对话话术 Playbook（ADB 调度）

飞书聊天里用自然语言指挥 adb 的固定契约。编号 2026-08-10 冻结，只追加不改号。

## 固定编号速查卡

| # | slug | 别名 |
|---|------|------|
| 1 | beacon | 灯塔 / beacon-core / beacon源码 |
| 2 | justin-ecommerce | demo / 电商 / demo-market / liuzai |
| 3 | milemon | 外贸站 / wordpress / b2b |
| 4 | rolo | demo-project / demo-team / 公司项目 |
| 5 | content-creator | creator-demo / creator |
| 6 | content-sync | 自媒体 / content-pilot / pi |
| 7 | shopxo_canada | shopxo / 商城 / demo-store |
| 8 | tool-station-network | 工具站 / 浏览器插件 / fehelper |

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

## 固定流程

触发 → `adb intent parse --utterance "<原话>"` → 回显信封（编号/项目/stage/feature/动作）→ 用户确认 → dry-run → dispatch → 报 dispatch_id。

缺参数：只问缺的那个，或列候选编号；绝不猜项目。

## 示例对话

- 用户：`adb 派发 5 的 anydoc implement` → 回显信封 → 确认 → approve + dispatch
- 用户：`1 的 worker-beacon-binding 派活 plan` → 回显信封 → 确认 → dispatch（plan 免审批）
- 用户：`待审有什么` → 飞书待审卡片（只读）
- 用户：`adb 验收刚才那个` → task show + reconcile

## 护栏

- implement/freeze 必须一次性 approval token；release 永远禁用。
- 审批 token 绝不写入聊天或日志。
- 未确认信封绝不 dispatch。
