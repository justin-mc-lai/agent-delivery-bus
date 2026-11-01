---
slug: watchdog
version: v0.1.5
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v0.1.5/features/watchdog/truth.md
  tests: docs/beacon/v0.1.5/features/watchdog/tests.md
  tasks: docs/beacon/v0.1.5/features/watchdog/tasks.md
  evidence: docs/beacon/v0.1.5/features/watchdog/evidence.md
---

# Requirement Truth: watchdog

## 人话

前台会派活，但没人知道活是不是卡住了。
这个湖加一个"巡夜人"：每 10 分钟醒一次，检查三件事——
最近有没有实质动作、router 自己还活着吗、有没有连续卡住的活。
一切正常就输出 `NOTHING` 什么都不写；出事才留痕、才动手。
它必须活在 adb 进程**外面**，否则 adb 一死它也死了，等于没巡。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "我想要做完所有需求交付一个 superagent"
goal: "进程外的巡夜人按节奏判定有没有实质进展；无异常保持静默，异常才介入并留痕；adbd 可被系统托管常驻。"
non_goals: "不接 IM 信道；不替换 router 决策；不做多机分布式；不做 UI；不自动安装 launchd。"
constraints: "Go 放 ADB 仓库 cmd/adbd/；只生成 plist 由人安装；真相只读。"
scope_mode: "lake"
```

## 状态模型

```text
watch: idle -> scanning -> clean | alerted -> acted
alert: none -> stalled | router_stalled | repeat_stall
escalation: none -> warned -> escalated
```

- `scanning`：一次 `watchdog --once` 运行中。
- `clean`：无异常，输出 `NOTHING`，不写 trace。
- `alerted`：发现异常，写 trace。
- `acted`：在 `--apply` 下执行了动作（requeue / unbind / escalate）。

终态：`clean`、`acted`。

## 用户旅程

1. 系统托管每 10 分钟跑一次 `adbd watchdog --once`。
2. 正常时它只打印 `NOTHING`，日志干净，人不用看。
3. 某个 worker 20 分钟没有任何动作 → 输出 `stalled thread=<t>`，写一行 trace。
4. router 自己心跳过期（比如进程死了）→ 输出 `router_stalled`，写 trace。
5. 同一个 thread 连续三轮都卡 → 升级 `repeat_stall`，6 小时内不再重复告警。
6. 加 `--apply` 时，卡住的绑定会被解开并把消息放回慢路径队列，等前台重新决定。

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | goal：判"有没有实质进展" | truth.md#Acceptance-Criteria |
| AC-002 | 状态模型：router 也必须被看 | truth.md#Acceptance-Criteria |
| AC-003 | goal：无事保持静默 | truth.md#Acceptance-Criteria |
| AC-004 | goal：出事才留痕 | truth.md#Acceptance-Criteria |
| AC-005 | 状态模型：连续卡住升级 | truth.md#状态模型 |
| AC-006 | goal：必须在 adb 进程外 | truth.md#Acceptance-Criteria |
| AC-007 | constraints：不自动改系统 | truth.md#Acceptance-Criteria |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `adbd watchdog --once` 判定依据是「距该 thread 最近一次实质动作的时长」而非进程是否存活：最近动作来自 decisions.jsonl 的 `decided_at`、绑定更新、worker 汇报三者中最新的一项；超过 `adbd_stall_after`（默认 20m）输出 `stalled thread=<t>`；从未有过动作的 thread 按绑定创建时间算 |
| AC-002 | `adbd route` 每次 drain 后写 `.beacon/adbd-heartbeat.json`（字段 `at`）；watchdog 发现心跳早于 `now - adbd_heartbeat_after`（默认 10m）时输出 `router_stalled` 并写 trace；心跳文件不存在等同过期 |
| AC-003 | 一切正常时只输出 `NOTHING` 一行；不写 watchdog trace、不改任何状态文件（测试断言文件 mtime/内容不变） |
| AC-004 | 每次介入写一行 `.beacon/adbd-watchdog.jsonl`，字段固定：`kind` / `thread` / `reason_code` / `evidence` / `action` / `at`；`--apply` 时 `action` 记录实际执行的动作，未加 `--apply` 时为 `observed` |
| AC-005 | 同一 thread 连续 3 次扫描都 `stalled` → 该行 `kind=repeat_stall`、`reason_code=repeat_stall`；同一 thread 的重复告警 6 小时内只写一次（测试用注入时钟验证） |
| AC-006 | `adbd agent plist` 输出一份 launchd plist 到 stdout（含 StartInterval 与 WorkingDirectory），**不写任何文件、不调用 launchctl**；watchdog 判定逻辑不依赖 adbd 常驻进程（测试在无 adbd 进程时通过） |
| AC-007 | `adbd agent status` 只读报告：plist 路径是否存在、是否被 launchd 加载（通过 `launchctl print` 退出码判断，命令不可用时报 `launchctl_unavailable`）；不得修改系统状态 |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `adbd_stall_after` | `20m` | thread 多久没动作算卡住 |
| `adbd_heartbeat_after` | `10m` | router 心跳多久算过期 |
| `adbd_repeat_window` | `6h` | 同一 thread 重复告警的静默窗口 |
| `adbd_interval` | `10m` | 生成的 plist 的 StartInterval |

## 禁止的迁移（人工维护，内核不校验）

- illegal: acted -> clean（必须先落 trace 再回到 clean）
- illegal: escalated -> warned

## Non-goals（冻结边界）

- 不接 IM/飞书信道（M4）
- 不替换 router 的决策，不重新判断消息内容
- 不做多机/分布式 watch
- 不自动安装 launchd（只生成 plist）
- 不改 beacon 真相

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 巡夜人的节奏放 launchd 还是 paseo schedule？ | launchd 每 10 分钟跑 adbd watchdog --once；只生成 plist，安装由人执行 | answered | launchd StartInterval=600 调 adbd watchdog --once；adbd agent plist 只打印，安装由人执行。 | human |
