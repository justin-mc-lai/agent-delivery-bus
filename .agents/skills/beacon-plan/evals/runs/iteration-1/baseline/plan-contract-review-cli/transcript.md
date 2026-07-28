# Eval plan-contract-review-cli (baseline)

## Prompt

feature=demo version=v0.0.1，需要可审计的多角度 review。说明如何跑 runtime evidence，execution_mode 与 artifact 路径。不要假装 parallel subagent。

## Output

Now I have a complete picture. Let me compile the comprehensive multi-angle review.

---

# 🔍 Multi-Angle Audit: `open-saas-base` — feature=`demo` version=`v0.0.1`

**Audit date**: 2026-07-21  
**Program**: `open-saas-base-v001`  
**Harness**: `beacon-plan` → `pln-program-auto`, now at `beacon-implement` readiness  
**P4 Ack**: ✅ acked by user 2026-07-21

---

## Angle 1: Program/Requirement Completeness (Truth vs PRD vs User Stories)

### 1.1 PRD → Feature Coverage

| PRD Goal | Lake | Covered? | Status |
|----------|------|----------|--------|
| CF Pages + D1 primary runtime | L0 + L5 | ✅ | wrangler.toml with D1 binding, deploy docs present |
| Better Auth identity (data self-owned) | L1 | ⚠️ | API surface exists. D1 persistence is declared "must" but evidence shows **in-memory only**. AC-ID-005 not yet satisfied at code level |
| Stripe billing + entitlement FSM | L2 | ✅ | 5-state FSM implemented, idempotent webhook, `DEFAULT_FEATURE_GRANTS` |
| Dual-locale shell (zh/en) | L3 | ✅ | i18n packs, `/api/i18n/:locale`, `/api/shell` |
| Plugin contract + example plugin | L4 | ✅ | Zod schema, `parsePluginManifest()`, `example-notes` plugin with ping route |
| Deploy docs/scripts for CF | L5 | ✅ | wrangler.toml, `docs/deploy/cloudflare.md`, `.env.example` |
| Agent skills: orient/add-plugin/smoke/release | L0 + L6 | ✅ | 4 skills materialized under `skills/` |

### 1.2 User Story → AC Coverage Gap

The FROZEN user journeys (`open-saas-base-journeys.md`) define 7 journeys with 24 ACs total. Every AC maps to a lake:

| Journey | AC Count | All Covered? |
|---------|----------|--------------|
| J-deploy | 3 | ✅ |
| J-identity | 5 | ⚠️ AC-ID-005 (D1 persistence) is documented as gap |
| J-billing | 4 | ✅ |
| J-shell | 3 | ✅ |
| J-plugin | 4 | ✅ |
| J-agent | 3 | ✅ |
| J-qa-release | 3 | ✅ |

**Finding**: 23/24 ACs covered. 1 explicit gap: D1 identity persistence declared as "must" but deferred to production wiring.

---

## Angle 2: Feature Graph Integrity

### 2.1 Dependency Graph Validation

```
l0-skeleton ──┬── l1-identity ──┬── l2-billing ──── l5-deploy
              │                 │
              │                 └── l3-shell ────── l4-plugin-v0
              │                                      │
              └── l4-plugin-v0 ◄─────────────────────┘
              │
              └── l5-deploy
                                                      │
l1-identity ──┬── l2-billing ── l6-smoke-qa ◄────────┘
              └── l3-shell ──── l4-plugin-v0 ────────┘
```

### 2.2 Issues Found

| # | Issue | Severity |
|---|-------|----------|
| G1 | `l3-shell` depends on `l1-identity` only — correct (shell needs auth API). But `l3-shell` doesn't declare dependency on `l2-billing` even though dashboard displays entitlement state (AC-SHELL-003 says "登录后 dashboard 显示 entitlement") | MEDIUM — either shell is display-only (can work without billing lake being complete) or dependency is missing |
| G2 | `l4-plugin-v0` depends on `l0-skeleton` + `l3-shell`. But the plugin contract has `entitlementsRequired` field and `hasFeature` integration. This implicitly needs L2 billing for full validation but the dependency isn't declared | LOW — plugin SDK can compile without billing; feature gating is runtime concern |
| G3 | `l5-deploy` depends on `l0-skeleton` + `l2-billing`. This is correct for D1 binding config, but deploy docs also need L1 identity secrets (BETTER_AUTH_SECRET). Not a broken dependency, just worth noting | LOW |

### 2.3 Topological Order Check

The declared topo order `l0 → l1 → l2 → l3 → l4 → l5 → l6` is **valid** — all edges go forward. However, parallelizable pairs exist:
- `l2-billing` and `l3-shell` could be parallelized (both only depend on `l1-identity`)
- `l5-deploy` could overlap with `l3-shell`/`l4-plugin-v0`

---

## Angle 3: Truth Freeze Readiness

All 7 feature packages have identical freeze status:

| Check | L0 | L1 | L2 | L3 | L4 | L5 | L6 |
|-------|:--:|:--:|:--:|:--:|:--:|:--:|:--:|
| User Intent one-liner | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Alignment vs Phased split | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Deferral ledger no pending | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| tests.md Command+Assertion | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Human freeze ack (operator)** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

**Finding**: All 7 feature packages are in `status: draft` with `Human freeze ack` unchecked. Per Beacon process, this is a **gate condition** — the planner review explicitly states: *"Do not implement further…may_implement: false until gen-truth freeze."*

However, all 28 tasks across 7 lakes are marked `[x]` done, and all evidence.md files show `status: evidence_recorded` with PASS results. This is a **process tension**: the code has been implemented before formal freeze.

---

## Angle 4: Evidence Traceability

### 4.1 AC → TC → Command → Evidence Chain

I traced every Acceptance Criteria through the tests.md coverage matrix. Here's the full chain for a representative sample:

```
AC-L2-001: payment_succeeded 从 none→active
  └─ TC-L2-001: unit layer
      └─ Command: pnpm exec vitest run packages/kernel/src/entitlements.test.ts
          └─ Assertion: FSM activates
              └─ Evidence: pnpm test (13 tests) PASS 2026-07-21
                  └─ Source: entitlements.ts applyEntitlementEvent("none","payment_succeeded") → {next:"active",changed:true}
✅ TRACEABLE
```

### 4.2 Evidence Quality Issues

| # | Issue | Severity |
|---|-------|----------|
| E1 | All 7 evidence.md files report **identical** evidence: `pnpm test (13 tests) PASS`. This is copy-pasted boilerplate — no lake-specific evidence differentiation. L5-deploy's evidence says "pnpm test (13 tests) PASS" when its tests.md only has doc-level grep commands | HIGH — evidence is not lake-specific; this undermines auditability |
| E2 | `pnpm test` reports 13 tests, but the tests.md matrices across 7 lakes describe ~29 TC rows. The mapping from which 13 tests cover which 29 TCs is opaque | MEDIUM |
| E3 | Smoke evidence (`qa/smoke-evidence.md`) claims "Register/login → `create-app.test.ts`" and "Checkout→entitlement FSM → kernel + web money loop tests" but doesn't show actual test names or output | MEDIUM |
| E4 | No test output files, coverage reports, or snapshot files are linked in any evidence.md. Evidence is purely declarative ("PASS") without artifact paths | HIGH |

---

## Angle 5: Source Code vs Claims Gap

### 5.1 Verified Alignments

| Claim (Truth doc) | Source Verification | Match? |
|-------------------|---------------------|--------|
| L0: `apps/web` + `packages/kernel` + `packages/plugin-sdk` + `plugins/*` layout | ✅ Directories exist | ✅ |
| L1: Better Auth config exposes baseURL/emailAndPassword | ✅ `packages/kernel/src/auth.ts` exists | ✅ |
| L2: Entitlement FSM with 5 states | ✅ `entitlements.ts` has `none|checkout_created|active|past_due|canceled` | ✅ |
| L2: Webhook idempotency via event ID dedup | ✅ `applyEntitlementEvent` returns `{ignored:true}` for `webhook_duplicate` | ✅ |
| L4: plugin.manifest.json schema via Zod | ✅ `manifest.ts` has `pluginManifestSchema` with Zod validation | ✅ |
| L5: wrangler.toml with D1 binding `DB` | ✅ `binding = "DB"` present; `database_id` is placeholder `00000000-...` | ✅ |
| L5: `.env.example` lists STRIPE/BETTER_AUTH/APP_URL | ⚠️ Not verified (file not read) | — |
| L6: 4 skills materialized | ✅ `skills/saas-base-orient/`, `saas-base-add-plugin/`, `saas-base-smoke-qa/`, `saas-base-release-checklist/` | ✅ |

### 5.2 Critical Gap

| # | Gap | Impact |
|---|-----|--------|
| C1 | **D1 persistence is not implemented.** L1 identity UD-L1-004 says "开发可用 memory store；生产路径文档要求 D1." AC-ID-005 says "生产路径：用户表在自有 D1，不得把身份数据只放第三方 Auth SaaS." The user journeys document states "v0.0.1 可先 memory store 证明 API；D1 persistence 为 L1 完成定义中的 must（未完成则 lake 不标 done）." This means **L1 is technically not done by its own definition** | HIGH — lake "done" status is self-contradictory |

---

## Angle 6: Security & Risk Review

### 6.1 PRD Risk Coverage

| PRD Risk | Mitigation Status | Verdict |
|----------|-------------------|---------|
| Payment webhook idempotency | ✅ `webhook_duplicate` event handled in FSM; event ID dedup noted in execution doc | ADEQUATE for v0 |
| Auth session security | ⚠️ Session store is in-memory. No token rotation, no refresh mechanism documented | NEEDS HARDENING for production |
| Plugin schema isolation | ⚠️ AC-PLG-004 says "卸载/禁用不得删除 kernel 用户与账单表（文档约束；运行时后续强化）" — this is a documentation-only constraint, no runtime enforcement | ACCEPTED as v0 limitation |

### 6.2 Secrets Audit

| Check | Status |
|-------|--------|
| `.env.example` lists keys, not values | ✅ (per AC-L5-003) |
| `wrangler.toml` has placeholder `database_id` | ✅ (`00000000-0000-0000-0000-000000000000`) |
| No hardcoded Stripe secret in source | ✅ (stub pattern) |

### 6.3 Unaddressed Risks

| Risk | Why |
|------|-----|
| CSRF protection | Not mentioned in any AC or truth doc |
| Rate limiting on auth endpoints | Not mentioned |
| Session token expiry/hijacking | Not in scope for v0 |

---

## Angle 7: Runtime Evidence Runbook

### 7.1 How to Reproduce Evidence

```bash
# === EXECUTION MODE ===
# All commands are OFFLINE-FIRST (no live CF/Stripe keys needed)
# Working directory: <workspace>/open-saas-base

# ---- CORE EVIDENCE (single command) ----
pnpm test
# Expected: 13 tests pass
# Artifact: stdout to terminal

# ---- SMOKE CHECKLIST ----
pnpm smoke:checklist
# Expected: all checklist items PASS
# Artifact: stdout to terminal

# ---- TYPECHECK ----
pnpm typecheck
# Expected: exit 0, no errors
# Artifact: stdout to terminal

# ---- PLUGIN VALIDATION ----
pnpm validate:plugins
# Expected: "example-notes" OK
# Artifact: stdout to terminal

# ---- DOC-LEVEL ASSERTIONS (grep-based) ----
# AC-L5-001: wrangler D1 binding
rg -n 'binding = "DB"' apps/web/wrangler.toml

# AC-L5-002: deploy doc steps
rg -n 'd1 create|secret put|deploy' docs/deploy/cloudflare.md

# AC-L5-003: env template
test -f .env.example && rg -n 'STRIPE|BETTER_AUTH|APP_URL' .env.example

# AC-L5-004: placeholder check
rg -n '00000000-0000-0000-0000-000000000000|placeholder|database_id' apps/web/wrangler.toml docs/deploy/cloudflare.md

# AC-L0-003: orient skill + AGENTS keywords
test -f skills/saas-base-orient/SKILL.md && rg -n 'Cloudflare|plugin|kernel' AGENTS.md

# AC-L0-004: PLAN program pointer
test -f PLAN.md && rg -n 'programs/open-saas-base-v001' PLAN.md
```

### 7.2 Artifact Path Map

```
Evidence artifacts                   Canonical doc source
─────────────────────────────────    ─────────────────────────────────
pnpm test stdout                     → docs/beacon/v0.0.1/qa/smoke-evidence.md
pnpm smoke:checklist stdout          → docs/beacon/v0.0.1/qa/smoke-evidence.md
Individual vitest runs               → docs/beacon/v0.0.1/features/l*/tests.md (coverage matrices)
grep match output                    → docs/beacon/v0.0.1/features/l*/tests.md (doc-layer TCs)
Program materials                    → docs/beacon/v0.0.1/programs/open-saas-base-v001/
Feature truth packages               → docs/beacon/v0.0.1/features/l*/
PRD                                  → docs/beacon/v0.0.1/prd/open-saas-base-v0.0.1.md
User journeys (FROZEN)               → docs/beacon/v0.0.1/user-story/open-saas-base-journeys.md
User stories                         → docs/beacon/v0.0.1/user-story/open-saas-base-v0.0.1.md
Execution domain doc                 → docs/beacon/v0.0.1/execution/domain-fsm-billing.md
Capability inventory                 → docs/beacon/v0.0.1/programs/open-saas-base-v001/source_capability_inventory.json
```

### 7.3 Per-Lake Test Execution

```bash
# L1 identity
pnpm exec vitest run apps/web/src/create-app.test.ts

# L2 billing FSM
pnpm exec vitest run packages/kernel/src/entitlements.test.ts

# L2 billing + L4 plugin (money loop integration)
pnpm exec vitest run apps/web/src/create-app.test.ts

# L3 i18n shell
pnpm exec vitest run apps/web/src/i18n.test.ts

# L4 plugin manifest
pnpm exec vitest run packages/plugin-sdk/src/manifest.test.ts

# L5/L6 doc-level: grep commands listed above
```

---

## Summary Verdict

| Dimension | Verdict |
|-----------|---------|
| Requirement completeness | **STRONG** — 23/24 ACs covered, 1 explicit deferred gap (D1 persistence) |
| Feature graph | **MINOR ISSUES** — 1 medium dependency ambiguity (L3 shell ↔ L2 billing), topo order valid |
| Truth freeze | **PROCESS GAP** — All 7 packages in `draft` status with unfrozen human ack, but code already implemented |
| Evidence traceability | **WEAK** — Evidence is copy-paste boilerplate across all lakes; no lake-specific trace; no artifact file paths |
| Source-code alignment | **STRONG** — Key claims (FSM, plugin schema, wrangler config, skills) all verified in source |
| Security | **ADEQUATE for v0** — No hardcoded secrets; placeholder DB ID; session security and CSRF are v0 deferred risks |
| Runtime runbook | **REPRODUCIBLE** — All commands are offline-first, deterministic, and documented in tests.md matrices |

### Top 3 Actions Before Claiming "Done"

1. **Freeze truth formally**: Run `beacon truth freeze` per lake or get human freeze ack on all 7 packages before further implementation
2. **Differentiate evidence**: Replace copy-pasted evidence.md boilerplate with lake-specific evidence (per-test output, timestamps, snapshot paths)
3. **Close the D1 gap**: Either implement D1 persistence for L1 identity (per AC-ID-005 "must"), or explicitly downgrade it to a deferred non-goal with a new version target


## Grade

```json
{
  "passed": 3,
  "total": 3,
  "score": 1.0,
  "results": [
    {
      "expectation": "mentions_planner_review_cli",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "mentions_artifact_path",
      "pass": true,
      "reason": "matched"
    },
    {
      "expectation": "no_fake_parallel_subagent",
      "pass": true,
      "reason": "matched"
    }
  ]
}
```
