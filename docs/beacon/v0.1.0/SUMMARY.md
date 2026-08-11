# Beacon v0.1.0 Summary

## 版本定位

v0.1.0 是 pi-agent 程序的 Lake A：把 pi agent 接入 ADB 作为第二执行器，并补齐 goal 阶段 closure 契约。

## 特性包

- [pi-executor](features/pi-executor/truth.md)：driver_pi ExecutorAdapter（SPI）、goal closure 契约、per-project 执行器路由、本机 smoke、hermes 兼容回归。

## 边界

- pi-curator（选题策展/知识库）在 Lake B，不在本版本。
- 渠道入站桥、审批 actor 身份、CI、跨设备同步不在此版本。
- release 永远人工门。

## 材料顺序

1. `global-boundaries.md`
2. `SUMMARY.md`
3. `execution/index.md` / `execution/architecture-blueprint.md`（存在时）
4. `features/pi-executor/`（truth/tests/tasks/evidence）
