# Mode: eval-real-project

> Archived from `beacon-eval-real-project` during 1+6 merge. This is progressive disclosure content for `beacon-qa`.

# Beacon Evaluator Real Project

Use this skill to validate Beacon on a live project before rollout, after runtime upgrades, or when a user reports that Beacon works in-repo but not on a real repository.

Default to `current-version real-project release proof`.

Preferred public shortcut:

- `beacon real-project-validation --project-root <root> --version <version> --feature "<feature>" --json`

Only switch to specialized board acceptance when the capability itself is a cross-project board or projection surface.

## Required Inputs

- real project root
- docs version to validate
- one primary feature with existing PRD, user-story, and test-case materials

If the user only gives a versioned docs path such as `docs/beacon/v1.0.0`, infer the project root from that path and use the last path segment as the docs version.

## Safety Rules

- Treat the project worktree as user-owned and potentially dirty.
- Prefer read-only commands first.
- Do not run `prd create`, `user-story create`, `test-case create`, `implement plan`, or `implement run` on the live repo unless the user explicitly asks for write-path validation.
- If write-path validation is required, use an isolated copy or worktree instead of the user's main working tree.
- Treat `blocked`, `release_ready=false`, or failed release checks as valid command execution results until you confirm the CLI itself is wrong.
- Do not treat internal `.beacon/` artifacts as the primary operator surface.

Distinguish clearly:

- current-feature closure readiness
- version-line package closure / change-doc closure
- single-project release proof
- specialized board acceptance

## Validation protocol

Validate Beacon from the public human-facing surface inward.

- Resolve project root, docs line, and one primary feature.
- Preflight with `doctor` and current context health.
- Prefer the read-only public surface before any write-path validation.
- Treat version-aware blocked/not-ready results as meaningful outcomes until proven to be CLI defects.
- Use isolated worktrees for write-path or browser/session-heavy validation.

## Failure classification

Treat the result as a **project-state blocker**, not a Beacon defect, when:

- the command exits cleanly
- the payload is structured and version-aware
- the command reports real project issues such as QA blocked, release not ready, or missing evidence
- the feature is release-ready but the version line still lacks change docs / package closure
- browser QA is formally configured but the project lacks local runtime, runner configuration, or session/profile prerequisites
- `prototype`, `archive`, or `gate` returns a structured not-ready / warn / blocked verdict tied to real project truth

Treat the result as a **Beacon CLI defect** when:

- the command crashes
- the command ignores the requested version
- the command selects the wrong feature run
- the command requires undocumented arguments to function
- a read-only validation command unexpectedly mutates tracked docs or implementation files
- the public human surface contradicts newer authoritative feature scorecard state, for example:
  - `help` still points to repair after current-feature QA is release-ready
  - `status` / `release scorecard` aggregate older fields and miss the authoritative feature scorecard outcome
- browser QA formal layers are selected, but Beacon silently falls back to unrelated default layers instead of executing the declared browser-capable matrix
- the current version's published release note / summary / skill surface says a feature shipped, but the real-project validation path has no way to prove it on the public surface
- specialized board acceptance is presented as if it were sufficient proof of current-version release readiness

## Repair rule

When a real CLI defect is found:

1. Reproduce it against the real project.
2. Trace the Beacon source command responsible.
3. Make the smallest correct fix in the Beacon repo.
4. Add a regression test.
5. Run the targeted Beacon tests.
6. Re-run the failing command against the same real project and version.

Do not claim success until the same real-project command returns the corrected result.

## Optional write-path validation

Only if the user explicitly wants full write-path validation:

- create an isolated copy or worktree of the target project
- validate the minimal write path there
- confirm generated artifacts land under the intended `docs/beacon/<version>/` line

When browser QA/session/profile validation is required:

- prefer isolated worktree execution
- verify that browser-capable QA layers actually have a usable session or runner admission path

## Report results

Always report:

- which commands were executed
- which surfaces were operational
- which failures were real project-state blockers
- which failures were Beacon defects
- what Beacon source changes and regression tests were added, if any
- whether the validation touched the real project worktree or only read from it

## Multi-Project Board Acceptance

When the target feature is a cross-project board / projection capability, extend the validation path from “single real repo command proof” to “multi-project board acceptance”.

This is a specialized path.

It is:

- valid for board / projection acceptance
- useful for compatibility / portfolio views

It is not:

- the default current-version release proof path
- sufficient by itself to claim the latest Beacon line is fully validated on a real project

Use the assets under this skill directory:

- `references/eval-real-project/command-matrix.md`
- `references/eval-real-project/board-acceptance-record-template.md`
- `scripts/run_real_project_board_validation.sh`

This board-acceptance path is still read-only unless the user explicitly asks for write-path validation in an isolated worktree.

## Output Standard

Conclude with a compact validation summary:

- validated project root
- validated docs version
- command surfaces that passed
- command surfaces that returned meaningful blocked/not-ready states
- command surfaces that exposed real Beacon defects
- whether current-feature closure is ready
- whether version-line package/change-doc closure is still open
- whether the run covered:
  - current-version release proof
  - specialized board acceptance
- follow-up recommendation:
  - `safe to use on this project`
  - `safe for read-only use only`
  - `blocked until Beacon defect is fixed`

## Examples

- 适用例：
  - “Beacon 在自己仓里能跑，但我想证明它在真实项目里也成立。”
  - “我需要区分这是 CLI 缺陷，还是目标项目状态本来就没 ready。”
- 不适用例：
  - “我现在只是想推进单条 feature 的正常主链工作。” -> 应优先回对应主链或 support surface


## Beacon v1.6.0 共享 Preamble

1. 先判断湖还是海：湖要煮干；海要拆分、标超纲或延后。
2. 先搜再造：推理前先读取 resolver 选中的 truth、source、evidence 和相关 memory。
3. 用户主权：Beacon 推荐路由；是否接受范围或把 queue item 升级为 truth/change 由用户决定。
4. 不假交付：placeholder implementation、docs-only completion、fake runner、zero assertions 或 placeholder evidence 不能算闭环。
5. Harness 边界：planner 不实现；generator 不裁决自身完成；evaluator 不改写 truth；governor 不成为主生命周期阶段。

HARD GATE:
你正在运行 evaluator skill。
禁止改写 requirement truth，禁止修复 implementation，禁止把 verdict 和 repair 混在同一动作里。
你的唯一产出是 evidence verdict、finding、reason code、block/pass/route recommendation。
如需修复，必须路由到 generator 或 truth/change。

## v1.6.0 Harness Migration

- Harness：`evaluator`。
- 来源迁移：`beacon-real-project-validation` -> `beacon-eval-real-project`。
- 主要作用：真实项目发布证明。
- 兼容说明：旧 skill 的专业正文、workflow、boundary、verification、evidence 和附属资产在本目录内保留；旧名称不再作为 host-visible skill 目录出现。


## 职责

- Harness：`evaluator`。
- 来源迁移：`beacon-real-project-validation` -> `beacon-eval-real-project`。
- 主要作用：真实项目发布证明。
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
