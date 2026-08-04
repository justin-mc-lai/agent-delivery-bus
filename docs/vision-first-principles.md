# 核心诉求与第一性原理：个人 AI 生产飞轮

**日期**：2026-08-03（初版）
**修订**：2026-08-03（源码级现状核查后修订 —— 全部断言经项目源码/配置/数据库核实）
**背景**：基于对 agent-delivery-bus（调度分配）、beacon（交付真相）、personal-brain（个人知识库）、selfmedia-creator / selfmedia-sync-ai（自媒体矩阵）、kun / loopx / oh-my-pi（长程 loop 机制）的整体分析，从第一性原理整理个人核心诉求。

---

## 1. 一句话诉求

> **构建一个以个人知识库为唯一输入源、以自媒体矩阵为输出面、以软件交付为能力杠杆、由 agent 长程自治执行的个人生产飞轮——人类只保留三个决策点：选题拍板、审批放行、发布确认。**

## 2. 第一性原理（三个不变式）

### 不变式 1：时间是唯一稀缺资源，人是唯一决策单元
- 个人产出的上限 = 决策质量 × 执行吞吐。AI 不替代判断，只替代执行。
- 推论：**一切可确定的执行都应自动化；一切不可确定的判断都应保留给人**。系统的价值 = 把执行从人身上剥离的程度，同时不破坏判断的质量。

### 不变式 2：知识是复利资产，内容与产品是知识的外化
- 个人知识库（Obsidian + gbrain）是**唯一 truth 源**：选题来自知识、创作表达知识、数据反馈更新知识。
- 推论：知识环必须是闭环——沉淀 → 选题 → 产出 → 反馈 → 再沉淀。**断环即断流**（选题拍脑袋、反馈不回流 = 系统死亡）。
- **现状核查警示**：本飞轮当前的最大断点正是**反馈环节**（详见 §5.3），必须作为第一优先级风险对待。

### 不变式 3：长程不是"一直跑"，而是"每个环节都可验证、可恢复、可交接"
- 长程失败的模式不是"跑不动"，而是"跑飞"：成本失控、目标漂移、假完成、状态丢失。
- 推论：长程稳定性的本质 = **证据制（每步有证据）+ 预算制（每步有上限）+ 状态制（每步可恢复）**。

## 3. 全景架构：三环一总线（附现状标注）

```text
┌────────────── 知识环（输入） ──────────────┐
│ personal-brain (Obsidian + gbrain) ✅      │
│   选题调研：日榜流水线 ✅（雏形）             │
│   选题池 + 证据化：❌（缺口）                │
└────────────────────┬───────────────────────┘
                     │ 选题拍板①（池级确认）
                     ▼
┌────────────── 内容环（输出） ──────────────┐
│ 创作链：母版→humanize→render ✅（微信走通） │
│ 分发：A轨 skill/B轨 API + published-track ✅ │
│ 多格式：vlog/漫剧 ⚠️（仅 scaffold）          │
│ 反馈回路：❌（指标全 0，analyze-data 空壳）   │
└────────────────────┬───────────────────────┘
                     │ 发布确认③（release gate）
                     ▼
┌────────────── 产品环（杠杆） ──────────────┐
│ beacon truth→QA→release ✅                 │
│ pi agent 执行器：❌（待接入）                │
└────────────────────┬───────────────────────┘
                     │ 审批放行②（approval token）
                     ▼
┌────────── Agent Delivery Bus（中央调度） ──────────┐
│ 治理：审批/幂等账本/证据对账 ✅（ledger 空，骨架可用） │
│ 调度：❌（creator 侧有 planned cron 未激活）         │
│ 执行器：hermes ✅ · pi agent ❌                    │
└─────────────────────────────────────────────────────┘
```

## 4. 从第一性原理推导的六条核心诉求

### 诉求 1：知识库是唯一输入源，选题调研从知识库出发
- **推导**（不变式 2）：内容断环即死。选题是知识库中"尚未被表达"或"值得被表达"的资产的检索结果。
- **现状核查（已修正）**：
  - ✅ personal-brain 真实可用（bases/companies/concepts/daily/ideas/… 结构完整，gbrain 查询面已装）。
  - ✅ 已有雏形而非空白：GitHub Trending 日榜流水线（`fetch-github-trending.sh` 10:00 playwright-cli 抓取 → star 差分 → oss-sync enrich → **`brain-writeback.sh` 写回 personal-brain**）、选题类 skill（selfmedia-topic-radar / selfmedia-last30days）。
  - ❌ 缺口收窄为：**选题池化与证据化**（当前产出是笔记/列表，不是带证据字段——知识来源 + 市场信号 + 状态——的可调度选题池）。
- **人拍板①**：确认选题"池"方向，非逐篇确认。

### 诉求 2：创作是多格式矩阵，从走通的单一主链逐步扩展
- **推导**：同一知识资产应外化为多格式（图文/blog、vlog、漫剧），共享同一素材库与证据链。
- **现状核查（已修正——此前表述过于乐观）**：
  - ✅ **主链已真实走通**：母版（MASTER-SPEC v1 + meta.yaml）→ humanize 链（read-repo → 叙事 → 视觉 enrich → humanizer-zh）→ render（wenyan 微信 HTML）→ 发布（A 轨 Wechatsync / B 轨 publish-api）→ **published-track 回执**（记录 `publish_engine=fork-selfmedia-sync-ai engine_version=<commit>` + media_id/appmsgid，commit 级追溯）。微信文章与贴图草稿（type=77，未提交改动）已闭环。
  - ✅ 双轨发布引擎已定：`content/publish-backends.yaml`，default=fork-selfmedia-sync-ai（9528），降级 legacy-wechatsync（9527）。
  - ⚠️ **多格式是 scaffold 不是能力**：OpenMontage 漫剧仅 v0.0.4 draft 登记（生成/分发解耦已设计，60s 试片在案，全平台视频矩阵在 backlog）；B 站视频、抖音、头条 publish 均为 stub/占位（toutiao 靠 legacy 草稿同步兜底）。
  - ❌ **多格式扩展的前置缺口：素材库分层**（一篇一库已设计，但跨格式复用层未建）。
- **约束**：每个发布产物必须带回执证据（postId/media_id/engine_version），复用 ADB 幂等与证据机制。

### 诉求 3：调度分配是总线，不是队列
- **推导**（不变式 1）：调度器在正确的时间、用正确的执行器、把正确的工作投给正确的项目，保证幂等与证据闭环。
- **现状核查**：
  - ✅ ADB 骨架正确且**设计意图已含三环**：`config/projects.local.json` 的 `knowledge_source: { slug: personal-brain }` + `truth_gate: beacon` + `executor: hermes`，6 个真实项目已登记（beacon/selfmedia-sync-ai/selfmedia-creator/shopxo_canada/rolo/tool-station-network）。
  - ✅ 治理闭环完整：intent → preflight（truth 上下文 + executor 可用性）→ 一次性审批 token（FSM issued→reserved→consumed）→ 幂等投递（idempotency key + dispatch FSM）→ 证据对账（truth-gate closure，缺失证据保持 reconciling）。
  - ❌ 缺口（源码确认）：无调度器、无定时、无并发/quota、投递后不管（依赖 hermes 在线，无自愈）、release 硬禁、ledger 为空（骨架状态）、goal 阶段显式 deferred。
- **补法**：ADB 保留治理，调度层采用 loopx scheduler-hint 契约（quota 决策 → 宿主 cadence → rrule 唤醒 + ack），执行器抽象扩展 pi agent（不改变 hermes 优先）。

### 诉求 4：定时任务是飞轮"心跳"，激活已设计的而非从零造
- **推导**：飞轮可持续的前提是周期性自动运转。
- **现状核查（已修正——此前表述"无定时"不准确）**：
  - ✅ **已有设计未激活**：`content/pipelines/oss-pick-daily.yaml`（`status: planned`，`schedule: "0 10 * * *"`，四端平台）；`scripts/daily-trending.sh` 日榜；ADB 侧 `ops-digest-cron` feature（v0.0.3 已立项：truth/tests/tasks 存在，实现未做）；creator HEARTBEAT.md 明示"当前无定时任务"。
  - ❌ 缺的是**激活 + 统一调度层**：让 planned cron 真正被调度器执行（而非人的日历提醒），并把所有心跳（日榜、digest、回流、排期）收口到同一 quota/evidence 记账下。
- **补法**：三层心跳——① 日常轮询类（kun GraphScheduler tick 式 setInterval + unref）；② 跨宿主唤醒类（loopx rrule 改写 + ack）；③ 一次性长任务类（pi AsyncJobManager）。全部共用 ADB quota/evidence，防无人值守跑飞。

### 诉求 5：长程稳定性 = 证据 + 预算 + 状态（缺一不可）
- **推导**（不变式 3）：三环任何一环的长程任务都必须满足证据制、预算制、状态制。
- **现状核查**：
  - ✅ 证据制在**分发侧已落地**（published-track engine_version 追溯）与**交付侧已成熟**（beacon verifier / truth-gate closure）——这是飞轮最可信的资产。
  - ❌ 预算制全缺（ADB 无 quota、无并发上限）；状态制中 beacon 有 checkpoint/resume，但创作链（publish 中途断）与知识链（回流中断）无恢复机制。
- **落地**：按 `beacon-longrun-stability-analysis-2026-08.md` 的 P0 项（quota 层、spend-after-validated-writeback、Δ 评测口径）先行，P1 项（事件溯源强化、硬 lease）随后；创作链发布中断恢复优先补。

### 诉求 6：人的决策点只有三个，且必须"批而不做"
- **推导**（不变式 1）：人的注意力全部花在判断上。
- **三个决策点**：① 选题拍板（池级）；② 审批放行（ADB approval token，一次性，受限阶段）；③ 发布确认（release gate，内容与产品共用）。
- **现状核查**：审批与发布门已实现（ADB token FSM + beacon release gate）；选题池级拍板尚未成型（无池可拍）。
- **约束**：系统绝不绕过这三扇门（fail-closed）；也绝不把其他环节的决策推给人。

## 5. 现状核查结论：飞轮断点矩阵

### 5.1 环状态总览

| 环 | 真实状态 | 一句话 |
|---|---|---|
| 知识环 | ✅ 基座真实，选题有雏形 | 缺选题池化与证据化 |
| 内容环 | ⚠️ 主链走通，反馈断链 | 发布→追踪✅；**追踪→回流❌** |
| 产品环 | ✅ beacon 全链成熟 | 缺 pi agent 执行器接入 |
| 调度总线 | ⚠️ 骨架可用，调度空白 | 治理✅；调度/定时/quota❌ |
| 长程稳定 | ⚠️ 部分落地 | 证据✅；预算/状态❌ |

### 5.2 关键确认（愿景可信资产）
1. **分发侧证据链已真实落地**：published-track 16 张平台表 + engine_version commit 级追溯 + media_id 回执——证据制的正确性已被验证。
2. **双轨引擎与降级设计已定**：9528 fork / 9527 legacy，default 已切换，防单点。
3. **ADB 设计意图已含三环**：knowledge_source=personal-brain 就在注册表 schema 里。
4. **微信主链完整闭环**（文章 + 贴图草稿 + 草稿清理脚本），可作为其余平台的样板。

### 5.3 核心风险（第一性原理直接判死刑的断点）
**反馈回路缺失 = 知识环断流**。目前：published-track 互动指标全为 0（update-metrics.sh 无人调用）、`analyze-data` skill 是空壳、sync-ai analytics 是产品遥测非内容数据。
- 后果：创作者不知道哪篇有效 → 选题池失去信号 → 知识库失去"什么值得沉淀"的反馈 → 飞轮退化为"每日制造"，而非"复利积累"。
- **这是比调度缺失更优先的问题**：没有反馈，飞轮转得再稳也是空转。

## 6. 资产映射与缺口清单（核查后修订）

| 环节 | 已有资产（真实） | 缺口（真实） |
|---|---|---|
| 知识环 | personal-brain、gbrain、agent-reach、日榜流水线 + brain-writeback.sh | 选题池化与证据化；反馈数据入库 |
| 内容环 | 母版→humanize→render→publish 主链、published-track（16 表）、双轨引擎 | **反馈回路（指标采集+分析+回流）**；素材库分层；多格式（vlog/漫剧）从 scaffold 到能力 |
| 产品环 | beacon truth/QA/release 全链 | pi agent 执行器（driver_pi） |
| 调度总线 | ADB 治理全链（审批/幂等/证据）、6 项目注册表、ops-digest-cron 立项 | 调度器、定时激活（oss-pick-daily planned→enabled）、quota、并发、多执行器 |
| 长程稳定 | beacon GoalRun、证据制（双端） | P0 quota、事件溯源、lease、Δ 评测、创作链中断恢复 |
| 平台能力 | 微信全链、掘金/知乎/头条 legacy 兜底 | toutiao publish、bilibili-video、douyin（stub→real） |

## 7. 落地路线（按飞轮闭环顺序，核查后修订）

1. **反馈回路先补（P0，优先级高于一切）**：published-track 指标采集激活（update-metrics.sh 上心跳）→ analyze-data 空壳补实现（至少微信/小红书/抖音三端）→ 回流 personal-brain。**没有反馈，飞轮空转**。
2. **心跳激活（P0）**：oss-pick-daily 从 planned → enabled；ADB 补调度层 + ops-digest-cron 落地；所有心跳收口同一 quota/evidence 记账。
3. **选题池化（P1）**：日榜/调研产出 → 带证据字段的选题池（知识来源 + 市场信号 + 状态），人池级拍板。
4. **闭环加固（P1）**：quota 层、spend-after-validated-writeback、创作链发布中断恢复。
5. **产品环接入（P1-P2）**：beacon truth → ADB 派发 → pi agent 执行（driver_pi 先接 CLI/worktree 模式跑通长程交付）。
6. **矩阵扩展（P2）**：素材库分层 → vlog → 漫剧（openmontage 从 scaffold 到能力）→ B 站视频/抖音/头条 publish 补齐。

## 8. 验收标准（飞轮是否转起来的判据，核查后修订）

- [ ] 一个选题从知识库被检索出来（带证据）到发布上线，**中间不需要人参与任何执行环节**
- [ ] **反馈回路真实运转**：发布 N 天后指标（阅读/互动）自动入库，且回流 personal-brain（闭环断裂可被自动检出）
- [ ] 三个决策点外，系统 30 天内没有向人提出"执行类"问题
- [ ] 任意环节中断（进程死/网络断），恢复后从 checkpoint 继续，无重复劳动
- [ ] 每个产出（调研报告/内容/功能）都有可审计的证据链（含 engine_version 级追溯）
- [ ] 任意任务有成本/次数上限，超限自动降级或提请人工，绝不静默跑飞
- [ ] 定时心跳（日榜/digest/回流）连续 30 天无人工介入稳定运转
