# ADB 调度速查卡（飞书置顶）

编号固定于 2026-08-10，只追加不改号。提到「adb」或调度词（派发/派活/派单/分配/调度/审批/同意/验收/对账/待审/看板/任务/状态）即调度类意图。

## 项目编号（示例；以你本地注册表为准）

| # | slug | 别名 |
|---|------|------|
| 1 | demo-app | app / demo |
| 2 | demo-content | content / creator |
| 3 | demo-web | web / site |
| 4 | demo-ai | ai / agent |

真实编号由 `adb projects list --numbered` 输出（机器强制，只追加不改号）。

## 话术模板

`[编号|项目名] + [动作词] + [stage] + [feature]`

例：`adb 派发 2 的 demo-feature implement`；`1 的 demo-feature 派活 plan`；`待审有什么`；`adb 验收刚才那个`。

## 流程

触发 → intent parse → 回显信封 → 你确认 → dry-run → dispatch → 报 dispatch_id。缺参数只问，不猜。

## 护栏

implement/freeze 需审批；release 永不自动；审批 token 不进聊天。

## 项目管理

- 登记：`adb projects register --slug <slug> --class <platform|managed|knowledge> --repo <path>`
- 列表（机器编号）：`adb projects list --numbered`
- 删除（软删除，需确认）：`adb projects delete <编号|slug> --yes`
- 恢复：`adb projects restore <编号|slug>`

编号只追加不改号；删除后编号保留、项目不可派发、可恢复。
