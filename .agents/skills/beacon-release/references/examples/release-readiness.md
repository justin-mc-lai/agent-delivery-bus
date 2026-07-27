# Beacon Skill Example: Release Readiness

This example focuses on **v1.4.6 CLI release evidence and gate consistency** (`beacon --version` → `Beacon CLI v1.4.6`).

Read first:

- [`../../../docs/beacon/v1.4.6/SUMMARY.md`](../../../docs/beacon/v1.4.6/SUMMARY.md)
- [`../../../docs/beacon/v1.4.6/execution/index.md`](../../../docs/beacon/v1.4.6/execution/index.md)
- [`../../../docs/beacon/v1.4.6/release/v1.4.6.md`](../../../docs/beacon/v1.4.6/release/v1.4.6.md)

```bash
# 1) Inspect current package state
beacon status --board --project-root . --version v1.4.6

# 2) Verify context and sync machine materials after runtime upgrades
beacon doctor verify-context \
  --project-root . \
  --strict \
  --json
beacon doctor verify-materials \
  --project-root . \
  --version v1.4.6 \
  --json
beacon doctor repair-package \
  --project-root . \
  --version v1.4.6 \
  --dry-run \
  --json
beacon doctor sync-materials \
  --project-root . \
  --version v1.4.6 \
  --all-features \
  --json

# 3) Ensure current feature has implementation-plan truth before release gate
beacon implement plan \
  "Beacon v1.4.6 release line" \
  --project . \
  --version v1.4.6 \
  --json

# 4) Run release gate and release readiness checks
beacon gate check release \
  --project-root . \
  --version v1.4.6 \
  --json
beacon release check v1.4.6 --project .

# 5) Confirm the fixed HUD hook can inject context into Codex
printf '%s' '{"hook_event_name":"UserPromptSubmit","prompt":"继续 Beacon v1.4.6 release"}' \
  | beacon debug codex-fixed-hud --project-root . --version v1.4.6 --json
```
