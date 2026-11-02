---
slug: metrics
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
  truth: docs/beacon/v0.1.5/features/metrics/truth.md
  tests: docs/beacon/v0.1.5/features/metrics/tests.md
  tasks: docs/beacon/v0.1.5/features/metrics/tasks.md
  evidence: docs/beacon/v0.1.5/features/metrics/evidence.md
---

# Requirement Truth: metrics

## 人话

系统说它变快了、变稳了，但没有任何数字。
这个湖不新增埋点：它直接读已经存在的两份流水（决策记录和执行回执），
算出四个指标。数据不够就直说不够，不编。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "顺序修复以上缺口、断点，不绕过不回避不拖延"
goal: "从现有 trace 聚合四个效果指标，让变快变稳能被数字验证。"
non_goals: "不新增埋点；不做可视化；不改 router；不接飞书。"
constraints: "Go 放 cmd/adbd（stdlib only）；只读现有文件；数据不足必须显式说明。"
scope_mode: "lake"
```

## 指标定义

| 指标 | 来源 | 算法 |
|------|------|------|
| `time_to_first_worker` | 慢路径队列 `recorded_at` + 决策 `decided_at` | 同一 message_id 的两者差值；输出 p50/p95 |
| `task_completion` | 回执 `dispatched_at` + `finished_at` | 差值；输出 p50/p95 |
| `decisions_per_thread` | 决策记录 | 决策数 / 去重 thread 数 |
| `fastpath_share` | 决策记录 | `reason_code` 属于 `deduped`/`bound_thread` 的条数 / 总条数 |

p95 用最近秩法（nearest-rank），样本数 < 5 时不输出该分位并标 `insufficient_data`。

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `adbd metrics --once` 读取 `adbd-decisions.jsonl`、`adbd-reports.jsonl` 与慢路径队列，输出四个指标；支持 `--since <RFC3339>` 与 `--outcome json|text` |
| AC-002 | `time_to_first_worker` 按 message_id 关联队列 `recorded_at` 与决策 `decided_at`，输出 `count`/`p50_ms`/`p95_ms`；样本不足 5 时 `p95_ms` 缺省且标 `insufficient_data` |
| AC-003 | `task_completion` 按回执的 `dispatched_at`/`finished_at` 计算，输出 `count`/`p50_ms`/`p95_ms`；无回执时 `count=0` 且不编造分位 |
| AC-004 | `decisions_per_thread` = 决策总条数 / 去重 thread 数（保留两位小数）；`fastpath_share` = `reason_code` 属于 `deduped`、`bound_thread` 两者之一的条数 / 总条数 |
| AC-005 | 文件缺失、文件为空时：对应指标 `count=0`，整体 `status=insufficient_data`，退出码 0；任何情况下不得输出未经计算的分位数字 |

## 配置项

无新增（沿用 `--root` 定位 `.beacon/`）。

## 禁止的迁移（人工维护，内核不校验）

- illegal: insufficient_data -> pass（样本不足不得标成就绪）

## Non-goals（冻结边界）

- 不做 Prometheus/外部指标导出
- 不做图表或 UI
- 不改决策与回执的写入
- 不接飞书

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | user utterance | truth.md#User-Intent-Snapshot |
| AC-002 | user utterance | truth.md#User-Intent-Snapshot |
| AC-003 | user utterance | truth.md#User-Intent-Snapshot |
| AC-004 | user utterance | truth.md#User-Intent-Snapshot |
| AC-005 | user utterance | truth.md#User-Intent-Snapshot |

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 指标从哪来？ | 直接聚合已有 decisions.jsonl / reports.jsonl / slowpath，不新增埋点 | answered | 聚合已有三份流水，不新增埋点。 | human |
