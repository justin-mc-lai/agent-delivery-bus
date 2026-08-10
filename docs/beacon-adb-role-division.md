# Research: Beacon × Agent Delivery Bus 作用对照（分工与边界）

**日期**：2026-08-05
**目的**：固化 Beacon 与 agent-delivery-bus（ADB）在个人 AI 生产飞轮中的定位、分工与协作契约，避免后续演进中职责漂移。
**来源**：三环一总线（vision-first-principles）、beacon-loop-capability-next-2026-08.md（loop 完善见解）、worker-beacon-binding truth（v0.0.3 冻结）。

---

## 1. 一句话定位

| 项目 | 一句话 | 角色比喻 |
|---|---|---|
| **Beacon** | 定义"什么是正确" + 判定"是否完成" | 立法 + 验收官 |
| **ADB** | 调度谁去干 + 把关谁批的 / 花多少 / 算不算完 | 监工 + 调度中心 |

## 2. 各自管什么

### Beacon（真相与验收层）

- **truth 冻结**：需求 → 人话（L0）+ FSM + AC + Intent Coverage Matrix → freeze。范例：vision-flywheel（v0.0.4）在 ADB 仓库立项冻结。
- **verifier 证据制**：`illegal: auto-complete without verifier`——没有证据不许说完成。
- **gate 链**：preflight → Truth Review Gate → QA → release（release 永远人工门）。
- **长程状态机**：GoalRun（created → running → stage_active → host_interrupted|stage_blocked → awaiting_verifier → completed|blocked|release_pending）。
- **核心资产**：truth 包（truth/tests/tasks/evidence）、FSM 五元组、Intent Coverage Matrix、human-readable L0、Δ vs Baseline 评测口径（研究）。

### Agent Delivery Bus（调度与治理层）

- **审批**：一次性 approval token（FSM issued→reserved→consumed，默认 TTL 900s），受限阶段（implement/freeze）强制。
- **幂等派发**：idempotency key（`adb-v1-<sha256(规范化请求)>`）+ dispatch FSM（draft→awaiting_approval→queued→dispatched→reconciling→completed）。
- **调度心跳**（v0.0.4 已实现运营）：`adb schedule`（register/list/show/should-run/ledger/cron-template）+ quota 记账（slot 制，spend-after-validated-writeback）+ heartbeat 事件流。
- **证据对账**：reconcile 复用 truth-gate closure，缺证据保持 reconciling。
- **核心资产**：projects registry（knowledge_source=knowledge-os / truth_gate=beacon / executor=hermes）、approval、dispatch ledger、schedule 命令族、quota ledger、boundary 治理。

## 3. 协作关系（谁是谁的插件）

```
beacon 冻结 truth（立法）
   ↓
ADB 派发 implement（监工：preflight 检查 beacon truth 上下文 + 审批）
   ↓
hermes/pi 工人干活（执行 beacon skill，worker-beacon-binding 内嵌指令）
   ↓
产出 evidence（.beacon/evidence/implement/...）
   ↓
ADB reconcile 用 beacon truth-gate 验收（closure 查证据）
   ↓
beacon QA/release（最终判定）
```

**具体耦合点（源码/契约级）**：

1. **truth_gate adapter = beacon（默认参考实现，非唯一）**：Beacon 是 TruthGate SPI 的默认参考 adapter（`adapters/beacon.py`），preflight 跑 `beacon doctor verify-context --strict` + 声明版本一致；验收时查 `<repo>/.beacon/evidence/implement/<feature>/*.json`。任何实现 TruthGate SPI 的系统都可替换。
2. **worker binding 是 profile 制，beacon 是内置参考 profile**：dispatch 任务 body 含 binding manifest（beacon profile 输出 `### Beacon worker binding` 段，stage → beacon plan/implement/qa 模板，兼容 worker-beacon-binding v0.0.3）+ evidence spec；项目可通过 registry 声明自定义 profile，ADB 不要求必须用 Beacon。
3. **两道门不互相替代**：ADB approval token（能不能派）+ beacon release gate（算不算发布）都过才算完。
4. **truth 包随项目走**：beacon 流程在任何仓库可跑（ADB 及各类内容/产品项目均有 `docs/beacon/`）；ADB 的 truth 包在 ADB 仓库 `docs/beacon/<version>/` 下，由 beacon 流程管理。

## 4. 分界线（一句话规则）

> **凡是"标准、判定、真相"进 beacon；凡是"派工、审批、配额、账本"进 ADB；凡是"执行、创作"进 hermes/pi。**

实例：

| 动作 | 归属 | 例子 |
|---|---|---|
| 选题该不该进待审 | Beacon（判定） | v0.0.5 VerticalGate（双层垂直画像，fail-closed） |
| 每天 9 点该不该跑、跑几次 | ADB（调度） | `adb schedule should-run`（quota + 健康门卫） |
| 跑出 5 条题推飞书 | 执行（hermes） | search-boundary-curate 心跳 |
| 题目算不算有效 | 人拍板 + Beacon 标准 | 待审 → approve/reject |

## 5. 飞轮中的总图景

- **beacon = 方法论 + 参考 truth gate**（在哪都能用：任何项目的 `docs/beacon/` 都是它的舞台）。
- **ADB = 通用调度/通信层**（强规则派发信封：binding profile + evidence spec + closure 契约；Beacon 是它的第一个参考宿主和内置 profile，但不是唯一依赖）。
- **hermes/pi = 工人**（执行层，不参与定义与判定）。
- **人 = 三扇门**（选题拍板 / 审批放行 / 发布确认），永不撤销。

两者不是竞争关系，是"标准"与"执行调度"的配合关系。

## 6. 后续分工预告（loop 完善路线）

| 下一步 | 落点 | 依据 |
|---|---|---|
| 反馈环（指标采集→analyze→回流） | 内容创作项目（beacon 定标准，ADB 提供心跳） | beacon-loop-capability-next 见解 2 |
| 心跳交接契约 + evidence_threshold 触发 | ADB（v0.0.6） | 见解 1 |
| hard lease、无进展退避、事件溯源 | ADB（v0.0.6-0.0.7） | 见解 3/4/5 |
| pi 执行器（driver_pi） | ADB（执行器抽象） | 见解 6 |
| loop 机制研究、Δ 评测口径 | Beacon（research） | 见解 6 |
| 新 truth 立项冻结 | Beacon 流程（包在对应项目仓库） | truth 生命周期 |

## 7. 结论

Beacon 管"对错"，ADB 管"流转"，hermes/pi 管"干活"，人管"拍板"。四条线职责清晰、契约固定（truth-gate / worker-binding / 双门制），任何一方的演进不得侵入对方职责——这正是飞轮能持续长程运转的组织前提。
