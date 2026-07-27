# Beacon example — complete reusable DESIGN.md

Format: google-labs-code/design.md compatible YAML front matter + Beacon prose.
Surfaces: see `skills/beacon/examples/design-surfaces/`.

```bash
# auto-select surface from requirement truth and write DESIGN.md
beacon design baseline --project . --version auto --surface auto --write --json

# explicit ends
beacon design baseline --project . --surface pc-console --write --force --json
beacon design baseline --project . --surface web --write --force --json
beacon design baseline --project . --surface h5 --write --force --json
beacon design baseline --project . --surface client --write --force --json
```

When DESIGN.md exists: non-destructive fine-tune proposal.
When truth has `ux_required` / UI pages: implement must not freestyle — baseline first, then strict DESIGN alignment.

---

---
version: alpha
name: Example Product
description: "Restrained product web: quiet neutrals, one accent, content hierarchy before chrome."
colors:
  primary: "#0071e3"
  on-primary: "#ffffff"
  secondary: "#6e6e73"
  neutral: "#f5f5f7"
  canvas: "#ffffff"
  ink: "#1d1d1f"
  border: "#d2d2d7"
  danger: "#ff3b30"
  success: "#34c759"
  warning: "#ff9f0a"
typography:
  h1:
    fontFamily: "SF Pro Display, system-ui, -apple-system, sans-serif"
    fontSize: 40px
    fontWeight: 600
    lineHeight: 1.1
    letterSpacing: -0.02em
  h2:
    fontFamily: "SF Pro Text, system-ui, -apple-system, sans-serif"
    fontSize: 24px
    fontWeight: 600
    lineHeight: 1.25
  body-md:
    fontFamily: "SF Pro Text, system-ui, -apple-system, sans-serif"
    fontSize: 16px
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "SF Pro Text, system-ui, -apple-system, sans-serif"
    fontSize: 13px
    fontWeight: 500
    lineHeight: 1.3
rounded:
  sm: 6px
  md: 8px
  lg: 12px
spacing:
  sm: 8px
  md: 16px
  lg: 24px
  xl: 32px
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    rounded: "{rounded.md}"
    minHeight: "44px"
  button-secondary:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    border: "{colors.border}"
  card:
    backgroundColor: "{colors.canvas}"
    border: "{colors.border}"
    rounded: "{rounded.md}"
---

# DESIGN.md — Example Product

Status: ready
Preset: `restrained-product` (Restrained Product, source: apple)
Beacon version context: `v1.6.7`
Generation: unattended-complete-baseline
Delivery surface: `web` (Web Product / Marketing-capable)
Surface auto-reason: explicit
Nav pattern: top nav + content column
Primary layout pattern: content first + primary CTA
Touch min: 44px · Density: comfortable

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

Web Product / Marketing-capable. Density `comfortable`. Navigation: top nav + content column. Primary pattern: content first + primary CTA.

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
- Surface nav: top nav + content column.

## Elevation & Depth

- Prefer borders over shadows.
- No glassmorphism decoration.
- One elevation language only.

## Shapes

See YAML `rounded` scale.

## Components

See YAML `components` map. Implement maps these tokens to real components.

## Delivery surface pack (copy-ready)

Surface: `web` — Web Product / Marketing-capable

### Surface Do
- One primary CTA per viewport
- Quiet neutrals + single accent
- Product imagery or content first; chrome recedes
- Explicit empty/loading/error/success

### Surface Don't
- Purple-to-blue AI gradients
- Multiple competing primaries
- Glassmorphism decoration
- Dense admin tables as the default public IA

### CSS variables (surface)

```css
:root {
  --bg: #f5f5f7;
  --fg: #1d1d1f;
  --card: #ffffff;
  --border: #d2d2d7;
  --muted: #6e6e73;
  --primary: #0071e3;
  --primary-fg: #ffffff;
  --danger: #ff3b30;
  --success: #34c759;
  --warning: #ff9f0a;
  --radius: 8px;
  --touch-min: 44px;
  --density: comfortable;
}
```

## Visual tokens (CSS variables legacy bridge)

```css
:root {
  --bg: #F5F5F7;
  --fg: #1D1D1F;
  --card: #FFFFFF;
  --border: #D2D2D7;
  --muted: #6E6E73;
  --primary: #0071E3;
  --primary-fg: #FFFFFF;
  --danger: #FF3B30;
  --success: #34C759;
  --warning: #FF9F0A;
  --radius: 8px;
  --font: -apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", sans-serif;
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-5: 24px;
  --space-6: 32px;
  --touch-min: 44px;
  --density: comfortable;
}
```

## Interaction principles

- focus-visible must exist for keyboard users
- primary interactive targets should be at least 44x44
- button hierarchy must distinguish primary from secondary actions
- empty/loading/error/success states must be intentional and explicit
- Keyboard: Tab order matches visual order; Escape closes overlays.
- Loading: skeletons or explicit progress; never blank freeze without explanation.
- Navigation: surface-native (top nav + content column).
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
- One primary CTA per viewport
- Quiet neutrals + single accent
- Product imagery or content first; chrome recedes
- Explicit empty/loading/error/success

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
- Purple-to-blue AI gradients
- Multiple competing primaries
- Glassmorphism decoration
- Dense admin tables as the default public IA

## Accessibility & responsive

- Contrast: body text meets WCAG AA intent.
- Focus visibility: required.
- Touch/pointer targets: surface min `44px` (critical actions still prefer 44px).
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
