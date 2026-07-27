# Agent Delivery Bus

本地 Agent 调度控制面：项目注册、Beacon 严格预检、一次性审批、SQLite 审计、
Hermes Kanban 幂等派工和 Beacon/Hermes 对账。

首版明确不自动 release、不修复目标项目、不读取 Hermes 内部数据库。

```bash
bin/adb projects list --json
bin/adb doctor --project beacon --json
bin/adb dispatch --project beacon --stage plan --feature example --dry-run --json
```

默认配置是 `config/projects.json`，默认本地状态库是
`data/agent-delivery-bus.sqlite3`。可用全局参数 `--config`、`--db` 覆盖。

开发验收使用 pytest 生成 Beacon QA9 所需的 JUnit 证据：

```bash
python3 -m pip install -e '.[test]'
python3 -m pytest -q
```
