# Eval plan-contract-review-parity (with_skill)

## Prompt

要把成熟 OSS 完整复刻到本项目，做全、同等能力、不要 MVP。请做 multi-angle planner review。即使源 OSS 名称尚未指定，也必须先输出结构化 contract：intent_snapshot/scope_mode/parity_matrix/deferral_ledger/findings/recommended_route，再列出澄清问题。禁止只提问不输出结构。

## Output

Now I have all the reference materials. Let me produce the full structured contract.

---

## Beacon Planner Review — Structured Output Contract

```yaml
# ──────────────────────────────────────────────
# META
# ──────────────────────────────────────────────
mode_id: pln-review
public_id: plan
execution_mode: single_process_multi_reviewer
fallback_reason: >
  No subagent runtime detected in the current environment.
  All reviewer lanes executed sequentially within a single process.
  This is not parallel subagent execution.
runtime_evidence_only: true
requirement_truth: false
formal_verdict_authority: false
release_verdict_authority: false
evidence_refs:
  - "beacon/.agents/skills/beacon-plan/references/pln-review/finding-schema.md"
  - "beacon/.agents/skills/beacon-plan/references/pln-review/intent-parity-artifacts.md"
  - "beacon/.agents/skills/beacon-plan/references/pln-review/state-machine-review.md"
  - "beacon/.agents/skills/beacon-plan/references/pln-review/compound-parity-review.md"
  - "beacon/.agents/skills/beacon-plan/references/pln-review/multi-agent-runtime.md"
```

---

### INTENT SNAPSHOT

```text
original_user_wording:
  "要把成熟 OSS 完整复刻到本项目，做全、同等能力、不要 MVP。
   请做 multi-angle planner review。即使源 OSS 名称尚未指定，
   也必须先输出结构化 contract，再列出澄清问题。禁止只提问不输出结构。"

strong_intent_signals:
  - "完整复刻" (complete replication)
  - "做全" (do it all / full scope)
  - "同等能力" (equivalent capability)
  - "不要 MVP" (no MVP)
  - "multi-angle planner review"

named_source_project: UNKNOWN (blocked)
source_url: UNKNOWN (blocked)
target_project: UNKNOWN (ambiguous — working directory is /Users/apple/Developer/Personal/products/)
non_negotiable_boundaries:
  - Must be full parity, NOT MVP
  - Must be multi-angle review (not single-pass)
  - Structured contract must precede clarifying questions
user_approved_deferral: none
```

---

### SCOPE MODE

```text
scope_mode: full_parity
reason: >
  User explicitly demanded 做全 (full scope), 同等能力 (equivalent capability),
  不要 MVP (no MVP). Under intent-parity-artifacts.md, this locks scope_mode
  to full_parity. Any downgrade requires an explicit deferral ledger entry
  and user decision.
```

---

### DIFF SCOPE

```text
base_ref: UNKNOWN
status: >
  No diff scope available. Source OSS project not identified.
  Target project within /Users/apple/Developer/Personal/products/ not specified.
  No code changes exist to review.
scope_confidence: 0
```

---

### SOURCE CAPABILITY INVENTORY

| # | Source capability | Source workflow/state | Source boundary/permission | Source evidence ref | Target Beacon landing | Target coverage expectation |
|---|---|---|---|---|---|---|
| — | **BLOCKED** | — | — | — | — | — |

```text
note: >
  Source OSS not identified. Source capability inventory is blocked
  until user supplies the OSS name, repo URL, and desired version.
```

---

### PARITY MATRIX

| Source capability | Target truth | Target tests | Target evidence | Status | Deferral |
|---|---|---|---|---|---|
| — | — | — | — | **blocked: missing source** | pending |

```text
note: >
  Parity matrix is empty because the source OSS has not been identified.
  Full parity requires a complete matrix mapping each source capability
  to Beacon truth, tests, implementation evidence, and status.
```

---

### DEFERRAL LEDGER

| deferral_id | capability | reason | impact | user_decision | expiry_or_revisit | recommended_route |
|---|---|---|---|---|---|---|
| `defer-001` | ALL capabilities | Source OSS not yet identified | Cannot begin any capability mapping | **pending** | After user specifies source OSS | `stop` → `plan` after source identified |

```text
note: >
  Currently the entire replication is deferred not by choice but by
  underspecification. This is not a scope cut — it's a prerequisite block.
```

---

### STATE MODEL

```text
state_model:
  required: true
  reason: >
    OSS replication inherently involves lifecycle, workflow, state
    management, and cross-component transitions. Once source is identified,
    the source's own state model must be inventoried and mapped.
  states: BLOCKED (source unknown)
  allowed_transitions: BLOCKED (source unknown)
  invalid_transitions: BLOCKED (source unknown)
  side_effects: BLOCKED (source unknown)
  recovery_paths: BLOCKED (source unknown)
  rollback_paths: BLOCKED (source unknown)
  resume_paths: BLOCKED (source unknown)
  terminal_states: BLOCKED (source unknown)
  state_axes: BLOCKED (source unknown)
  axis_combinations: BLOCKED (source unknown)
  coverage_implications: >
    Once source is identified, Beacon truth must include a State Model
    or Diagram Truth Layer. Route to beacon-gen-truth for initialization.
```

---

### SELECTED REVIEWERS + LANES

| # | Reviewer | Type | Trigger | Lane output |
|---|---|---|---|---|
| 1 | `intent-fidelity` | always-on | User said 做全/同等能力/不要 MVP | 2 findings (P0, P1) |
| 2 | `scope-mode` | always-on | Scope classification required | Locked to `full_parity` |
| 3 | `coverage-shape` | always-on | Behavioral evidence vs mock-only | 1 finding (P1) |
| 4 | `route-boundary` | always-on | Planner boundary enforcement | 1 finding (P0) |
| 5 | `state-machine` | always-on | Lifecycle/workflow/parity | 1 finding (P1) |
| 6 | `source-parity` | conditional | 完整复刻/同等能力/做全 | 1 finding (P0) |
| 7 | `deferral-sovereignty` | conditional | MVP/deferral language present | 1 finding (P0) |
| 8 | `security-boundary` | conditional | OSS replication → auth/permission | *deferred* (source unknown) |
| 9 | `performance-boundary` | conditional | OSS replication → runtime bounds | *deferred* (source unknown) |
| 10 | `brownfield-contract` | conditional | Target project may be existing codebase | *deferred* (target unknown) |

---

### FINDINGS

---

**FINDING #1**

```text
finding_id: review-intent-001
reviewer: intent-fidelity
lane_id: lane-intent-fidelity-01
severity: P0
confidence: 100
authority_level: user_quote
issue: >
  User demanded full parity (做全/同等能力/不要 MVP) for OSS replication,
  but neither the source OSS project name, URL, version, nor the target
  project within /Users/apple/Developer/Personal/products/ has been specified.
  Scope mode is locked to full_parity per user intent, but no capabilities
  can be inventoried, no parity matrix can be built, and no truth can
  be frozen without the source reference.
evidence_refs:
  - user prompt (this conversation turn)
  - beacon/.agents/skills/beacon-plan/references/pln-review/intent-parity-artifacts.md (lines 28-31)
affected_surface: all truth surfaces (PRD, user-story, state-model, test-cases)
dedup_key: missing-source-oss
recommended_route: stop
user_decision_required: true
autofix_allowed: false
```

---

**FINDING #2**

```text
finding_id: review-source-parity-001
reviewer: source-parity
lane_id: lane-source-parity-01
severity: P0
confidence: 100
authority_level: assumption
issue: >
  Source capability inventory and parity matrix are both empty/blocked.
  Full parity delivery requires a complete source-to-target mapping.
  Without the source OSS identified, the entire parity review is blocked.
  Continuing to any downstream harness (truth/implement) without source
  identification violates the full_parity contract.
evidence_refs:
  - beacon/.agents/skills/beacon-plan/references/pln-review/intent-parity-artifacts.md (lines 30-46)
  - beacon/.agents/skills/beacon-plan/references/pln-review/compound-parity-review.md (lines 70-80)
affected_surface: parity_matrix, source_capability_inventory, coverage_mapping
dedup_key: missing-source-oss
recommended_route: stop
user_decision_required: true
autofix_allowed: false
```

---

**FINDING #3**

```text
finding_id: review-deferral-001
reviewer: deferral-sovereignty
lane_id: lane-deferral-sovereignty-01
severity: P0
confidence: 100
authority_level: user_quote
issue: >
  All capabilities are currently in a de facto deferred state because the
  source OSS is unknown. Per deferral sovereignty rules, the planner must
  not recommend freezing truth or routing to implement while deferral
  decisions are pending. The user must explicitly decide: (a) identify
  the source OSS, (b) define the target project, (c) confirm full_parity
  scope or authorize a scope downgrade.
evidence_refs:
  - beacon/.agents/skills/beacon-plan/references/pln-review/intent-parity-artifacts.md (lines 58-72)
affected_surface: deferral_ledger, scope_mode
dedup_key: missing-source-oss
recommended_route: stop
user_decision_required: true
autofix_allowed: false
```

---

**FINDING #4**

```text
finding_id: review-route-001
reviewer: route-boundary
lane_id: lane-route-boundary-01
severity: P0
confidence: 100
authority_level: assumption
issue: >
  Planner review cannot route to beacon-gen-truth, beacon-gen-implement,
  or beacon-eval-qa while P0 findings remain. The current block (missing
  source OSS) prevents any downstream harness work. Must stop here.
  Also: this is a planner review, not an implementation session. No code,
  no truth freeze, no QA/release verdict is authorized.
evidence_refs:
  - beacon/.agents/skills/beacon-plan/references/pln-review/finding-schema.md (lines 31-42)
  - beacon/.agents/skills/beacon-plan/references/pln-review/multi-agent-runtime.md (lines 48-54)
affected_surface: route_recommendation
dedup_key: route-boundary-enforcement
recommended_route: stop
user_decision_required: false
autofix_allowed: false
```

---

**FINDING #5**

```text
finding_id: review-state-001
reviewer: state-machine
lane_id: lane-state-machine-01
severity: P1
confidence: 75
authority_level: assumption
issue: >
  OSS replication inherently involves lifecycle, workflow, and state
  management. The state-machine reviewer is triggered by full_parity scope
  and the inherently stateful nature of replication work. However, the
  state model cannot be constructed until the source OSS is identified
  and its own workflows/states are analyzed. Beacon truth will require
  a State Model or Diagram Truth Layer once the source is known.
evidence_refs:
  - beacon/.agents/skills/beacon-plan/references/pln-review/state-machine-review.md (lines 7-14)
affected_surface: state_model, truth.md, tests.md
dedup_key: state-model-blocked-source
recommended_route: beacon-gen-truth (after source identified)
user_decision_required: true
autofix_allowed: false
```

---

**FINDING #6**

```text
finding_id: review-coverage-001
reviewer: coverage-shape
lane_id: lane-coverage-shape-01
severity: P1
confidence: 50
authority_level: assumption
issue: >
  Full parity replication must be verified by behavioral/runtime evidence,
  not only static/mock/docs proof. Without knowing what the source OSS is,
  the coverage shape cannot be designed. Once source is identified, the
  coverage_mapping must specify: unit tests, integration tests, E2E tests,
  and behavior evidence for each source capability.
evidence_refs:
  - beacon/.agents/skills/beacon-plan/references/pln-review/reviewer-catalog.md (coverage-shape row)
affected_surface: coverage_mapping, tests.md, evidence plan
dedup_key: coverage-blocked-source
recommended_route: beacon-eval-qa (after parity matrix populated)
user_decision_required: false
autofix_allowed: false
```

---

### SYNTHESIS

```text
merged_findings:
  - dedup_key "missing-source-oss" consolidates findings #1, #2, #3
    (intent-fidelity, source-parity, deferral-sovereignty all point to
     the same root cause: source OSS not identified)

conflict_groups: none
highest_severity: P0
consensus_route: stop

summary: >
  The review cannot proceed beyond the planner stage. Four P0 findings
  all converge on a single blocker: the source OSS has not been identified.
  The user's intent is clear (full parity, complete replication, no MVP),
  but the prerequisite information (source project name, URL, version,
  target project within the products monorepo) is missing.

  The planner must stop here. No truth freeze, implementation, QA, or
  release routing is authorized until the source OSS is identified and
  a source capability inventory can be built.
```

---

### RECOMMENDED ROUTE

```text
recommended_next_harness: stop
reason: >
  P0 findings remain unresolved. Source OSS not identified.
  Cannot route to truth/implement/QA/release.
next_steps_once_unblocked:
  1. User identifies source OSS → re-run pln-review with source refs
  2. After source identified → route to beacon-gen-truth for truth freeze
  3. After truth frozen → route to beacon-gen-implement for code
  4. After implementation → route to beacon-eval-qa for evidence
  5. After QA → route to beacon-eval-release for release gate
```

---

### AUTOFIX ROUTING

| Class | Finding IDs | Route |
|---|---|---|
| `manual` | review-intent-001, review-source-parity-001, review-deferral-001, review-route-001 | **human** — user must supply source info |
| `advisory` | review-state-001, review-coverage-001 | **defer/queue** until source identified |

---

### RELEASE OPS REVIEW

```text
status: NOT APPLICABLE
reason: >
  No implementation, no truth, no release candidate exists.
  Release ops review is deferred until after implementation.
```

---

### QA EVIDENCE HYGIENE

```text
status: NOT APPLICABLE
reason: >
  No tests, no junit.xml, no assertions, no benchmarks exist.
  QA evidence hygiene review is deferred until after implementation.
```

---

## 澄清问题 Clarifying Questions

在结构化 contract 之后，以下是需要你回答的关键问题：

### Q1: 源 OSS 项目是什么？

请提供：
- OSS 项目名称
- GitHub/GitLab 仓库 URL
- 目标版本/分支/tag（如 `v3.2.1` 或 `main`）

> 例如：「把 Plane (https://github.com/makeplane/plane) v0.22.0 完整复刻」

### Q2: 目标项目是哪一个？

`/Users/apple/Developer/Personal/products/` 下有 40+ 个子项目，你希望在哪个项目中复刻？
- `beacon/` — Beacon 产品本身？
- `open-saas-base/` — 开放 SaaS 基础？
- `selfmedia-sync-ai/` — 自媒体同步工具？
- `selfmedia-creator/` — 自媒体创作者？
- 还是创建一个新项目？

### Q3: 复刻的深度边界？

- **完整复刻前端 + 后端 + 数据库 + 基础设施**？
- **仅后端 API + 数据模型**？
- **仅前端 UI/UX**？
- **需要基础设施部署（Docker、K8s、CI/CD）？**

### Q4: 技术栈约束？

- 必须与原 OSS 相同技术栈，还是可以自由选择？
- 例如原 OSS 用 Python/Django，目标可以改用 Go/Node.js 吗？
- 是否有必须复用的基础设施（如 Supabase、Vercel、特定的云服务商）？

### Q5: 复刻后的目标用途？

- 学习研究？
- 作为本项目的核心引擎？
- 商业化/产品化？
- 这会影响许可证合规、架构决策和 scope 完整性判断。

---

**当前状态：BLOCKED — 等待用户提供源 OSS 身份后重新进入 pln-review。**


## Grade

```json
{
  "passed": 5,
  "total": 5,
  "score": 1.0,
  "results": [
    {
      "expectation": "scope_full_parity_or_block",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "has_findings",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "has_severity",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "not_silent_implement_route",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "has_parity_or_deferral",
      "pass": true,
      "reason": "matched"
    }
  ]
}
```
