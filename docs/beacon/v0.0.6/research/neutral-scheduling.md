# Research: neutral-scheduling

**Status**: support_advisory（已确认缺陷清单，来源代码锚定）

## 缺陷确认（source-anchored）

1. worker binding 硬编码 Beacon：`src/agent_delivery_bus/worker_binding.py` `STAGE_BEACON_BINDING` / `resolve_worker_binding` / `format_binding_section`。
2. 任务 body 缺 evidence 契约：`src/agent_delivery_bus/service.py` `task_body()`；证据路径仅存在于 `adapters/beacon.py` `closure()`。
3. 适配器全局单选：`src/agent_delivery_bus/adapters/factory.py` `adapters_from_config()`；`registry.py` `Project` 无 per-project 路由字段。
4. 文档 beacon-first：`skills/agent-delivery-bus/SKILL.md`、`docs/usage-playbook.md`、`docs/beacon-adb-role-division.md`。

## 修复目标

见 `programs/neutral-scheduling/program-manifest.md` 与 `features/neutral-scheduling/truth.md`（AC-NS-001..007）。
