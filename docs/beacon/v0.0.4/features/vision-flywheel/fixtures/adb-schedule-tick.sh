#!/bin/bash
# Hermes cron tick → ADB should-run gate (no embedded ADB daemon)
# Isomorphic with ops-digest-cron: trigger outside ADB.
set -euo pipefail
SLUG="${1:?slug required}"
adb schedule should-run "$SLUG" --json | grep -q '"action": "run"' || exit 0
# Operator/controller path executes the registered command; heartbeat must not auto-dispatch.
