# Program manifest: workflow-lifecycle

**Version**: v0.0.7
**Feature slug**: workflow-lifecycle
**plan_mode**: auto
**scope_mode**: lake
**generated_at**: 2026-08-10

## Intent

ADB 作为通用调度内核，补齐"工作流生命周期"能力：

1. **beacon 生命周期 = 第一方能力**：plan / truth / implement / qa / freeze / goal 六阶段可派发，force-load 对应 beacon skill（beacon-plan/beacon-truth/beacon-implement/beacon-qa/beacon-goal）。
2. **渠道无关关键词表**：飞书/微信/Line 等渠道使用同一张规范映射（`adb intent keywords --json`），同句话 → 同一 envelope。
3. **第三方对标预设**：superpowers / openspec（skill 工作流形状，非 CLI 工具）。
4. **通用开源库适配（host-agent 模式）**：用户丢一个开源库 → `adb workflow ingest` 确定性盘点并产出分析请求 → 宿主 agent（承载 adb 的 LLM）回填响应 → 校验 → 草案 → 确认 → 安装 → 绑定。adb 自身不调外部 LLM。
5. **可调试可验收**：分析/校验/安装/派发全程 JSONL trace；`adb workflow trace/debug/replay`；`adb workflow verify` 验收探针；坏工作流 fail-closed。

## AC 汇总

| AC | 要点 |
|---|------|
| AC-WF-001 | beacon 六阶段可派发并 force-load；缺 skill fail-closed |
| AC-WF-002 | 渠道无关关键词表 + `adb intent keywords --json`；同句话同 envelope |
| AC-WF-003 | superpowers/openspec 预设可安装/绑定 |
| AC-WF-004 | 通用 ingest：盘点→request→宿主回填→校验→draft→confirm→install（记录 commit+trace_id） |
| AC-WF-005 | JSONL trace + trace/debug/replay 命令 |
| AC-WF-006 | `adb workflow verify` 验收探针 |
| AC-WF-007 | 绑定后 dispatch→reconcile 闭环；坏库/无锚点字段 fail-closed |
| AC-WF-008 | illegal：无确认安装、危险命令、无证据字段、伪造 trace |

## Route

- truth: `beacon truth init -f workflow-lifecycle -v v0.0.7` → fill → TRG → freeze R1
- implement: 关键词模块 / workflows ingest+trace+verify / CLI / worker_binding 六阶段
- qa: pytest + beacon qa run
- release: 人工门
