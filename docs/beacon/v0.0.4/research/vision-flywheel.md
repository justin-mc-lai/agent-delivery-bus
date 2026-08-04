---
slug: vision-flywheel
version: v0.0.4
status: support_advisory
authority: beacon-gen-change
promotion_ref: "user:2026-08-04 愿景正式 feature 立项（truth 冻结）"
upstream: docs/vision-first-principles.md
---

# Research: vision-flywheel（个人 AI 生产飞轮）

## 结论先行

愿景（`docs/vision-first-principles.md`，2026-08-03 修订版）经源码级现状核查确认：
- **可信资产**：分发侧证据链（published-track engine_version 追溯）、ADB 治理全链（审批/幂等/证据）、beacon truth/QA/release、personal-brain + 日榜流水线雏形。
- **核心断点**：反馈回路缺失（指标全 0 / analyze-data 空壳）、调度心跳未激活（oss-pick-daily planned）、ADB 无调度层。
- **本包（v0.0.4）闭包**：ADB 侧「调度心跳 + quota + 心跳证据对账」——飞轮转起来所需的调度底座。
- **海级拆分**：反馈回路、选题池化、pi 执行器、多格式、平台补齐全部进 Phased Backlog / Deferral（不在本包 closure）。

## Lake vs Ocean

- `lake_or_ocean: 海`（三环飞轮为海级 scope）
- Round 0 拆分：本包只煮干 ADB 调度心跳层；其余环在 Phased Backlog 显式声明、不在单包宣称 closure。

## Source Refs

- 愿景源：`docs/vision-first-principles.md`
- 稳定性分析：`docs/beacon/research/beacon-longrun-stability-analysis-2026-08.md`（S4 预算、S7 并发缺口）
- 演进方案：`docs/beacon/research/beacon-longrun-pi-agent-evolution-2026-08.md`（loopx quota/should_run、kun 无进展退避）
- 参照包：`docs/beacon/v0.0.3/features/ops-digest-cron/`（cron_owner=hermes、不内嵌调度器的既定方向）

## 设计决策（本包闭包范围）

1. **不内嵌 cron daemon**（延续 ops-digest-cron 的 D 方向）：ADB 只做「调度条目注册 + should-run 判定 + 心跳记账」，实际 tick 由 hermes cron 触发（与 ops-digest-cron 同构）。
2. **quota 记账**（S4 缺口）：每次派发/心跳计 slot，quota 耗尽 → blocked/throttled；spend-after-validated-writeback（evidence 落盘后才计配额）。
3. **心跳证据对账**：每次心跳运行记录进 dispatch ledger（追加式事件），与 truth-gate closure 复用对账逻辑。
4. **should-run 确定性判定**（loopx 借鉴）：capability/workspace 门卫 + quota 门卫，无 LLM 判定。
