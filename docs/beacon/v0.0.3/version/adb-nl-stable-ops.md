# 版本规划：adb-nl-stable-ops

## 版本定位

- target_version: `v0.0.3`
- base_version: `v0.0.2`
- strategy: `bump:patch`
- owner: `justin`
- generated_at: `2026-07-30 07:23:30Z`

## 目标与范围

- 主题：adb-nl-stable-ops
- 目标：NL-stable ADB ops program: intent envelope through digest lakes
- 范围：对齐 think/user-story/prd/test-case/workflow/gate 闭环

## 版本演进规则

1. 默认 `--version auto` 取当前最大语义版本。
2. 指定 `--base-version + --bump` 计算下一目标版本。
3. 指定 `--version vX.Y.Z` 走手动版本锁定。

## 追溯

- 推荐随后执行：
  - `beacon evolution-summary create v0.0.3`
  - `beacon workflow start "adb-nl-stable-ops" --version v0.0.3`

