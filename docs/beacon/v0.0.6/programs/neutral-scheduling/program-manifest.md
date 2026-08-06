# Program manifest: neutral-scheduling

**Version**: `v0.0.6`
**Feature slug**: `neutral-scheduling`
**plan_mode**: `auto`（既有源码 + 已确认缺陷清单，无开放性产品问题）
**scope_mode**: `lake`
**generated_at**: 2026-08-06

## Intent

把 agent-delivery-bus（ADB）从"Beacon 专属调度器"修正为**中立的多 agent 通信调度组件**：
通过强规则接口（dispatch envelope + binding manifest + evidence spec）接收交付任务，
不限制 truth gate 用 Beacon 还是其他系统；Beacon 作为内置参考 profile 保留，可被其他项目/agent 替换。

## 缺陷确认清单（source-anchored）

1. **worker binding 硬编码 Beacon**
   - source: `src/agent_delivery_bus/worker_binding.py` `STAGE_BEACON_BINDING`、`resolve_worker_binding()`、`format_binding_section()`（固定输出 `### Beacon worker binding` 与 `beacon_skill`/`beacon_command`）
   - 后果: 任何非 Beacon 的 worker/truth 系统收到的派工单都要求执行 Beacon skill，无法中立。
2. **任务 body 缺 evidence 契约**
   - source: `src/agent_delivery_bus/service.py` `task_body()`；`src/agent_delivery_bus/adapters/beacon.py` `closure()`（证据路径仅存在于 adapter 内部）
   - 后果: worker 无法从派工单本身得知"证据写哪、要哪些文件、验收查什么"；closure 仅 glob 证据文件存在性，不绑定 dispatch_id，重试可能误用旧证据。
3. **适配器全局单选，不支持 per-project 路由**
   - source: `src/agent_delivery_bus/adapters/factory.py` `adapters_from_config()`（全局一对 executor/truth_gate）；`src/agent_delivery_bus/registry.py` `Project`（无 truth_gate/executor/binding_profile 字段）
   - 后果: 无法在同一个 ADB 中让不同项目/agent 使用不同 truth gate 或执行器，多 agent 协同被锁死。
4. **文档/契约头 beacon-first**
   - source: `skills/agent-delivery-bus/SKILL.md`（"Dispatch task bodies embed stage→Beacon skill binding"）、`docs/usage-playbook.md`（"内嵌 Beacon skill 指令（worker-beacon-binding 契约）"）、`docs/beacon-adb-role-division.md`（"ADB = beacon 的第一个/最核心的落地宿主"）
   - 后果: 开源叙事把 Beacon 写成唯一宿主，与中立调度组件的定位冲突。

## 交付目标（acceptance 口径）

- `worker_binding` 改为 profile 制：内置 `beacon` profile（保留现契约字段与 `### Beacon worker binding` 段，兼容 v0.0.3 消费者）；新增通用 profile 能力（由项目 registry 指定 skill/command/evidence_spec）。
- 派工单 body 增加 `evidence_spec`（证据目录、glob、closure 要求），schema_version 升级并保留解析兼容。
- `Project` 支持 per-project `truth_gate` / `executor` / `binding_profile`（默认回落到全局配置）；CLI 按项目解析适配器。
- closure 校验证据与 dispatch 的归属（evidence manifest 带 dispatch_id），避免跨次误判。
- 文档重述为"通用调度层 + Beacon 参考实现"；`docs/beacon-adb-role-division.md` 同步修订。
- 本机验收：`config/projects.json` 注册项目 + hermes executor 可真实 dispatch/reconcile；现有 pytest 全绿。

## Non-goals

- 不新增 pi/其他执行器后端（只保证接口可插拔）。
- 不改 dispatch/approval/schedule 的核心 FSM 语义。
- 不做自动 release（release 永远人工门）。

## Route

- P4 已由本 manifest 覆盖 → per-lake 进入 `beacon-gen-truth`：`beacon truth init -f neutral-scheduling -v v0.0.6` → fill → freeze。
- 实现：`beacon-implement`（worker binding / registry / factory / service / docs）。
- QA：`beacon-qa` + pytest + 本机 hermes 实跑。
