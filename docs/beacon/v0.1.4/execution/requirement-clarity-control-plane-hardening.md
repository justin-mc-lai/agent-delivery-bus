# Requirement Clarity: control-plane-hardening (v0.1.4)

## 范围收口

只做四件事：

1. 版本真值唯一化 + 四表面投影（catalog 新建，pyproject/AGENTS/CLAUDE/onboarding 投影，git tag 对齐）。
2. SQLite schema_version 迁移框架 + `adb backup` 备份策略。
3. SPI capabilities 版本化契约，移除 TypeError 回退链。
4. 只跑测试的 CI。

## 明确不做

- 不自动 release / 不推 PyPI。
- 不迁移存量业务数据内容。
- 不改 hermes/pi/beacon 的外部调用语义。
- 不做网络备份或加密备份。

## 验收口径

- `scripts/verify-version-alignment.py`（含 --check-tag）退出码 0。
- `python3 -m pytest -q` 全绿（既有 211 + 新增验收测试）。
- legacy 库迁移、backup smoke、capabilities 协商均有行为测试证据。
