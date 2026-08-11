#!/bin/bash
# pi-beacon installer: merge pi settings skills and copy the ADB/Prism bridge.
# Idempotent; supports --dry-run and PRISM_SKILLS_DIR override.
set -euo pipefail

DRY_RUN=0
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
  esac
done

AGENT_DIR="${PI_CODING_AGENT_DIR:-$HOME/.pi/agent}"
SETTINGS="$AGENT_DIR/settings.json"
EXT_DIR="$AGENT_DIR/extensions"
EXT_SRC="$(cd "$(dirname "$0")" && pwd)/extension.ts"
PRISM_SKILLS="${PRISM_SKILLS_DIR:-$HOME/Developer/Personal/products/prism/skills}"

echo "pi-beacon install (dry-run=$DRY_RUN)"
echo "  settings=$SETTINGS"
echo "  extension=$EXT_DIR/adb-bridge.ts"

run() {
  if [ "$DRY_RUN" = "1" ]; then
    echo "  [dry-run] $*"
  else
    "$@"
  fi
}

if [ ! -f "$SETTINGS" ]; then
  run mkdir -p "$AGENT_DIR"
  run python3 - "$SETTINGS" <<'PY'
import json, sys
path=sys.argv[1]
open(path,"w",encoding="utf-8").write(json.dumps({"skills": []}, ensure_ascii=False, indent=2)+"\n")
PY
fi

run python3 - "$SETTINGS" "$HOME/.codex/skills" "$HOME/.agents/skills" "$PRISM_SKILLS" <<'PY'
import json, sys, os
path, *roots = sys.argv[1:]
data=json.load(open(path, encoding="utf-8"))
skills=list(data.get("skills") or [])
for root in roots:
    root=os.path.expanduser(root)
    if os.path.isdir(root) and root not in skills:
        skills.append(root)
data["skills"]=skills
open(path,"w",encoding="utf-8").write(json.dumps(data, ensure_ascii=False, indent=2)+"\n")
PY

run mkdir -p "$EXT_DIR"
if [ "$DRY_RUN" = "0" ] && [ -f "$EXT_SRC" ]; then
  cp "$EXT_SRC" "$EXT_DIR/adb-bridge.ts"
  echo "  installed extension -> $EXT_DIR/adb-bridge.ts"
fi

echo "done. Reload pi (/reload) or restart to activate."
