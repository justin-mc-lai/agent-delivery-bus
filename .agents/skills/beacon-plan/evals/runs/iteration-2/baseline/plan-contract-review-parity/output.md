根据 beacon-plan 的 `pln-review` 模式（`mode_id=pln-review`），按 "Underspec still emit" 规则，先输出结构化 contract，后附澄清问题。

---

# Planner Review Contract — OSS Full-Parity Port

## 1. 合约元信息

| 字段 | 值 |
|------|-----|
| `mode_id` | `pln-review` |
| `public_id` | `plan` |
| `execution_mode` | `single_process_multi_reviewer` |
| `fallback_reason` | 源 OSS 未指定，无法执行 subagent runtime；审阅基于通用 OSS 复刻框架先行 |
| `recommended_next_harness` | `stop` → 待澄清后路由至 `pln-program` 或 `goal` |
| `evidence_refs` | 无（源 OSS 未指定，无可引用源码） |

---

## 2. intent_snapshot

```
意图：将一款成熟 OSS 完整复刻到本项目，做到全能力对等（full parity），
     不做 MVP、不做渐进式裁剪。目标产物在功能、架构、API、运维、
     安全、文档等维度与源 OSS 达到同等成熟度。

触发语境：用户未指定源 OSS 名称，但明确要求"全、同等能力、不要 MVP"。
         这表明源 OSS 是已知的、有明确 feature set 的成熟项目，
         复刻目标是在本项目技术栈下重建等效系统。
```

**隐含假设（待验证）：**
- 目标技术栈已确定 / 或将在源 OSS 指定后确定
- 复刻范围包括所有核心模块 + 周边生态
- 许可证兼容，允许复刻
- 团队有能力消化源 OSS 的全部设计决策

---

## 3. scope_mode

```
scope_mode: full_parity (hard)

含义：
- 源 OSS 的所有 GA 功能必须在目标系统中具备等效实现
- Beta/experimental 功能需明确标记为 parity-tracked 或显式 defer
- 非功能性需求（性能、可扩展性、安全）对标，不低于源水平
- 文档、测试覆盖、运维工具链同步复刻
- 不允许以 "MVP 够用" 为由裁剪功能范围

边界：
- 源 OSS 已 deprecated 且将在下一大版本移除的模块 → 可标记 skip
- 源 OSS 特定于其技术栈/生态的集成 → 映射到目标栈等效方案
- 第三方插件/社区扩展 → 不纳入 core parity，列入 ecosystem 追踪
```

---

## 4. parity_matrix (blocked — awaiting source OSS identification)

```
状态：BLOCKED
原因：源 OSS 名称/版本未指定，无法构建具体的模块级对等矩阵。

矩阵结构预案（源 OSS 指定后即刻填充）：

┌──────────────────┬──────────┬──────────┬──────────┬──────┬──────┬────────┐
│ Module/Domain    │ Source   │ Target   │ Gap      │ Est. │ Risk │ Pri    │
│                  │ Cap      │ Status   │          │ (d)  │ (H/M/L)│ (P0-P3)│
├──────────────────┼──────────┼──────────┼──────────┼──────┼──────┼────────┤
│ (待填充)          │          │          │          │      │      │        │
└──────────────────┴──────────┴──────────┴──────────┴──────┴──────┴────────┘

审查维度（每个维度独立评分）：
  A. 功能完整性    — 每个 feature 是否等效覆盖
  B. API 兼容性    — 接口签名、行为语义是否一致
  C. 数据模型      — Schema、迁移、存储层是否对等
  D. 架构对等      — 模块边界、数据流、扩展点
  E. 运维就绪      — 部署、监控、日志、备份、HA
  F. 安全合规      — 认证、授权、数据保护、审计
  G. 测试覆盖      — 单元/集成/E2E 覆盖率对标
  H. 文档          — 用户文档、API 文档、运维手册
  I. 生态/集成      — 插件系统、SDK、第三方对接
```

---

## 5. deferral_ledger (预登记 — 待源 OSS 指定后确认)

```
状态：PENDING
说明：以下为通用复刻场景中常见的可延迟项模板，
      具体条目需在源 OSS 指定后逐条确认/否决。

┌──────────────────────────────────────┬──────────┬──────────────────────┬──────────────┐
│ Item                                 │ Defer?   │ Rationale            │ Re-eval      │
│                                      │ (TBD)    │                      │ Trigger      │
├──────────────────────────────────────┼──────────┼──────────────────────┼──────────────┤
│ 历史数据迁移工具                       │ LIKELY   │ 新部署无历史数据      │ 有旧系统迁移   │
│                                      │ DEFER    │                      │ 需求时        │
├──────────────────────────────────────┼──────────┼──────────────────────┼──────────────┤
│ 已废弃 API/模块兼容层                  │ LIKELY   │ 无存量用户无需兼容     │ 需要对接旧     │
│                                      │ DEFER    │                      │ 客户端时      │
├──────────────────────────────────────┼──────────┼──────────────────────┼──────────────┤
│ 多语言 i18n（非核心语言）              │ TBD      │ 取决于目标用户群       │ 国际化需求     │
│                                      │          │                      │ 确认时        │
├──────────────────────────────────────┼──────────┼──────────────────────┼──────────────┤
│ 特定云平台部署模板（非目标平台）         │ LIKELY   │ 仅支持目标部署平台     │ 多云战略       │
│                                      │ DEFER    │                      │ 确认时        │
├──────────────────────────────────────┼──────────┼──────────────────────┼──────────────┤
│ 社区贡献指南/治理模型                   │ LIKELY   │ 内部项目暂不需要       │ 计划开源时     │
│                                      │ DEFER    │                      │              │
├──────────────────────────────────────┼──────────┼──────────────────────┼──────────────┤
│ 性能基准测试套件（对标源 OSS）          │ LIKELY   │ 可在功能完成后补充     │ 功能验收后     │
│                                      │ DEFER    │                      │              │
└──────────────────────────────────────┴──────────┴──────────────────────┴──────────────┘
```

---

## 6. findings (P0 — 阻断性发现)

### F-001: 源 OSS 未指定 — BLOCKING
```
Severity: P0 — 阻断
Angle: 所有审查维度
Detail: 无法进行任何具体对等分析。所有模块级、API 级、数据模型级的
        parity review 均依赖源 OSS 的代码库/文档作为 ground truth。
Impact: parity_matrix 为空，deferral_ledger 为模板，无法估算工期和资源。
Action: 用户必须指定源 OSS 名称 + 目标版本号（精确到 minor version）。
```

### F-002: 目标项目技术栈未声明 — BLOCKING
```
Severity: P0 — 阻断
Angle: 架构对等 (D)、生态/集成 (I)
Detail: 复刻策略高度依赖技术栈选择：
        - 同栈复刻（如 Python→Python）：可直接移植，风险最低
        - 跨栈复刻（如 Python→Go/Rust/TS）：需要设计等效抽象，
          部分生态集成可能需要完全重新设计
        - 逆向复刻（仅根据文档/行为重建，不看源码）：法律风险 +
          行为差异风险极高
Impact: 无法评估架构复杂度、无法规划实现路径。
Action: 确认目标技术栈 + 复刻方式（同栈/跨栈/逆向）。
```

### F-003: 复刻方法未声明 — HIGH
```
Severity: HIGH
Angle: 架构对等 (D)、功能完整性 (A)
Detail: 三种常见方法，风险和成本差异巨大：
  a) Fork + 改造 — 从源 OSS fork，逐步替换/适配到目标技术栈
  b) 参照重写 — 以源 OSS 为 spec，用目标栈从头实现
  c) 功能复刻 — 仅按外部行为重建，不看源码（clean-room）
Impact: 方法选择影响工期、法律合规、行为兼容性。
Action: 确认复刻方法。
```

### F-004: 许可证/法律合规未评估 — HIGH
```
Severity: HIGH
Angle: 安全合规 (F)
Detail: 复刻 OSS 涉及知识产权边界：
        - Clean-room 复刻的法律要求
        - Copyleft 许可证（GPL/AGPL）的传染性
        - 专利条款
        - 商标使用限制
Impact: 法律风险可能导致项目不可行。
Action: 确认源 OSS 许可证类型 + 复刻合规策略。
```

### F-005: 无现有代码库分析 — MEDIUM
```
Severity: MEDIUM
Angle: 所有
Detail: 当前项目状态未知：是新项目还是已有代码库上叠加？
        如果是已有项目，现有架构是否支持全量复刻的扩展？
Impact: 影响工作量估算和架构决策。
Action: 确认目标项目当前状态（全新/已有代码库）。
```

---

## 7. recommended_route

```
当前状态：BLOCKED — 必须解决 P0 findings 后方可继续

推荐路径（待 P0 解除后）：

Step 1: pln-source
  对源 OSS 做深度 source-anchored research：
  - 模块/包清单 + 依赖图
  - 公开 API surface 提取
  - 数据模型 catalog
  - 配置项清单
  - 扩展点/插件架构

Step 2: pln-review (re-run)
  基于 Step 1 的 source analysis 填充：
  - 完整 parity_matrix（逐模块/逐 API）
  - 确认后的 deferral_ledger
  - 风险矩阵
  - 工期估算（t-shirt size → 人天）

Step 3: pln-program
  将 parity_matrix 转换为可执行 program：
  - Phase 拆解（foundation → core → advanced → ecosystem）
  - 依赖关系图
  - 里程碑定义
  - 资源需求

Step 4: goal (长程)
  如需端到端交付管线：plan → truth → design → implement → qa → release

                     pln-review (本次)
                          │
                    ┌─────┴─────┐
                    │  P0 解除后  │
                    └─────┬─────┘
                          │
                    pln-source
                          │
                    pln-review (re-run, 完整)
                          │
                    pln-program
                          │
                    goal (长程管线)
```

---

## 8. 澄清问题

以上 structured contract 已按要求先行输出。以下是解决 P0/P1 findings 的最小必要问题集：

### P0 — 必须回答
1. **源 OSS 是什么？** 请提供名称 + GitHub/GitLab 地址 + 目标复刻的版本号（如 v2.5.3）。
2. **目标技术栈是什么？** 语言、框架、数据库、部署平台。
3. **复刻方法？** Fork+改造 / 参照重写 / Clean-room 功能复刻？
4. **目标项目当前状态？** 全新项目 / 在已有代码库上叠加？如已有代码库，请提供路径。

### P1 — 强烈建议回答
5. **团队规模与 timeline 约束？** 几人团队？预期多久完成？
6. **目标用户/使用场景？** 内部工具 / SaaS 产品 / 开源项目？
7. **有无已识别的不可复刻模块？** 例如依赖特定硬件的功能、闭源第三方集成等。
8. **许可证合规策略？** 源 OSS 许可证是什么？是否需要法律审查？

---

**合约状态：AWAITING_INPUT** — 产出本文件不代表审查完成；P0 解除后方可进入实质 parity analysis。
