#!/bin/bash
# Hermes cron tick → 库拾公众号 AI Spec/开源选题 · 每日 5 条进 ADB 待审（不 auto-approve）
# stdout 交给 Hermes --no-agent 直推飞书
set -euo pipefail
ADB_BIN="${ADB_BIN:-adb}"
SLUG="${1:-search-boundary-curate}"
DAY_INDEX="${DAY_INDEX:-}"
PROJECT_PROFILE_REF="${PROJECT_PROFILE_REF:-proj-adb-oss-picks}"
ACCOUNT_PROFILE_REF="${ACCOUNT_PROFILE_REF:-acct-kushi-gzh}"

if "$ADB_BIN" schedule show "$SLUG" --json >/dev/null 2>&1; then
  "$ADB_BIN" schedule should-run "$SLUG" --json | grep -q '"action": "run"' || exit 0
fi

export ADB_BIN SLUG DAY_INDEX PROJECT_PROFILE_REF ACCOUNT_PROFILE_REF
python3 - <<'PY'
import json, os, subprocess, sys
from datetime import datetime, timezone

adb = os.environ.get("ADB_BIN", "adb")
project_ref = os.environ.get("PROJECT_PROFILE_REF", "proj-adb-oss-picks")
account_ref = os.environ.get("ACCOUNT_PROFILE_REF", "acct-kushi-gzh")
day_index = os.environ.get("DAY_INDEX") or None
if day_index is not None and str(day_index).strip():
    day_index = int(day_index)
else:
    day_index = datetime.now(timezone.utc).timetuple().tm_yday

topics = []
try:
    from agent_delivery_bus.boundary import kushi_daily_topic_batch
    topics = kushi_daily_topic_batch(day_index=day_index, count=5)
except Exception:
    topics = [
        {
            "topic": "本周值得盯的 GitHub 开源 AI Agent 框架更新",
            "query_hints": ["github ai agent", "开源 agent framework"],
            "sources": ["kushi://wechat-gzh/image_post"],
            "rationale": "库拾·AI Spec 贴图｜开源雷达",
            "project_profile_ref": project_ref,
            "account_profile_ref": account_ref,
            "provenance": "in-vertical-fixture",
        },
        {
            "topic": "把一条 AI Spec 画成信息图：输入/工具/护栏",
            "query_hints": ["ai spec diagram", "agent spec 信息图"],
            "sources": ["kushi://wechat-gzh/image_post"],
            "rationale": "库拾·image_post｜规范可视化",
            "project_profile_ref": project_ref,
            "account_profile_ref": account_ref,
            "provenance": "in-vertical-fixture",
        },
        {
            "topic": "开源 LLM Ops 小工具：评测/追踪/成本",
            "query_hints": ["llm ops opensource", "github llm toolkit"],
            "sources": ["kushi://wechat-gzh/image_post"],
            "rationale": "库拾·oss-picks｜实用开源清单",
            "project_profile_ref": project_ref,
            "account_profile_ref": account_ref,
            "provenance": "in-vertical-fixture",
        },
        {
            "topic": "从 README 到可复现：开源 AI 库最小跑通清单",
            "query_hints": ["reproducible ai repo", "github quickstart"],
            "sources": ["kushi://wechat-gzh/image_post"],
            "rationale": "库拾·开源 AI 库｜上手摩擦↓",
            "project_profile_ref": project_ref,
            "account_profile_ref": account_ref,
            "provenance": "in-vertical-fixture",
        },
        {
            "topic": "Agent 工具调用失败怎么写进 Spec",
            "query_hints": ["agent tool error spec", "retry boundary"],
            "sources": ["kushi://wechat-gzh/image_post"],
            "rationale": "库拾·AI Spec｜失败路径可视化",
            "project_profile_ref": project_ref,
            "account_profile_ref": account_ref,
            "provenance": "in-vertical-fixture",
        },
    ]

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
    for s in item.get("sources") or ["kushi://wechat-gzh/image_post"]:
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
lines = [
    f"库拾 · GitHub 开源 AI / AI Spec 选题（待审）· {today}",
    f"今日入库 {len(ingested)} 条 · 均为 awaiting_review · 请人工拍板后生效",
    "",
]
for i, row in enumerate(ingested, 1):
    lines.append(f"{i}. {row.get('topic') or '(no topic)'}")
    lines.append(f"   id={row.get('id') or '-'}")
lines.extend([
    "",
    "拍板命令：",
    "  adb boundary pending --json",
    "  adb boundary decide <id> --actor you --decision approve|reject --json",
    "  adb approvals awaiting --channel feishu --json",
])
print("\n".join(lines))
PY
