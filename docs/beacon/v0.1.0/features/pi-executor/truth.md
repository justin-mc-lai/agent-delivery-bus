---
slug: pi-executor
version: v0.1.0
status: draft
language: zh
domain_required: true
ux_required: false
package_maturity: filled
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
materials_status: current
canonical_refs:
  prd: docs/beacon/v0.1.0/features/pi-executor/truth.md
  user_story: docs/beacon/v0.1.0/features/pi-executor/truth.md
  test_case: docs/beacon/v0.1.0/features/pi-executor/tests.md
---

# Requirement Truth: pi-executor (v0.1.0)

## 人话

把 pi agent（oh-my-pi）接成 ADB 的**第二执行器**：ADB 通过 ExecutorAdapter SPI 把派工单交给 pi，pi 按绑定 skill 工作流执行并产出证据；同时把上一轮遗留的 **goal 阶段 closure 死角**补上——goal 派发后必须有带 dispatch_id 的 manifest 才能 reconcile 收口。项目可以声明 `executor=pi` 或阶段策略让长程任务走 pi、短任务继续走 hermes；pi 未安装或不可用时 preflight fail-closed，绝不静默回落。

- 能做：driver_pi 适配器（create_task/show_task/find_by_idempotency/preflight/skills_available）；goal closure 契约；per-project 执行器路由与阶段策略；本机 smoke（无 pi CLI 阻断、有 pi CLI dry-run→dispatch→reconcile 闭环）。
- 不能做：pi 自动审批/自动派发；pi 替代 beacon 判定；release 自动放行；pi 自己写需求 truth。
- 怎样算完：SPI 实现 + goal closure + 路由 + smoke 全部有行为测试；无 pi CLI 时 fail-closed；hermes 既有路径 145 测试保持全绿；QA 通过后 release 仍人工门。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖（Lake A）
language: zh
scope_mode: lake
feature: pi-executor
revision: R1
program: pi-agent (v0.1.x)
depends_on:
  - neutral-scheduling (v0.0.6)
  - workflow-lifecycle (v0.0.7)
first_party: beacon lifecycle（goal 阶段 closure 补齐）
executor: pi (driver_pi) + hermes（兼容并存）
knowledge_base: 不在本湖（pi-curator Lake B 承接）
release_gate: human always
```

## 用户旅程

1. 触发：项目登记 `executor=pi`（或阶段策略指定 goal/长程走 pi），用户对承载 adb 的 agent 说派发意图。
2. 关键操作：`adb intent parse` → 确认 → `adb dispatch --dry-run` → preflight（pi CLI 存在且健康）→ `adb dispatch` → pi 执行绑定 skill → 产出证据 manifest。
3. 结果：goal/长程任务通过 `BeaconAdapter.closure(stage="goal")` 校验 dispatch_id 后 completed；hermes 短任务路径不变。
4. 异常：pi CLI 缺失 → `pi_cli_unavailable` blocked；pi 任务失败 → executor_failed；goal manifest 缺失/不匹配 → evidence_ownership_mismatch，保持 reconciling。

## First principles

- 系统边界：ADB 核心只依赖 SPI；pi 是执行面（工人），不参与标准/判定/账本。
- 不可变：派工单必须带 evidence spec；closure 必须绑定 dispatch_id；release 永远人工门；hermes 兼容路径不得破坏。
- 可推翻假设：driver_pi 具体命令形态（`pi launch --agent` 等）可随 oh-my-pi CLI 演化，但 SPI 契约与证据绑定不可变。

## Domain Model

| Entity | Key fields | Invariant / requires |
|--------|------------|----------------------|
| PiExecutorAdapter | name=pi, cli_name, task_skill_args | 实现 ExecutorAdapter SPI；pi CLI 缺失 → preflight fail-closed |
| PiRunReceipt | task_id, board, idempotency_key, remote_status | 同 idempotency key 复用同一 task；不重复创建 |
| GoalClosureManifest | dispatch_id, stage=goal, feature, files[] | 必须存在且 dispatch_id 等于当前 dispatch |
| ExecutorRoutingPolicy | project.executor, stage→executor map | 项目显式 > 策略 > 全局默认 hermes；未知执行器 fail-closed |

## Entity Precedence

| Entity | Order | Requires |
|--------|-------|----------|
| ExecutorRoutingPolicy | 1 | registry 解析 |
| PiExecutorAdapter | 2 | pi CLI 可执行（或 fail-closed） |
| PiRunReceipt | 3 | create_task 回执含 task_id |
| GoalClosureManifest | 4 | 任务产出写入声明路径 |
| ClosureVerdict | 5 | manifest.dispatch_id == dispatch_id |

## Domain FSM — PiExecutorRun

| State | From | Guard |
|-------|------|-------|
| submitted | — | preflight pass（pi 就绪） |
| pi_running | submitted | create_task 回执 task_id |
| artifact_ready | pi_running | 远程状态 terminal success |
| evidence_pending | artifact_ready | 证据文件按 spec 存在 |
| closure_verified | evidence_pending | manifest.dispatch_id 匹配 |
| closure_failed | evidence_pending | 缺证据 / dispatch_id 不匹配 |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|-----|--------|
| submitted | create_task() | pi CLI 健康 + preflight pass | pi_running | 写 PiRunReceipt |
| pi_running | worker_reports(done) | 远程状态 terminal success | artifact_ready | 记录执行回执 |
| artifact_ready | closure(manifest) | 证据文件存在 | evidence_pending | 标记待 closure |
| evidence_pending | closure(manifest) | manifest.dispatch_id == dispatch_id | closure_verified | transition completed |
| evidence_pending | closure(manifest) | 缺证据 / 不匹配 | closure_failed | 保持 reconciling |

**终态：** closure_verified（任务级）；release 仍为独立人工门。

### Legal walks

1. **W-PI-01** submitted → pi_running → artifact_ready → evidence_pending → closure_verified · TC-PI-005
2. **W-PI-02** goal 派发 → BeaconAdapter.closure(goal) 校验 manifest → completed · TC-PI-003/TC-PI-005
3. **W-PI-03** hermes 兼容路径不受影响 · TC-PI-007

## Illegal transitions

- submitted → pi_running without pi CLI（静默回落默认执行器）· TC-PI-ILL-001
- artifact_ready → closure_verified with dispatch_id mismatch · TC-PI-ILL-002
- 任何阶段伪造/缺失 goal manifest · TC-PI-ILL-003
- pi 自动 approve/auto-dispatch · TC-PI-ILL-004
- release 自动放行 · TC-PI-ILL-005

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-PI-001 | `adapters/pi.py` 实现 ExecutorAdapter SPI（name=pi；preflight/board/workspace/ensure/create/show/find/skills_available）；pi CLI 缺失 → preflight `pi_cli_unavailable` fail-closed |
| AC-PI-002 | create_task 通过 pi CLI 创建任务，任务 body 含 binding manifest + evidence spec；同 idempotency key 幂等复用；回执含 task_id/board |
| AC-PI-003 | goal closure 契约：`BeaconAdapter.closure(stage="goal")` 校验 `<repo>/.beacon/state/goal/<feature>/manifest.json` 的 dispatch_id；缺失/不匹配 → evidence_ownership_mismatch，reconcile 保持 reconciling |
| AC-PI-004 | per-project 执行器路由：`executor=pi` 按项目解析；阶段→执行器策略（默认 hermes 兼容）；未知执行器 fail-closed |
| AC-PI-005 | 本机 smoke：无 pi CLI → dry-run blocked（pi_cli_unavailable）；有 pi CLI → dry-run→dispatch→reconcile 闭环（含 goal manifest 校验） |
| AC-PI-006 | illegal：pi 自动 approve/auto-dispatch 拒绝；heartbeat 不得自动派发 pi；release 永远人工门 |
| AC-PI-007 | 兼容性：hermes 默认路径与 AdapterResolver 全局默认不变；既有 145 测试保持全绿 |

## Intent Coverage Matrix

| intent_id | source | strength | landing | status |
|-----------|--------|----------|---------|--------|
| INT-001 | user | must | AC-PI-001 | covered |
| INT-002 | user | must | AC-PI-002 | covered |
| INT-003 | user | must | AC-PI-003 | covered |
| INT-004 | user | must | AC-PI-004 | covered |
| INT-005 | user | must | AC-PI-005 | covered |
| INT-006 | user | must | AC-PI-006 | covered |
| INT-007 | user | must | AC-PI-007 | covered |

## Non-goals

- 不实现 pi-curator（选题策展/知识库写回）——Lake B。
- 不实现渠道入站桥与审批 actor 身份映射。
- 不引入 pi 替代 beacon 判定或 release 门。
- 不做跨设备配置同步与 CI（维护线另行处理）。

## Freeze readiness

- [x] Intent Matrix must 无 missing
- [x] 用户旅程无待补文案
- [x] First principles / Domain Model 已具体化
- [x] 业务 FSM 五元组 + illegal
- [x] 每 AC ≥ 1 TC（Command+Assertion）+ exec-layer TC
- [x] tasks 均有 ac= + evidence= 绑定
- [x] package_maturity: filled
- [ ] `beacon truth review` pass 后再 freeze
