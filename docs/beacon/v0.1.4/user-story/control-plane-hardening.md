# User Story: control-plane-hardening (v0.1.4)

## 目标

作为 ADB 运营者，我希望控制面的版本、存储与适配器契约可审计、可升级、可回放，
并且每次代码变更都有自动化质量门，这样我才能放心地把真实项目继续交给它调度。

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-CPH-001 | version-truth-catalog.md 存在且机器可解析；pyproject.version / AGENTS.md / CLAUDE.md / onboarding state 与 catalog 投影一致；提供 `version_truth` 模块 + `scripts/verify-version-alignment.py` 校验 |
| AC-CPH-002 | 校验脚本支持 `--check-tag`：git tag 与 catalog.git_tag 一致（release 后成立） |
| AC-CPH-003 | Storage 有 schema_version 迁移框架：`PRAGMA user_version` + `schema_migrations` 审计表；legacy 库升级（executor_board / channel_actor 等）可回放；重复 initialize 幂等 |
| AC-CPH-004 | `adb backup` 命令备份 SQLite 账本（在线一致性复制）+ projects json + manifest；`--dest` 可指定目标；恢复步骤写入文档 |
| AC-CPH-005 | 每个 ExecutorAdapter 声明 `capabilities`；service 按 capabilities 传参（skills/session_id）；adapter resolver 按声明签名调用；全程无 `except TypeError` 回退链 |
| AC-CPH-006 | `.github/workflows/ci.yml` 只跑测试 + 版本校验；不包含任何 release/publish 步骤 |
| AC-CPH-007 | 兼容回归：既有 211 测试全绿；新增验收测试覆盖 AC-CPH-001..006 |
