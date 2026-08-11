# Research: pi-executor（Lake A of pi-agent program）

**版本**: v0.1.0
**性质**: planner support_advisory（非 requirement truth）
**日期**: 2026-08-11

## 一句话定位

pi agent（oh-my-pi）作为 ADB 的第二执行器：ADB 通过 ExecutorAdapter SPI 派发，pi 执行绑定 skill 工作流并产出证据；goal 阶段 closure 契约同步补齐，长程任务可 reconcile 收口。

## 关键事实（source-anchored）

- ExecutorAdapter SPI 已存在（`src/agent_delivery_bus/adapters/spi.py`），现有实现 hermes/null；pi 未接入。
- `BeaconAdapter.closure` 目前只有 plan/implement/qa/freeze 分支，goal 返回 stage_not_enabled —— goal 派发后 reconcile 无法收口（上轮 F2）。
- 本机 `pi` CLI 未安装；oh-my-pi 源码在 `oss-sync/agents/agent-systems/oh-my-pi` —— driver_pi 必须 fail-closed（pi_cli_unavailable）。
- per-project 执行器路由已存在（registry `executor` 字段 + AdapterResolver），可直接扩展。
- hermes 兼容路径与 145 测试为回归基线，不得破坏。

## 湖边界

- 本湖：driver_pi 适配器、goal closure 契约、执行器路由、本机 smoke。
- 下一湖（Lake B pi-curator）：approved 选题池 → 知识检索 → 证据选题卡 → personal-brain 回写；知识库相关能力不在本湖。

## 验收锚

- 无 pi CLI → `pi_cli_unavailable` fail-closed。
- goal manifest 缺失/不匹配 → evidence_ownership_mismatch，保持 reconciling。
- hermes 默认路径不变，既有测试全绿。
