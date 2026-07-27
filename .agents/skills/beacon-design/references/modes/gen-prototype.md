# Mode: gen-prototype

> Archived from `beacon-gen-prototype` during 1+6 merge. This is progressive disclosure content for `beacon-design`.

# Beacon Generator Prototype Binding

This skill routes prototype work through Beacon's `prototype` surface.

For `v1.4.9+`, Prototype is a design-result adapter, not a UI/UX execution or design generation surface.
For `v1.4.8` and older versions, legacy requirement-driven wireframe freeze bundles may still be generated, reviewed, frozen, or inspected for compatibility.

## Use for

- accepted `ui-ux-dev` result intake
- design-result adapter binding
- PRD / user-story / test-case truth binding diagnostics
- page intent and state visibility contract freezing
- interaction handoff mapping before implementation
- historical prototype bundle status/archive/release compatibility
- legacy `design_context` / `design_reasoning` / `ux_contract` inspection when old materials exist
- legacy `DESIGN.md` / `preview-dark.html` preview asset inspection when old materials exist
- requirement-to-screen-and-UX handoff before implementation

## Operator rule

Prefer:

- `beacon prototype adapt ...` after an accepted `ui-ux-dev` result exists
- `beacon prototype status ...` to inspect adapter binding or legacy bundle status
- `ui-ux-dev` for exploration, system recommendation, review, polish, extract, and library lookup

For `v1.4.9+`, do not use Beacon prototype to generate wireframes or visual direction.
Route UI/UX execution to `ui-ux-dev`, then write the accepted result back through `beacon prototype adapt`.

Keep `prototype` inside the requirement governance boundary:

- it adapts accepted design output into Beacon truth bindings
- it is not final visual approval
- it is not production UI generation
- it is not a generic design playground
- it is required only when accepted UI/UX output affects screen intent, state visibility, interaction handoff, PRD promise, user-story acceptance, or test-case coverage

Expected adapter truth should make screen intent, state visibility, interaction handoff, and PRD/user-story/test-case bindings auditable.

Legacy design anchors that may still appear in historical requirement packs or compatibility tests:

- `apple-inspired-design-md`
- `ui-ux-pro-max-skill`
- `awesome-design-md`

## Examples

- 适用例：
  - “ui-ux-dev 的结果已经接受，需要把 screen intent、状态可见性和 handoff 绑定回 Beacon truth。”
  - “实现前需要证明这个设计结果映射到了 PRD、user-story、test-case。”
  - “我要检查旧版本 prototype bundle 的状态或 release history。”
- 不适用例：
  - “这是纯后端契约/治理 feature，没有真实 UX/UI surface。” -> 不应强行进入 `prototype`
  - “我想探索 UI 方向、做视觉 polish、找组件库。” -> 应去 `ui-ux-dev`
  - “我想直接做最终高保真视觉稿或生产页面。” -> 这不是 `prototype` 的职责

## Cold-start anchors

- `prototype` 在 `v1.4.9+` 解决的是 accepted design result 到 requirement truth 的绑定。
- UI/UX execution belongs to `ui-ux-dev`。
- 一旦进入 `prototype adapt`，重点是让交互、状态、页面意图和 truth bindings 可审计，而不是生成展示效果。


## Beacon v1.6.0 共享 Preamble

1. 先判断湖还是海：湖要煮干；海要拆分、标超纲或延后。
2. 先搜再造：推理前先读取 resolver 选中的 truth、source、evidence 和相关 memory。
3. 用户主权：Beacon 推荐路由；是否接受范围或把 queue item 升级为 truth/change 由用户决定。
4. 不假交付：placeholder implementation、docs-only completion、fake runner、zero assertions 或 placeholder evidence 不能算闭环。
5. Harness 边界：planner 不实现；generator 不裁决自身完成；evaluator 不改写 truth；governor 不成为主生命周期阶段。

HARD GATE:
你正在运行 generator skill。
禁止自证完成，禁止给 QA/release verdict，禁止把 placeholder/docs-only/fake-runner/zero-assertion/placeholder-evidence 当成交付闭环。
只能在 resolver-selected truth、用户已接受 scope 和本 skill authority 内写 truth 或 delivery artifact。
如需判断是否通过，必须路由到 evaluator。

## v1.6.0 Harness Migration

- Harness：`generator`。
- 来源迁移：`beacon-prototype` -> `beacon-gen-prototype`。
- 主要作用：已接受 UI/UX 原型结果绑定。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`generator`。
- 来源迁移：`beacon-prototype` -> `beacon-gen-prototype`。
- 主要作用：已接受 UI/UX 原型结果绑定。
- 默认语言：中文为主；英文只用于稳定术语、路径、命令或协议标识。

## 边界

- Planner 只产出 research/planning artifact 和 route recommendation，不写 truth、implementation、QA verdict 或 release verdict。
- Generator 只在 resolver-selected truth、用户已接受 scope 和本 skill authority 内写 truth 或 delivery artifact，不自证完成。
- Evaluator 只产出 evidence verdict、finding、reason code 和 route recommendation，不改 truth、不修 implementation。
- Governor 只维护 context、metadata、archive、hooks、automation、status 和 diagnostic support，不成为主生命周期 stage。

## 路由

- 粗提示词先归约为 outcome、truth/source/evidence refs、湖/海、truth_gap、test_gap、implementation_risk、verification_risk 和 recommended_route。
- `docs/beacon/<version>/research/<feature-slug>.md` 是 planner `support_advisory` artifact；没有用户确认和 `promotion_ref`，不能升级为 requirement truth。
- 需要跨 harness 时，停止当前动作，输出 route recommendation，并等待用户确认。
