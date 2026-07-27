# Reviewer Catalog

`beacon-pln-review` keeps these reviewers internal. They are not host-visible public skills and they do not write truth, code, QA verdicts, release verdicts, or `.machine/`.

## Always-On Reviewers

| Reviewer | Purpose | Typical finding |
|---|---|---|
| `intent-fidelity` | Preserve the user's original outcome, strength words, and non-negotiable promises. | User asked for full parity but plan reduced to MVP. |
| `scope-mode` | Classify scope as `full_parity`, `mvp`, `patch`, `research`, `brownfield_takeover`, or `compatibility_only`. | Scope mode missing or inconsistent with user wording. |
| `coverage-shape` | Check whether acceptance can become behavior evidence, not only mock/static/docs proof. | Tests rely on mock-only or static proof for real behavior. |
| `route-boundary` | Keep planner inside research/planning artifact and route recommendation. | Planner tries to write truth, code, `.machine`, QA verdict, or release verdict. |
| `state-machine` | Review lifecycle, workflow, loop, resume/stop/recovery, permission and source-parity branch flow, illegal transitions, terminal states, and state axes. | Feature has lifecycle risk but no State Model / Diagram Truth Layer. |

## Conditional Reviewers

| Reviewer | Trigger | Output focus |
|---|---|---|
| `source-parity` | Source project, OSS, mature reference, complete clone, same capability, full parity, or "做全". | Source capability inventory and parity matrix. |
| `deferral-sovereignty` | MVP, cut, later, defer, partial coverage, blocked implementation, or unimplemented capability. | Deferral ledger and explicit user decision requirement. |
| `security-boundary` | Auth, permissions, public routes, input parsing, secrets, sensitive data, or remote execution. | Permission, trust, data exposure, and fail-closed questions. |
| `performance-boundary` | Long tasks, loops, concurrency, caching, bulk processing, token/time budget, or retry. | Runtime bounds, backpressure, observability, and benchmark needs. |
| `design-flow` | UI/UX, user workflow, cross-screen state, design system, or interaction state. | Flow states, visual state coverage, handoff route. |
| `brownfield-contract` | Takeover, legacy truth, mixed docs/runtime, compatibility window, archived docs, or migration debt. | Authority map, compatibility boundary, migration evidence. |
| `evidence-proof-shape` | Claim depends on tests, benchmark, runtime evidence, QA, or release readiness. | Evidence class, proof strength, fake-delivery risk. |
| `simplification-risk` | Simplification could erase parity, user promises, state branches, or acceptance detail. | What is safe to simplify and what needs user approval. |
| `founder-business` | Product/business outcome, ROI, positioning, customer pain, or market-level challenge is requested. | Business risk and founder-level tradeoff. |
| `adversarial-doubt` | Assumptions are weak, user asks to challenge, or findings need falsification. | Counterexamples and disconfirming evidence. |
| `adr-documentation` | Architectural decision, durable tradeoff, or documentation clarity is needed. | ADR route and decision record shape. |
| `context-engineering` | Prompt/context drift, missing docs, or overloaded context could change output. | Context package, source order, and missing anchors. |

## Synthesis Rule

Merge duplicate findings by affected truth surface and route. Keep the highest severity, highest authority evidence, and any conflicting reviewer confidence. The final output has one recommended route, plus unresolved conflicts when reviewers disagree.
