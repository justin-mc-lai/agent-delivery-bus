# ADB 备份策略

**适用**：本地单机控制面（SQLite 账本 + registry 配置）。备份是治理面的可恢复性兜底，
恢复永远人工执行。

## 备份对象

| 对象 | 路径 | 说明 |
|------|------|------|
| 派发/审批/审计账本 | `data/agent-delivery-bus.sqlite3` | sqlite backup API 在线一致性复制 |
| 活跃 registry 配置 | `config/projects.local.json`（或 `config/projects.json`） | 普通文件复制 |
| 备份清单 | `<dest>/manifest.json` | 时间戳、来源、文件清单、integrity_check |

## 执行

```bash
bin/adb backup --json
# 默认落到 data/backups/adb-backup-<YYYYmmdd-HHMMSS>/
bin/adb backup --dest /Volumes/Backup/adb-2026-08-17 --json
```

策略建议：每天一次（hermes cron 或系统 cron），保留最近 N 份（例如 14 份），
至少一份离开本机磁盘（移动盘/云盘；账本只存 token 的 sha256 哈希，不含明文凭证）。

## 恢复

1. 停止 adb 相关 cron/进程。
2. 用备份目录里的 `agent-delivery-bus.sqlite3` 覆盖 `data/` 下同名文件。
3. 用备份的 `projects*.json` 覆盖 `config/` 下同名文件。
4. `bin/adb doctor --project <slug> --json` 验证；`bin/adb fleet --json` 看全貌。

## 边界

- 备份不含执行器（hermes/pi/beacon）的私有状态；worker 侧状态靠各自机制恢复。
- 备份不自动上传；多机同步不在本策略内。
- 账本属于个人数据，按本机敏感数据处理；恢复永远人工执行。
