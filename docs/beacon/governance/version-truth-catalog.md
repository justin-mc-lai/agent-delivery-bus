---
schema_version: "1.0"
canonical: true
event_type: freeze-version-contract
project_slug: "agent-delivery-bus"
package_name: "agent-delivery-bus"
package_version: "0.1.4"
active_docs_line: "v0.1.4"
runtime_toolchain: "beacon"
runtime_version: "v1.6.12"
git_tag: "v0.1.4"
updated_at: "2026-08-17T00:00:00+00:00"
---

# Version Truth Catalog — agent-delivery-bus

> 本文件是 agent-delivery-bus 仓库的版本真值唯一来源（canonical）。
> 其余所有版本表面（pyproject、AGENTS.md、CLAUDE.md、onboarding state、git tag）
> 都只是本文件的单向投影。禁止散落手改。

## 真值轴

| Axis | Meaning | Canonical value |
|------|---------|-----------------|
| Package / Product | 本仓库作为产品的发布版本 | `0.1.4`（`pyproject.toml [project].version` 投影） |
| Project docs line | 本仓库 Beacon 交付树的最新行 | `v0.1.4`（`docs/beacon/v0.1.4/` 投影） |
| Runtime toolchain | 本仓库运行时依赖的 Beacon CLI | `v1.6.12`（工具链，不是产品版本） |
| Git tag | release 时的版本投影 | `v0.1.4` |

## 交付行

| Version | Status | Feature | Notes |
|---------|--------|---------|-------|
| v0.0.1 | archived | delivery-bus-mvp | initial public history |
| v0.0.2 | archived | neutral-scheduling | - |
| v0.0.3 | archived | worker-beacon-binding | - |
| v0.0.4 | archived | vision-flywheel | - |
| v0.0.5 | archived | vertical-gate | - |
| v0.0.6 | archived | neutral-scheduling | - |
| v0.0.7 | archived | workflow-lifecycle | - |
| v0.1.0 | released | pi-executor | release tag v0.1.0 |
| v0.1.1 | released | pi-curator | released on main |
| v0.1.2 | released | session-routing | released on main |
| v0.1.3 | released | channel-session-hardening | released on main |
| v0.1.4 | active | control-plane-hardening | current delivery line |

## 投影规则（单向）

| Surface | Source field | File(s) |
|---------|--------------|---------|
| package_version | `package_version` | `pyproject.toml` |
| docs target | `active_docs_line` | `AGENTS.md` / `CLAUDE.md` Beacon 块 |
| docs_version | `active_docs_line` | `.beacon/state/project-onboarding.json` |
| runtime_version | `runtime_version` | `AGENTS.md` / `CLAUDE.md` / onboarding state |
| git tag | `git_tag` | `git tag`（release 时创建） |

## 边界

- Beacon Runtime 版本（`v1.6.12`）是螺丝刀型号，不是本仓库的产品版本；不得把 runtime 版本投影为 package_version。
- 校验工具：`scripts/verify-version-alignment.py`（模块 `agent_delivery_bus.version_truth`）。
- 对本文件的结构性修改（新增交付行、升级 package_version）必须先过 truth 流程，再投影到其它表面。
