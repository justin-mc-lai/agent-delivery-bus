---
slug: control-plane-hardening
version: v0.1.4
status: frozen
language: zh
domain_required: true
ux_required: false
package_maturity: filled
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
materials_status: current
canonical_refs:
  prd: docs/beacon/v0.1.4/features/control-plane-hardening/truth.md
  user_story: docs/beacon/v0.1.4/features/control-plane-hardening/truth.md
  test_case: docs/beacon/v0.1.4/features/control-plane-hardening/tests.md
---

# Requirement Truth: control-plane-hardening (v0.1.4)

## 人话

把 ADB 控制面自己的地基补牢：① 版本只剩一个真值（catalog），其余表面自动对齐并可机检；
② SQLite 账本有版本迁移框架和可重复的本地备份；③ 执行器/解析器契约升级为显式能力声明，
不再靠异常回退猜能力；④ 补一个只跑测试、不碰发布流程的 CI。

- 能做：catalog 校验脚本；schema migration 回放 + 幂等；`adb backup` + 恢复文档；
  capabilities 协商；GitHub Actions 测试 CI。
- 不能做：release 自动放行；数据库业务行迁移；网络备份/多机同步。
- 怎样算完：四组 AC 全部有行为测试；校验脚本对四表面比对通过；legacy 库升级与
  备份 smoke 有证据；既有 211 测试全绿；release 人工门。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
scope_mode: lake
feature: control-plane-hardening
revision: R1
program: control-plane-hardening (v0.1.4)
depends_on:
  - channel-session-hardening (v0.1.3)
channels: local / feishu / weixin / line
release_gate: human always
```

## 用户旅程

1. 运营者新增一个交付行：改 catalog → 跑 `scripts/verify-version-alignment.py` → 各表面投影一致。
2. 升级旧 SQLite 账本：`Storage` 初始化自动按 schema_version 迁移，审计表留痕，幂等可重跑。
3. 运营者执行 `adb backup --dest <dir>`：账本 + 配置 + manifest 落地，文档写明恢复方式。
4. 开发者加新适配器：声明 `capabilities`，service 按能力传参；CI 全量测试把关。
5. 异常：catalog 与表面不一致 → 校验失败（reason_code）；旧库迁移中断 → 有据可查可修复。

## First principles

- 版本真值唯一 = 一写多读；catalog 是唯一提交源，其余表面只读投影。
- 存储演进 = 版本化迁移 + 幂等回放 + 审计；schema 变更不再是无版本补丁。
- 适配器契约 = 显式声明能力，调用方按能力协商；异常只用于异常，不用于协议协商。
- CI 边界 = 自动门只做验证，永不做发布；发布保留人工。

## Domain Model

| Entity | Key fields | Invariant |
|--------|------------|-----------|
| VersionCatalog | package_version, active_docs_line, runtime_version, git_tag | 唯一提交源；各表面投影一致 |
| SchemaMigration | version, name, applied_at | 单调递增；幂等；审计可查 |
| BackupManifest | dest, sources, created_at | 账本在线一致性复制；恢复 = 反向复制 |
| AdapterCapabilities | task_skills, task_session | 声明即契约；未声明视为不支持 |

## Entity Precedence

| Entity | Order |
|--------|-------|
| VersionCatalog | 1 |
| SchemaMigration | 2 |
| AdapterCapabilities | 3 |
| BackupManifest | 4 |

## Domain FSM — SchemaVersion

| State | From | Guard |
|-------|------|-------|
| legacy_v0 | — | 旧库初次进入 |
| migrated_v2 | legacy_v0 | 迁移 v1..v2 顺序回放完成 |
| up_to_date | migrated_v2 / fresh | user_version == SCHEMA_VERSION |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| legacy_v0 | initialize() | 无 | migrated_v2 | 按序执行迁移 + 审计 + 更新 user_version |
| migrated_v2 | initialize() | user_version == target | up_to_date | 幂等跳过 |
| any | 跳版本迁移 | version gap > 0 | blocked | fail-closed（不得静默跳过） |

**终态：** up_to_date；release 仍为独立人工门。

### Legal walks

1. **W-CPH-01** fresh DB → user_version==SCHEMA_VERSION + 审计行 · TC-CPH-003
2. **W-CPH-02** legacy DB（缺 executor_board / channel_actor）→ 迁移后结构完整 + 幂等重跑 · TC-CPH-004
3. **W-CPH-03** catalog → 四表面一致（--check-tag release 后） · TC-CPH-001

## Illegal transitions

- 不经过 catalog 直接手改 pyproject / AGENTS / CLAUDE / onboarding · TC-CPH-ILL-001
- 跳过迁移版本号直接到最新 · TC-CPH-ILL-002
- 适配器未声明能力却要求注入 skills · TC-CPH-ILL-003
- CI 出现 release/publish 步骤 · TC-CPH-ILL-004

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-CPH-001 | version-truth-catalog.md 机器可解析（schema_version/package_version/active_docs_line/runtime_version/git_tag 字段齐全）；`version_truth.check_alignment()` 对 pyproject / AGENTS.md / CLAUDE.md（+ onboarding state 可选）比对通过；`scripts/verify-version-alignment.py` 退出码 0 |
| AC-CPH-002 | `verify-version-alignment.py --check-tag` 比对 git tag 与 catalog.git_tag；无 tag 时报 reason_code，有 tag 且不一致时报错 |
| AC-CPH-003 | Storage.initialize() 后 `PRAGMA user_version == SCHEMA_VERSION`；schema_migrations 审计表记录每次迁移（version/name/applied_at）；重复 initialize 不重复迁移 |
| AC-CPH-004 | legacy 库升级：dispatches 的 hermes_board→executor_board、approvals 补 channel_actor、boundary_proposals 旧列补齐；升级后结构与 fresh 库一致；幂等可重跑 |
| AC-CPH-005 | `adb backup` 生成 manifest；SQLite 账本用 sqlite backup API 在线一致性复制；projects.json/projects.local.json 复制到目标；缺失源文件不静默成功 |
| AC-CPH-006 | 每个 ExecutorAdapter（null/hermes/pi）声明 capabilities（task_skills/task_session）；service.dispatch 按能力传参；自定义 resolver 按签名调用（无 capability → 兼容 (project) 调用，有 capability → 全量签名）；源码无 `except TypeError` 回退链 |
| AC-CPH-007 | `.github/workflows/ci.yml` 存在且只含 test 步骤（matrix pytest + 版本校验）；无 release/publish/deploy job |
| AC-CPH-008 | 兼容回归：既有 211 测试全绿；新增验收测试覆盖 AC-CPH-001..007 |

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-CPH-001 | covered |
| INT-002 | user | must | AC-CPH-002 | covered |
| INT-003 | user | must | AC-CPH-003 | covered |
| INT-004 | user | must | AC-CPH-004 | covered |
| INT-005 | user | must | AC-CPH-005 | covered |
| INT-006 | user | must | AC-CPH-006 | covered |
| INT-007 | user | must | AC-CPH-007 | covered |
| INT-008 | user | must | AC-CPH-008 | covered |

## Non-goals

- 不做 release 自动化；不做 multi-host 备份；不做业务数据迁移；不改变外部适配器调用行为。

## Freeze readiness

- [x] Intent Matrix must 无 missing
- [x] 用户旅程无待补文案
- [x] First principles / Domain Model 已具体化
- [x] 业务 FSM 五元组 + illegal
- [x] 每 AC ≥ 1 TC（Command+Assertion）+ exec-layer TC
- [x] tasks 均有 ac= + evidence= 绑定
- [x] package_maturity: filled
- [ ] `beacon truth review` pass 后再 freeze
