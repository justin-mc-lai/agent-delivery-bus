# Research: pi-curator（Lake B of pi-agent program）

**版本**: v0.1.1
**性质**: planner support_advisory（非 requirement truth）

## 一句话定位

pi agent 作为 ADB 选题策展器：approved 选题池 → 知识检索（personal-brain 文件锚点 + agentmemory 可选）→ 宿主回填证据化选题卡 → 写回 ideas/ → 定时 tick；并入 pi-beacon 扩展（pi 原生加载 beacon/adb/prism 技能）与 driver_pi 有界任务正文。

## 关键事实（source-anchored）

- boundary approved 池与 FSM 已存在（`boundary.py`）；schedule should_run/quota 已存在。
- pi 原生支持 Agent Skills 标准，可加载 `~/.codex/skills` / `~/.claude/skills`（`skills.md`）；扩展系统支持 registerTool/registerCommand/事件（`extensions.md`）。
- prism 是 Beacon 对齐的创作 skill 仓（prism-goal/intel/master/director/produce/qa/release）。
- 真实 pi 验收已通过：driver_pi dispatch → pi 写 manifest → reconcile completed；完整派工单正文会让 pi 长循环 → 需有界正文。

## 湖边界

- 本湖：CuratorService + CLI + pi-beacon 扩展 + 有界正文 + 定时整合。
- 不做：创作生产（prism）、发布、反馈指标、渠道入站。
