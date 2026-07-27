# Mode: loop-goal

> Archived from `beacon-loop-goal` during 1+6 merge. This is progressive disclosure content for `beacon-goal`.

# Beacon Loop Goal

> ⚠️ **Support 发现器，不是日常交付入口。**  
> 一句话做到验收 → 请用 **`$beacon-goal`**（见 `examples/goal-one-shot-30s.md`）。


## Overview

**Support/internal discovery surface.** Enqueues intents into `goal-queue` for `$beacon-goal` to consume.
May assist advanced/heartbeat routing, but is **not** the user-facing one-shot delivery entry.
Governor harness type — only routes/discovers, never executes truth/implement/QA/release verdicts.


## Product Goal Split (v1.6.7+ R3)

- **唯一公共入口：`beacon-goal`**
- **本 skill：support discovery only**

- **This skill (`beacon-loop-goal`)**: discovery / triage / enqueue only.
- **Finish run-until-done**: use **`beacon-goal`** (Product Goal Runtime).
- Do not mark product delivery complete from this skill.

## When to Use

- Advanced/heartbeat discovery needs to **enqueue** work for `$beacon-goal`
- Debugging support routing / queue inspection
- Explicit support-path recovery (not the default daily entry)

**Daily delivery:** always prefer `$beacon-goal` / `beacon goal run`.

## Harness

**governor** - This skill only does routing, state tracking, and stage decisions. It never writes truth, implements code, gives QA verdicts, or issues release verdicts.

## Beacon v1.6.0 共享 Preamble

**Worktree flow (always):** `skills/beacon/references/git-worktree-execution-flow.md`


1. 先判断湖还是海：湖要煮干；海要拆分、标超纲或延后。
2. 先搜再造：推理前先读取 resolver 选中的 truth、source、evidence 和相关 memory。
3. 用户主权：Beacon 推荐路由；是否接受范围或把 queue item 升级为 truth/change 由用户决定。
4. 不假交付：placeholder implementation、docs-only completion、fake runner、zero assertions 或 placeholder evidence 不能算闭环。
5. Harness 边界：planner 不实现；generator 不裁决自身完成；evaluator 不改写 truth；governor 不成为主生命周期阶段。

HARD GATE:
你正在运行 governor skill。
禁止定义 requirement truth，禁止执行 implementation，禁止给 QA/release verdict，禁止成为主生命周期 stage。
你的唯一产出是 context/metadata/archive/hooks/automation/status/diagnostic support。
如发现 truth、implementation 或 gate 问题，只能路由。

## State Machine

```text
idle -> governance_precheck
  -> pass: truth_routing | blocked: governance_repair -> idle
truth_routing -> invoke beacon-gen-truth -> truth_verify
  -> frozen: review_routing | not_frozen: truth_routing (retry) | blocked: error_recovery
review_routing -> invoke beacon-pln-review -> review_verify
  -> implement_ready: standard_execution_precheck
  -> implement_ready (non_blocking incl P1-only advisory): standard_execution_precheck + continue audit
  -> truth_or_change_route: drift_routing
  -> user_decision_required | P0: error_recovery
  -> blocked: error_recovery
standard_execution_precheck -> verify canonical baseline + branch governance + workspace admission
  -> pass: implement_routing
  -> blocked: error_recovery
implement_routing -> invoke beacon-gen-implement -> implement_verify
  -> tests_pass: qa_routing | tests_fail: implement_routing (repair loop, max 3) | blocked: error_recovery
qa_routing -> invoke beacon-eval-qa -> qa_verify
  -> pass: release_gate | fail: implement_routing (repair loop, max 3) | blocked: error_recovery
release_gate -> human_confirmed: invoke beacon-eval-release -> done
  -> human_rejected: idle
error_recovery -> invoke beacon-gov-debug -> resume from blocked stage
```

## Same-Session Continue (AC-028)

After emitting the next route / next skill, **immediately load that skill in the same session**. Do not end the orchestration turn by only printing the next step. Use `beacon status loop-goal --resume [--utterance 继续] --run-id <run_id> --json` when recovering mid-run; a successful resume always returns actionable `current_stage` + `next_skill` + `resume_action` (or an explicit human-wait at `release_gate`). No-op success printing is forbidden.

## R7 Intent Continuation & Dual Runtime (AC-031..040)

### Resilient Stop Contract (AC-039)

Every stop path must emit `reason_code` + `resume_action` (`none` | `load_next_skill` | `await_human_confirmation` | `replay_substep`). Never return pass/no-op without state change when blocked or waiting.

### Intent Context (AC-031..033)

1. On each routing fork call `loop_goal_state.record_intent_fork(stage, utterance_slice, decision, reason_code)`.
2. Persist `seed_utterance`, `latest_utterance`, `intent_forks[]`, `decision_cache[]` in state outputs / `.beacon/state/loop-goal/<run_id>.intent-context.json`.
3. When `user_decision_required`, cache the user answer via `record_user_decision()`; do not re-AskQuestion for the same decision key in the same run.

### Interrupt Semantics (AC-034)

- `host_turn_interrupted` → resume to `last_substep` with `resume_action=replay_substep` (not full error_recovery restart).
- `stage_blocked` → route `beacon-gov-debug` or wait per stage contract.

### Resume Utterance Map (AC-035)

Short continuation tokens (`继续`, `continue`, `resume`, `go`) with a valid run map deterministically to `resume_action=load_next_skill` via `beacon status loop-goal --resume --utterance <token>`.

### Dual Runtime Parity (AC-036, AC-038)

| Mode | Entry | Contract |
|------|-------|----------|
| `host_skill` | Load this skill | AC-028 same-session continue |
| `cli_agent` | `beacon loop-goal run --project . --version <version> --feature <feature> --utterance "<prompt>" [--json]` | Same `loop-goal-state.v1` file; `build_resume` output must match host skill for the same `run_id` |

### Topology Gate (AC-037)

Before implement, `standard_execution_precheck` must consume `topology_gate_status` from `GitScopeGraph` (`.beacon/state/git-scope-graph/<version>/<feature>.json`). `blocked` topology_gate → `error_recovery`; do not enter `implement_routing`.

## Decision Tree

When this skill is invoked, follow this decision tree sequentially:

### Step 0: Initialize

1. Call `loop_goal_state.init()` with version, feature, utterance.
2. Read `run_id` from the returned state.

### Step 1: Governance Precheck

1. Run `beacon plan start --project-root . --version <version> --json`.
2. If blocked, call `loop_goal_state.route_to_error_recovery()` with the reason code, output the `beacon-gov-debug` route, and stop.
3. If pass, call `loop_goal_state.transition(to_stage="truth_routing")` and continue.

### Step 2: Truth Stage

1. Verify the current checkout is on governance `truth_canonical` (`main` or `master`); if not, route to `beacon-gov-debug` with `reason_code=truth_canonical_branch_mismatch` and stop.
2. Route to `beacon-gen-truth`.
3. After it returns, read `docs/beacon/<version>/features/<feature>/truth.md` frontmatter.
4. If `status: frozen`, transition to `review_routing`.
5. If not frozen, stay in `truth_routing`, output the next route, and stop.

### Step 2.5: Planner Review Decision Packet

1. Route to `beacon-pln-review` when lifecycle, state, loop, source parity, deferral, design, or cross-harness risk is present.
2. When runtime evidence is needed, run `beacon planner-review run <feature> --project-root . --version <version> --prompt "<utterance>" --json`.
3. Record the returned artifact with `loop_goal_state.record_review_decision()`.
4. **R6 blocking set only:** stop when `highest_severity=P0`, or `user_decision_required=true`, or `recommended_route ∈ {beacon-gen-truth, beacon-gen-change, beacon-gen-refreeze}`, or the packet is incomplete / unhandled. Route to `drift_routing` / `error_recovery` / `beacon-gov-debug` as recorded — do **not** auto-enter `implement_routing`.
5. **Standalone P1 (and P2/P3) must not block** when the route is implement-ready: continue to `standard_execution_precheck`, and require the continue audit (`continued_with_advisory=true` + severity). `--human-gate` / `auto_decide=false` may elevate P1 to a stop.
6. Only transition to `standard_execution_precheck` when the recorded decision packet is implement-ready (including P1-only advisory continue).
7. After recording, **same-session continue**: immediately load the next skill / CLI precheck — do not end the turn with print-only routing.

### Step 2.6: Standard Execution Precheck

1. Run `beacon status standard-execution-precheck <feature> --project-root . --version <version> --run-id <run_id> --json` (CLI-owned verification bundle; do not hand-craft pass results).
2. Verify canonical truth/refreeze has been committed on `main` or `master`.
3. Verify the feature worktree contains `truth_baseline_commit`; if not, block with `reason_code=truth_baseline_not_merged`.
4. If feature-local `.machine` revision/projection dirty state or stale refreeze is detected, require a scoped stash/archive quarantine ref before continuing.
5. Require the CLI verification bundle containing `truth-map verify` / `truth_map verify`, `verify-materials --strict`, `git diff --check`, `branch-governance verify`, workspace admission results, and **`topology_gate`** (v1.6.7+ via `beacon workspace topology-resolve`).
6. Require `implement check-branch` / workspace admission to pass and record `workspace_admission_ref`.
7. Require `topology_gate_status=pass` (or `up_to_date`) and record `topology_gate_ref`; `blocked` → `error_recovery` with `reason_code` from graph.
8. Record the CLI artifact with `loop_goal_state.record_standard_execution_precheck()` (rejects non-CLI artifacts with `verification_bundle_not_cli_owned`).
9. If the result is blocked (including topology_gate), route to `beacon-gov-debug`; only `pass` or `up_to_date` may continue to `implement_routing`.

### Step 3: Implement Stage

1. Route to `beacon-gen-implement`.
2. After it returns, run the feature tests declared by the requirement package.
3. If tests pass, transition to `qa_routing`.
4. If tests fail, increment repair loop; retry implement up to 3 times, then route to `beacon-gov-debug`.

### Step 4: QA Stage

1. Route to `beacon-eval-qa`.
2. If QA passes, transition to `release_gate`.
3. If QA fails, return to `implement_routing` up to the repair limit, then route to `beacon-gov-debug`.

### Step 5: Release Gate (HUMAN GATE - ALWAYS)

1. Stop and wait for human confirmation.
2. Even if `auto_decide=true`, release always requires human confirmation.
3. After confirmation, call `loop_goal_state.check_release_gate(human_confirmed=True)` and route to `beacon-eval-release`.
4. If human rejects, transition to `idle`.

### Error Recovery

1. Route blocked stages to `beacon-gov-debug`.
2. After repair, resume from the blocked stage.
3. If repair cannot complete, stop for manual intervention.

## Skill Routing Matrix

| Stage | Skill to Load | Check After Return |
|-------|--------------|-------------------|
| governance_precheck | CLI: `beacon plan start` | `status=pass` or `blocked` |
| truth_routing | `beacon-gen-truth` | on `truth_canonical` branch? `truth.md status=frozen`? |
| review_routing | `beacon-pln-review` | decision packet allows implement? |
| standard_execution_precheck | CLI: `beacon status standard-execution-precheck`; then `record_standard_execution_precheck` | CLI-owned bundle artifact present? canonical baseline committed/merged? workspace admission pass? |
| implement_routing | `beacon-gen-implement` | feature tests pass? |
| qa_routing | `beacon-eval-qa` | verdict=pass? |
| release_gate | `beacon-eval-release` | go/no-go after human confirmation |
| error_recovery | `beacon-gov-debug` | fix applied? |

## Auto-Decide Rules

| Decision Point | Default Behavior | Can Override? |
|---------------|-----------------|---------------|
| truth scope (lake/ocean) | auto-decide | `--human-gate` |
| truth freeze | auto-decide after verify pass | `--human-gate` |
| planner-review P1-only | auto-continue → `standard_execution_precheck` + continue audit | `--human-gate` elevates P1 to stop |
| planner-review blocking (P0 / user_decision_required / drift routes) | always stop | never auto-bypass |
| implement start | auto-decide after truth frozen + review implement-ready + precheck pass | `--human-gate` |
| QA start | auto-decide after tests pass | `--human-gate` |
| release start | must human confirm | never |

## State File

State is persisted at `.beacon/state/loop-goal/<run_id>.json` with schema `loop-goal-state.v1`.
Standard execution outputs include `standard_execution_flow_status`, `truth_baseline_commit`, `canonical_refreeze_committed`, `feature_worktree_reconciled`, `stale_feature_refreeze_quarantine_ref`, `verification_bundle_ref`, `workspace_admission_ref`, and (v1.6.7+) `topology_gate_status` / `topology_gate_ref`.
R7 intent outputs include `intent_context_ref`, `intent_forks`, `decision_cache`, `last_substep`, `loop_execution_mode`, and `resume_action`.
Observability outputs include `human_stop_metrics` (`human_stops_per_run`, `stop_reason_counts`, `p1_continue_count`, `pseudo_stall_flag`) and `planner_review_continue_audit`.
Resume via `beacon status loop-goal --resume [--run-id <id>] [--utterance <token>] --json` or `beacon loop-goal run` for CLI+agent parity.

## Boundary

- This skill is a governor skill - it never executes truth, implement, QA, or release.
- It uses sequential skill loading; each skill completes before the next route decision.
- It relies on file-system state, not agent memory.
- Release is always human-gated.
- Standalone P1 is advisory continue by default; only release + review blocking set are default human hard-stops.
