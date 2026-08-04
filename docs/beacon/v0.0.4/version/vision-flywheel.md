# 版本规划：vision-flywheel

## 版本定位

- target_version: `v0.0.4`
- base_version: `v0.0.3`
- strategy: `bump:minor`
- owner: `justin`
- generated_at: `2026-08-04 00:00:00Z`

## 目标与范围

- 主题：personal-production-flywheel（个人 AI 生产飞轮）
- 目标：ADB 调度心跳层——把愿景（docs/vision-first-principles.md）立项为正式 feature；本版本闭包 ADB 侧「调度 + quota + 心跳证据对账」能力
- 范围：对齐 truth → implement → qa → release 闭环；跨项目环节（反馈回路/选题池化/pi 执行器/多格式）进入 Phased Backlog

## 版本演进规则

1. 默认 `--version auto` 取当前最大语义版本。
2. 指定 `--base-version + --bump` 计算下一目标版本。
3. 指定 `--version vX.Y.Z` 走手动版本锁定。

## 追溯

- 愿景源：`docs/vision-first-principles.md`（2026-08-03 修订版，源码级现状核查）
- 相关研究：`docs/beacon/research/beacon-longrun-stability-analysis-2026-08.md`、`docs/beacon/research/beacon-longrun-pi-agent-evolution-2026-08.md`
- 推荐随后执行：
  - `beacon evolution-summary create v0.0.4`
  - `beacon workflow start "vision-flywheel" --version v0.0.4`
