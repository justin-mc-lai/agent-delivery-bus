# Agent Delivery Bus

[English](#english) | [中文](#中文)

Governed local control plane for multi-project agent delivery.

Not a wiki. Not a generic job queue. Not an autopilot release bot.

It answers one hard question safely:

> Which project, which stage, with whose approval, through which executor, and what evidence counts as done?

---

## English

### Why this exists

AI coding agents fail in predictable ways:

- they guess the wrong repository
- they retry and create duplicate work
- they treat "worker said done" as "product is done"
- they rewrite freely when collaboration rules are missing

Agent Delivery Bus (ADB) is a thin control plane that separates four authorities:

| Authority | Owner |
|-----------|-------|
| Project routing | Registry (`config/projects.json`) |
| Execution lifecycle | Executor adapter (example: Hermes Kanban) |
| Delivery verdict | Truth-gate adapter (example: Beacon) |
| Approval + idempotency + audit | ADB SQLite ledger |

Knowledge belongs elsewhere. A personal knowledge OS can feed intent into ADB, but ADB does not store inspiration, methods, or notes.

### Core architecture

```text
Human / Knowledge OS intent
            |
            v
   Agent Delivery Bus Core
 registry -> preflight -> approval -> idempotent dispatch -> reconcile
     |           |                          |                   |
     |           +-- TruthGateAdapter       |                   +-- evidence
     |           +-- ExecutorAdapter        v
     +-- projects.json                 example: Hermes
```

#### Open-source shape

- **Core**: registry, preflight orchestration, one-time approvals, SQLite ledger, dispatch FSM, reconcile loop
- **Adapter SPI**: `TruthGateAdapter` + `ExecutorAdapter`
- **Example adapters**: Beacon (truth gate), Hermes (executor)
- **Skill**: `skills/agent-delivery-bus`
- **Collaboration rules template**: `skills/collaboration-rules-template`
- **Runtime deps**: none (Python stdlib + SQLite)

### What it does

- resolve projects by slug / alias / path (no fuzzy guessing)
- run read-only strict preflight before real side effects
- require one-time scoped approval for `implement` / `freeze`
- create idempotent executor tasks
- reconcile worker success against truth-gate evidence
- return stable `reason_code` + `resume_action` on every block

### What it deliberately does not do

- auto-release / auto-merge / auto-deploy
- auto-repair broken project context
- read an executor's private database
- replace your knowledge base
- become a distributed cluster scheduler
- treat inbox notes as software truth

### Quick start

```bash
python3 -m pip install -e '.[test]'
cp config/projects.example.json config/projects.json
# edit repo paths in config/projects.json

bin/adb projects list --json
bin/adb doctor --project demo-platform --json
bin/adb dispatch --project demo-platform --stage plan --feature example --dry-run --json
```

Restricted stage flow:

```bash
bin/adb approve --actor you --project demo-app --stage implement --feature example --json
bin/adb dispatch --project demo-app --stage implement --feature example --approval-token <token> --json
bin/adb reconcile <dispatch-id> --json
```

### Configure adapters

```json
{
  "schema_version": "1.0",
  "adapters": {
    "executor": "hermes",
    "truth_gate": "beacon"
  },
  "projects": []
}
```

Implement your own backends against:

- `agent_delivery_bus.adapters.spi.ExecutorAdapter`
- `agent_delivery_bus.adapters.spi.TruthGateAdapter`

### Knowledge OS boundary

If you keep a second brain, use four working folders **outside** ADB:

1. **Projects** — active goals, progress, decisions, todos
2. **Knowledge assets** — refined methods, cases, templates
3. **Inspiration inbox** — raw captures
4. **Collaboration rules** — how AI must cooperate

ADB is only the governed handoff from “approved project work” to “executor + evidence”.

See `skills/collaboration-rules-template/`.

### Development

```bash
python3 -m pip install -e '.[test]'
python3 -m pytest -q
```

### Status

`v0.1.0` open-source extraction:

- core/control-plane stabilized
- Beacon/Hermes demoted to example adapters
- bilingual docs
- collaboration-rules skill template included

---

## 中文

### 为什么做这个

AI 写代码常见翻车点很固定：

- 猜错仓库
- 重试造成重复派工
- 把 worker 自报完成当成交付完成
- 没有协作规则时越改越乱

Agent Delivery Bus（ADB）是一个很薄的本地控制面，把四种权威拆开：

| 权威 | 归属 |
|------|------|
| 项目路由 | 注册表（`config/projects.json`） |
| 执行生命周期 | Executor 适配器（示例：Hermes Kanban） |
| 交付判定 | Truth-gate 适配器（示例：Beacon） |
| 审批 + 幂等 + 审计 | ADB SQLite 账本 |

知识不放在这里。个人知识库可以给 ADB 提供意图，但 ADB 不存灵感、方法论或笔记正文。

### 核心架构

```text
人 / 知识系统意图
            |
            v
   Agent Delivery Bus Core
 注册表 -> 预检 -> 审批 -> 幂等派工 -> 对账
     |        |               |          |
     |        +-- TruthGate   |          +-- 证据
     |        +-- Executor    v
     +-- projects.json   示例：Hermes
```

#### 开源形态

- **Core**：注册表、预检编排、一次性审批、SQLite 账本、派工状态机、对账
- **Adapter SPI**：`TruthGateAdapter` + `ExecutorAdapter`
- **示例适配器**：Beacon（真值门）、Hermes（执行器）
- **Skill**：`skills/agent-delivery-bus`
- **协作规则模板**：`skills/collaboration-rules-template`
- **运行时依赖**：无（Python 标准库 + SQLite）

### 能做什么

- 用 slug / alias / path 精确解析项目（禁止模糊猜测）
- 真实副作用前做只读严格预检
- `implement` / `freeze` 需要一次性、带 scope 的审批
- 创建可幂等的执行器任务
- 用真值门证据对账 worker 成功
- 任何阻断都返回稳定 `reason_code` + `resume_action`

### 明确不做

- 自动 release / merge / 部署
- 自动修复目标项目 context
- 读取执行器私有数据库
- 替代你的知识库
- 变成分布式集群调度器
- 把灵感箱内容直接写成软件 truth

### 快速开始

```bash
python3 -m pip install -e '.[test]'
cp config/projects.example.json config/projects.json
# 编辑 config/projects.json 中的仓库路径

bin/adb projects list --json
bin/adb doctor --project demo-platform --json
bin/adb dispatch --project demo-platform --stage plan --feature example --dry-run --json
```

受限阶段：

```bash
bin/adb approve --actor you --project demo-app --stage implement --feature example --json
bin/adb dispatch --project demo-app --stage implement --feature example --approval-token <token> --json
bin/adb reconcile <dispatch-id> --json
```

### 适配器配置

```json
{
  "schema_version": "1.0",
  "adapters": {
    "executor": "hermes",
    "truth_gate": "beacon"
  },
  "projects": []
}
```

自定义后端请实现：

- `agent_delivery_bus.adapters.spi.ExecutorAdapter`
- `agent_delivery_bus.adapters.spi.TruthGateAdapter`

### 知识库边界

如果你有第二大脑，请把这四个可执行文件夹放在 **ADB 之外**：

1. **项目**：正在推进的目标、进度、决策、待办
2. **知识资产**：已加工方法、案例、模板
3. **灵感收集**：未加工原料
4. **协作规则**：AI 如何配合你（最重要）

ADB 只负责把“已批准的项目工作”安全交接给“执行器 + 证据闭环”。

详见 `skills/collaboration-rules-template/`。

### 开发

```bash
python3 -m pip install -e '.[test]'
python3 -m pytest -q
```

### 状态

`v0.1.0` 开源抽取版：

- 控制面核心稳定
- Beacon/Hermes 降为示例适配器
- 中英双语说明
- 附带协作规则 skill 模板

### License

MIT
