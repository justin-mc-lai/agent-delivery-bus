#!/bin/bash
# Hermes cron tick → 内容号 AI Spec/开源选题 · 每日 5 条进 ADB 待审（不 auto-approve）
# stdout 交给 Hermes --no-agent 直推飞书
set -euo pipefail
ADB_BIN="${ADB_BIN:-adb}"
SLUG="${1:-search-boundary-curate}"
DAY_INDEX="${DAY_INDEX:-}"
FORCE_RUN="${FORCE_RUN:-0}"
PROJECT_PROFILE_REF="${PROJECT_PROFILE_REF:-proj-demo}"
ACCOUNT_PROFILE_REF="${ACCOUNT_PROFILE_REF:-acct-demo-gzh}"

if [[ "$FORCE_RUN" != "1" ]] && "$ADB_BIN" schedule show "$SLUG" --json >/dev/null 2>&1; then
  "$ADB_BIN" schedule should-run "$SLUG" --json | grep -q '"action": "run"' || exit 0
fi

export ADB_BIN SLUG DAY_INDEX PROJECT_PROFILE_REF ACCOUNT_PROFILE_REF

# Resolve a Python that can import agent_delivery_bus. Cron runs under launchd,
# whose PATH puts the Hermes venv first; that venv has NO adb package and would
# silently fall back to hardcoded topics (the daily-duplicate bug).
PYTHON_BIN="${PYTHON_BIN:-}"
if [ -z "$PYTHON_BIN" ]; then
  for cand in /usr/bin/python3 python3; do
    if "$cand" -c "import agent_delivery_bus" >/dev/null 2>&1; then
      PYTHON_BIN="$cand"
      break
    fi
  done
fi
if [ -z "$PYTHON_BIN" ]; then
  echo "ERROR: no python3 with agent_delivery_bus; refusing fallback topics" >&2
  exit 1
fi

"$PYTHON_BIN" - <<'PY'
import json, os, subprocess, sys
from datetime import datetime, timezone

adb = os.environ.get("ADB_BIN", "adb")
project_ref = os.environ.get("PROJECT_PROFILE_REF", "proj-demo")
account_ref = os.environ.get("ACCOUNT_PROFILE_REF", "acct-demo-gzh")
day_index = os.environ.get("DAY_INDEX") or None
if day_index is not None and str(day_index).strip():
    day_index = int(day_index)
else:
    day_index = datetime.now(timezone.utc).timetuple().tm_yday

try:
    from agent_delivery_bus.boundary import daily_topic_batch
    topics = daily_topic_batch(day_index=day_index, count=5)
except Exception as exc:  # noqa: BLE001 - fail closed, never repeat stale topics
    import traceback

    traceback.print_exc()
    print(
        f"ERROR: daily_topic_batch failed ({exc!r}); "
        "refusing to send fallback duplicates",
        file=sys.stderr,
    )
    sys.exit(1)

ingested = []
for item in topics:
    cmd = [
        adb, "boundary", "ingest",
        "--topic", item["topic"],
        "--rationale", item.get("rationale") or "",
        "--project-profile-ref", item.get("project_profile_ref") or project_ref,
        "--account-profile-ref", item.get("account_profile_ref") or account_ref,
        "--provenance", item.get("provenance") or "in-vertical-fixture",
        "--json",
    ]
    for q in item.get("query_hints") or []:
        cmd.extend(["--query", q])
    for s in item.get("sources") or ["demo://wechat-gzh/image_post"]:
        cmd.extend(["--source", s])
    proc = subprocess.run(cmd, capture_output=True, text=True)
    if proc.returncode != 0:
        print(proc.stdout or proc.stderr or f"ingest failed: {item['topic']}", file=sys.stderr)
        sys.exit(proc.returncode or 1)
    try:
        payload = json.loads(proc.stdout)
        row = (payload.get("data") or {}).get("proposal") or (payload.get("data") or {})
    except Exception:
        row = {"topic": item["topic"], "id": "?", "raw": proc.stdout.strip()}
    ingested.append(row)

today = datetime.now().strftime("%Y-%m-%d")
first_id = (ingested[0].get("id") if ingested else None) or "<id>"
lines = [
    f"下一步：对第 1 条拍板（约 1 分钟）",
    f"adb boundary decide {first_id} --actor you --decision approve --json",
    "",
    f"每日选题 · 今日 {len(ingested)} 条 · {today}",
    "状态：均为 awaiting_review · 未生效",
    "",
]
for i, row in enumerate(ingested[:5], 1):
    lines.append(f"{i}. {row.get('topic') or '(no topic)'}")
    lines.append(f"   id={row.get('id') or '-'}")
lines.extend(["", "下一步：approve 或 reject 任意一条 id"])
print("\n".join(lines))
PY
