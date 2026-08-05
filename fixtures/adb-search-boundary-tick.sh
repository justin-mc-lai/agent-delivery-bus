#!/bin/bash
# Hermes cron tick → 示例号公众号 AI Spec/开源选题 · 每日 5 条进 ADB 待审（不 auto-approve）
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
python3 - <<'PY'
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

topics = []
try:
    from agent_delivery_bus.boundary import daily_topic_batch
    topics = daily_topic_batch(day_index=day_index, count=5)
except Exception:
    topics = [
        {
            "topic": "本周值得盯的 GitHub 开源 AI Agent 框架更新",
            "query_hints": ["github ai agent", "开源 agent framework"],
            "sources": ["demo://wechat-gzh/image_post"],
            "rationale": "示例号·AI Spec 贴图｜开源雷达",
            "project_profile_ref": project_ref,
            "account_profile_ref": account_ref,
            "provenance": "in-vertical-fixture",
        },
        {
            "topic": "把一条 AI Spec 画成信息图：输入/工具/护栏",
            "query_hints": ["ai spec diagram", "agent spec 信息图"],
            "sources": ["demo://wechat-gzh/image_post"],
            "rationale": "示例号·image_post｜规范可视化",
            "project_profile_ref": project_ref,
            "account_profile_ref": account_ref,
            "provenance": "in-vertical-fixture",
        },
        {
            "topic": "开源 LLM Ops 小工具：评测/追踪/成本",
            "query_hints": ["llm ops opensource", "github llm toolkit"],
            "sources": ["demo://wechat-gzh/image_post"],
            "rationale": "示例号·oss-picks｜实用开源清单",
            "project_profile_ref": project_ref,
            "account_profile_ref": account_ref,
            "provenance": "in-vertical-fixture",
        },
        {
            "topic": "从 README 到可复现：开源 AI 库最小跑通清单",
            "query_hints": ["reproducible ai repo", "github quickstart"],
            "sources": ["demo://wechat-gzh/image_post"],
            "rationale": "示例号·开源 AI 库｜上手摩擦↓",
            "project_profile_ref": project_ref,
            "account_profile_ref": account_ref,
            "provenance": "in-vertical-fixture",
        },
        {
            "topic": "Agent 工具调用失败怎么写进 Spec",
            "query_hints": ["agent tool error spec", "retry boundary"],
            "sources": ["demo://wechat-gzh/image_post"],
            "rationale": "示例号·AI Spec｜失败路径可视化",
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
    f"示例号选题 · 今日 {len(ingested)} 条 · {today}",
    "状态：均为 awaiting_review · 未生效",
    "",
]
for i, row in enumerate(ingested[:5], 1):
    lines.append(f"{i}. {row.get('topic') or '(no topic)'}")
    lines.append(f"   id={row.get('id') or '-'}")
lines.extend(["", "下一步：approve 或 reject 任意一条 id"])
print("\n".join(lines))
PY
