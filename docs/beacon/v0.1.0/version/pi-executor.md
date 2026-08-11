# 版本规划：pi-executor

## 版本定位

- target_version: `v0.1.0`
- base_version: `v0.0.7`
- strategy: `bump:minor`
- owner: `user`
- generated_at: `2026-08-11 12:41:00Z`

## 目标与范围

- 主题：pi-executor
- 目标：pi-agent 程序 Lake A：ADB 第二执行器（driver_pi ExecutorAdapter + goal closure 契约 + 执行器路由 + 本机 smoke）
- 范围：对齐 think/user-story/prd/test-case/workflow/gate 闭环

## 版本演进规则

1. 默认 `--version auto` 取当前最大语义版本。
2. 指定 `--base-version + --bump` 计算下一目标版本。
3. 指定 `--version vX.Y.Z` 走手动版本锁定。

## 追溯

- 推荐随后执行：
  - `beacon evolution-summary create v0.1.0`
  - `beacon workflow start "pi-executor" --version v0.1.0`

