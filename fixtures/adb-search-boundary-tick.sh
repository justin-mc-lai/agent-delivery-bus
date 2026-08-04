#!/bin/bash
# Hermes cron tick → ADB search-boundary ingest (no auto-approve)
set -euo pipefail
ADB_BIN="${ADB_BIN:-adb}"
SLUG="${1:-search-boundary-curate}"
if "$ADB_BIN" schedule show "$SLUG" --json >/dev/null 2>&1; then
  "$ADB_BIN" schedule should-run "$SLUG" --json | grep -q '"action": "run"' || exit 0
fi
"$ADB_BIN" boundary ingest \
  --topic "scheduled frontier: agent tooling $(date -u +%Y-%m-%d)" \
  --query "agent delivery orchestration" \
  --query "personal knowledge search boundary" \
  --source "hermes-cron-fixture" \
  --rationale "scheduled network-search boundary sweep" \
  --json
