---
version: alpha
name: agent-delivery-bus (PC 中台 / Operator Console)
description: "Dense operator workbench: left rail, filterable tables, detail drawers, explicit blocked/error recovery."
colors:
  primary: "#171717"
  on-primary: "#ffffff"
  secondary: "#525252"
  neutral: "#fafafa"
  canvas: "#ffffff"
  border: "#e5e5e5"
  muted: "#737373"
  danger: "#ee0000"
  success: "#0a7"
  warning: "#f5a623"
  link: "#0070f3"
typography:
  h1:
    fontFamily: "Geist, Inter, system-ui, sans-serif"
    fontSize: 24px
    fontWeight: 600
    lineHeight: 1.25
  h2:
    fontFamily: "Geist, Inter, system-ui, sans-serif"
    fontSize: 18px
    fontWeight: 600
    lineHeight: 1.3
  body-md:
    fontFamily: "Geist, Inter, system-ui, sans-serif"
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace"
    fontSize: 12px
    fontWeight: 500
    lineHeight: 1.3
rounded:
  sm: 4px
  md: 6px
  lg: 8px
spacing:
  sm: 8px
  md: 12px
  lg: 16px
  xl: 24px
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    rounded: "{rounded.md}"
  button-secondary:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.primary}"
    border: "{colors.border}"
  table-header:
    backgroundColor: "{colors.neutral}"
    textColor: "{colors.secondary}"
  input:
    border: "{colors.border}"
    rounded: "{rounded.sm}"
    minHeight: "32px"
---

# DESIGN.md — agent-delivery-bus (PC 中台 / Operator Console)

Status: ready
Preset: `operator-console` (Operator Console, source: vercel)
Beacon version context: `v0.0.4`
Generation: unattended-complete-baseline
Delivery surface: `pc-console` (PC 中台 / Operator Console)
Surface auto-reason: surface library materialize
Nav pattern: left-rail + top utility
Primary layout pattern: table + filter + detail drawer
Touch min: 32px · Density: compact

This file is the **project-level visual truth**. Implement / QA / UI ship gates treat it as binding when present.
It is complete enough to ship a coherent UI **without human design decisions**, while remaining fine-tunable against an existing DESIGN.md.

## Open-source UI agent principles (Beacon absorption)

Beacon design routes preserve upstream capabilities but **do not** treat them as runtime truth:

| Upstream | Routes | Kind | Absorbed into this DESIGN.md |
|----------|--------|------|------------------------------|
| vibe-to-ui | explore, extract | execution-capable | information_hierarchy, layout_models, first_screen_intent |
| ui-ux-pro-max | system | execution-capable | tokens, component_system, states, responsive, accessibility |
| impeccable | review, polish | execution-capable | spacing, focus_visibility, microcopy, ship_readiness, anti_ai_ui |
| awesome-design-md | library | reference-only | pattern_references_only |

Rules:
- `awesome-design-md` is **reference-only** (patterns / analogies). Never authority for ship.
- `vibe-to-ui` / `ui-ux-pro-max` / `impeccable` may generate candidates; accepted facts still bind via Beacon truth / this DESIGN.md / version `docs/beacon/<v>/design/*`.
- Google `design.md` YAML front matter above is **normative machine-readable tokens**; prose below is rationale.
- No external design skill is required at runtime to apply this baseline (`external_reference_runtime_dependency=false`).

## Overview

PC 中台 / Operator Console. Density `compact`. Navigation: left-rail + top utility. Primary pattern: table + filter + detail drawer.

## Design philosophy

1. **Lazy-user first**: first screen answers the main job before secondary chrome.
2. **Recognition over recall**: labels and primary actions must be obvious.
3. **State honesty**: empty / loading / error / success / blocked / stale / recovery are first-class.
4. **Restrained density**: no marketing hero, no fake dashboard charts, no AI gradient theater.
5. **Domain-bound chrome**: badges and primary buttons bind to business FSM states when a domain FSM exists.
6. **Fine-tune, don't freestyle**: when a prior DESIGN.md exists, extend it; do not invent a second design system.
7. **Surface honesty**: PC 中台 / Web / H5 / Client each keep their IA; do not paste mobile IA onto desktop console.

## Colors

See YAML `colors` tokens. Primary drives interaction; neutrals carry structure.

## Typography

See YAML `typography` tokens. Prefer system stacks for product OS feel.

## Layout

- Base grid from YAML `spacing`.
- First screen order: **status → primary action → supporting list → secondary disclosure**.
- Surface nav: left-rail + top utility.

## Elevation & Depth

- Prefer borders over shadows.
- No glassmorphism decoration.
- One elevation language only.

## Shapes

See YAML `rounded` scale.

## Components

See YAML `components` map. Implement maps these tokens to real components.

## Delivery surface pack (copy-ready)

Surface: `pc-console` — PC 中台 / Operator Console

### Surface Do
- Left rail for primary IA; overflow tools in secondary menus
- Tables are first-class; filters sticky; row actions explicit
- Blocked/error rows show recovery, not only red text
- Dense but readable; monospaced labels for IDs/timestamps

### Surface Don't
- Marketing hero or mesh gradients on operator pages
- Card-in-card nesting for form sections
- Hide destructive actions without confirm
- Use mobile bottom-tab IA on desktop console

### CSS variables (surface)

```css
:root {
  --bg: #fafafa;
  --fg: #171717;
  --card: #ffffff;
  --border: #e5e5e5;
  --muted: #737373;
  --primary: #171717;
  --primary-fg: #ffffff;
  --danger: #ee0000;
  --success: #0a7;
  --warning: #f5a623;
  --radius: 6px;
  --touch-min: 32px;
  --density: compact;
}
```

## Visual tokens (CSS variables legacy bridge)

```css
:root {
  --bg: #FAFAFA;
  --fg: #171717;
  --card: #FFFFFF;
  --border: #E5E5E5;
  --muted: #737373;
  --primary: #000000;
  --primary-fg: #FFFFFF;
  --danger: #E00;
  --success: #0A7;
  --warning: #F5A623;
  --radius: 6px;
  --font: ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-5: 24px;
  --space-6: 32px;
  --touch-min: 32px;
  --density: compact;
}
```

## Interaction principles

- focus-visible must exist for keyboard users
- primary interactive targets should be at least 44x44
- button hierarchy must distinguish primary from secondary actions
- empty/loading/error/success states must be intentional and explicit
- Keyboard: Tab order matches visual order; Escape closes overlays.
- Loading: skeletons or explicit progress; never blank freeze without explanation.
- Navigation: surface-native (left-rail + top utility).
- Handoff: deep-links restore page + entity + state.

## UI state coverage (required)

| State | User sees | Primary action |
|-------|-----------|----------------|
| empty | honest empty + why | create / import / connect |
| loading | progress or skeleton | cancel if long-running |
| error | cause + recovery | retry / fix config |
| success | confirmation without parade | next step or dismiss |
| blocked | gate reason | unblock path |
| stale | data age / refresh needed | refresh |
| recovery | post-failure path | resume / reset |

When Domain FSM exists, bind business states via `ui-state-matrix.md`. Ship gates require matrix states ⊆ domain states.

## Do's and Don'ts

### Do
- Quiet neutrals + one accent
- Explicit empty/loading/error/success
- Domain badges and disabled illegal actions
- Focus-visible rings and adequate targets
- Left rail for primary IA; overflow tools in secondary menus
- Tables are first-class; filters sticky; row actions explicit
- Blocked/error rows show recovery, not only red text
- Dense but readable; monospaced labels for IDs/timestamps

### Don't
- gradient text
- purple-to-blue ai gradient
- card inside card
- accent side stripe
- glassmorphism used as decoration
- bounce_or_elastic_easing
- Invent FSM states not in truth
- Copy reference-library aesthetics as project law
- Cross-surface IA paste (e.g. H5 bottom tabs on PC 中台)
- Marketing hero or mesh gradients on operator pages
- Card-in-card nesting for form sections
- Hide destructive actions without confirm
- Use mobile bottom-tab IA on desktop console

## Accessibility & responsive

- Contrast: body text meets WCAG AA intent.
- Focus visibility: required.
- Touch/pointer targets: surface min `32px` (critical actions still prefer 44px).
- Responsive: collapse secondary detail before primary judgment content.
- Motion: no bounce/elastic; prefer 150–200ms.

## Version-wide contracts

- `docs/beacon/<version>/design/style.md`
- `docs/beacon/<version>/design/component-system.md`
- `docs/beacon/<version>/design/interaction-contract.md`
- `docs/beacon/<version>/design/state-matrix.md`
- `docs/beacon/<version>/design/surfaces/<surface>.DESIGN.md` (copy-ready surface snapshot)
- feature `ui-state-matrix.md` bound to Domain FSM

## Unattended default & fine-tune policy

1. **No human design decision**: use this DESIGN.md + surface pack as visual OS.
2. **Existing DESIGN.md**: fine-tune mode keeps brand/domain specifics; fills missing axes.
3. **Requirement truth claims visual delivery** (`ux_required` / UI pages): auto-select surface from truth keywords, then freeze DESIGN.md before implement.
4. **Upstream polish**: `beacon design route polish|review|system|explore` may propose deltas; bind via prototype adapt / change-refreeze.
5. **Implement hard rule**: when DESIGN.md exists, LLM must not freestyle colors/spacing/components.

## Truth update policy

Update this file when accepted design facts become project-level truth.
Design support output is not truth until bound. Promise-changing visual changes require change/refreeze before implementation.
