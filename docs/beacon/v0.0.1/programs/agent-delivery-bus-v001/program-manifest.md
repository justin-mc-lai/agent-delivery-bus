---
schema: beacon-program-manifest-v1
program_slug: agent-delivery-bus-v001
version: v0.0.1
plan_mode: interactive
plan_mode_reason: cli_flag
scope_mode: full_parity
lake_or_ocean: 海
status: planned
revision_id: R1
language: zh
feature_graph_ack: true
scope_ack_ref: user-confirmation-2026-07-27
---

# Program Manifest

## Program Intent

构建一个稳定的本地 Agent Delivery Bus：以 Hermes Kanban 作为持久化调度内核，
以 Beacon 作为需求与交付 gate，以 Personal Brain 作为知识来源；统一管理本地
Beacon 源码仓及其 managed 项目，提供项目注册、严格预检、作用域审批、幂等派工、
任务回执、对账和符合规范的 Hermes/Codex skill。

## P4 Feature Graph Ack

- ack: accepted
- ack_ref: `user-confirmation-2026-07-27`
- decision: 首个交付湖只包含 `delivery-bus-mvp`
- rationale: 先稳定控制面和契约，再扩展知识采集、定时任务、UI 和 release adapter

## Scope

- In: registry、doctor、approval、dispatch、Hermes adapter、reconcile、skill。
- Out: 自动 release、自动修复目标项目、Hermes/Beacon 内部数据库耦合、Web UI、
  定时调度、Orca 主调度器。

## Delivery Boundary

Hermes worker complete 不是 program complete。首湖必须先通过 feature truth、实现测试、
Beacon QA 证据和人工验收；本 program 不在 v0.0.1 自动调用 release。
