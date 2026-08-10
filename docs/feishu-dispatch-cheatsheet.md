# ADB 调度速查卡（飞书置顶）

编号固定于 2026-08-10，只追加不改号。提到「adb」或调度词（派发/派活/派单/分配/调度/审批/同意/验收/对账/待审/看板/任务/状态）即调度类意图。

## 项目编号

| # | slug | 别名 |
|---|------|------|
| 1 | beacon | 灯塔 / beacon源码 |
| 2 | justin-ecommerce | demo / 电商 / demo-market |
| 3 | milemon | 外贸站 / wordpress / b2b |
| 4 | rolo | demo-project / demo-team / 公司项目 |
| 5 | content-creator | creator-demo / creator |
| 6 | content-sync | 自媒体 / content-pilot / pi |
| 7 | shopxo_canada | shopxo / 商城 / demo-store |
| 8 | tool-station-network | 工具站 / 浏览器插件 / fehelper |

## 话术模板

`[编号|项目名] + [动作词] + [stage] + [feature]`

例：`adb 派发 5 的 anydoc implement`；`1 的 worker-beacon-binding 派活 plan`；`待审有什么`；`adb 验收刚才那个`。

## 流程

触发 → intent parse → 回显信封 → 你确认 → dry-run → dispatch → 报 dispatch_id。缺参数只问，不猜。

## 护栏

implement/freeze 需审批；release 永不自动；审批 token 不进聊天。
