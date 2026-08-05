"""Search-boundary curation: ingest → awaiting_review → human decide → active.

Wire status remains `pending` for CLI compatibility; truth token is awaiting_review.
ADB does not crawl the web. Proposals bind vertical profiles; VerticalGate
fail-closes complete drift before pending.
"""

from __future__ import annotations

import json
import re
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from .errors import DeliveryBusError
from .storage import Storage

VALID_DECISIONS = frozenset({"approve", "reject"})
VALID_STATUSES = frozenset({"pending", "approved", "rejected"})

DEFAULT_PROJECT_PROFILE_REF = "proj-adb-oss-picks"
DEFAULT_ACCOUNT_PROFILE_REF = "acct-kushi-gzh"

_OFF_VERTICAL_RE = re.compile(
    r"表情包|贴纸合集|情侣|闺蜜|宠物|情感漫|航司|打工人情绪|无字万能|热梗二创贴图",
    re.IGNORECASE,
)


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _profiles_root() -> Path:
    # src/agent_delivery_bus/boundary.py → repo root
    return Path(__file__).resolve().parents[2] / "fixtures" / "vertical-profiles"


def load_vertical_profile(ref: str) -> dict[str, Any]:
    """Load an auditable vertical profile by id or path."""
    ref = (ref or "").strip()
    if not ref:
        raise DeliveryBusError(
            "boundary_profile_ref_required",
            "vertical profile ref is required",
            resume_action="pass --project-profile-ref and --account-profile-ref for 自媒体",
        )
    root = _profiles_root()
    candidates = [
        Path(ref),
        root / ref,
        root / f"{ref}.json",
    ]
    if "/" not in ref and not ref.endswith(".json"):
        candidates.append(root / f"{ref}.json")
    for path in candidates:
        if path.is_file():
            data = json.loads(path.read_text(encoding="utf-8"))
            if not isinstance(data, dict) or not data.get("id"):
                raise DeliveryBusError(
                    "boundary_profile_invalid",
                    f"profile {ref} missing id",
                )
            data["_path"] = str(path)
            return data
    raise DeliveryBusError(
        "boundary_profile_not_found",
        f"vertical profile not found: {ref}",
        resume_action=f"place profile under {root}",
    )


def vertical_gate(
    *,
    topic: str,
    query_hints: list[str],
    rationale: str,
    project_profile: dict[str, Any],
    account_profile: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """Fail-closed gate: complete vertical drift cannot enter awaiting_review."""
    blob = " ".join(
        [
            topic or "",
            " ".join(query_hints or []),
            rationale or "",
        ]
    ).lower()
    excluded: list[str] = []
    excluded.extend(str(x) for x in (project_profile.get("must_exclude") or []))
    if account_profile:
        excluded.extend(str(x) for x in (account_profile.get("out_of_scope") or []))
    hit = [token for token in excluded if token and token.lower() in blob]
    if not hit:
        matched = _OFF_VERTICAL_RE.search(blob)
        if matched:
            hit = [matched.group(0)]
    if hit:
        return {
            "allow": False,
            "reason_code": "vertical_gate_rejected",
            "hits": hit,
            "resume_action": "rewrite topic inside project/account vertical or reject",
        }

    must_include = [str(x).lower() for x in (project_profile.get("must_include") or []) if str(x).strip()]
    in_vertical = any(token in blob for token in must_include) if must_include else True
    has_value = bool((rationale or "").strip()) and len((rationale or "").strip()) >= 4
    if not in_vertical and not has_value:
        return {
            "allow": False,
            "reason_code": "vertical_gate_rejected",
            "hits": ["missing_in_vertical_signal"],
            "resume_action": "add in-vertical keywords or a meaningful value rationale",
        }
    return {"allow": True, "reason_code": "", "hits": []}


class BoundaryService:
    def __init__(self, storage: Storage):
        self.storage = storage

    def ingest(
        self,
        *,
        topic: str,
        query_hints: list[str] | None = None,
        sources: list[str] | None = None,
        rationale: str = "",
        project_profile_ref: str = "",
        account_profile_ref: str = "",
        provenance: str = "in-vertical-fixture",
        require_account_profile: bool = True,
        auto_activate: bool = False,
    ) -> dict[str, Any]:
        topic = (topic or "").strip()
        if not topic:
            raise DeliveryBusError(
                "boundary_topic_required",
                "boundary ingest requires --topic",
                resume_action="pass a non-empty --topic",
            )
        if auto_activate:
            raise DeliveryBusError(
                "illegal_boundary_auto_activate",
                "ingest must land in pending; auto-activate is illegal",
                resume_action="call `adb boundary decide --decision approve` after review",
            )

        project_ref = (project_profile_ref or "").strip()
        account_ref = (account_profile_ref or "").strip()
        if not project_ref:
            raise DeliveryBusError(
                "boundary_profile_ref_required",
                "ingest requires --project-profile-ref",
                resume_action=f"pass --project-profile-ref {DEFAULT_PROJECT_PROFILE_REF}",
            )
        if require_account_profile and not account_ref:
            raise DeliveryBusError(
                "boundary_profile_ref_required",
                "自媒体 ingest requires --account-profile-ref",
                resume_action=f"pass --account-profile-ref {DEFAULT_ACCOUNT_PROFILE_REF}",
            )

        project_profile = load_vertical_profile(project_ref)
        account_profile = load_vertical_profile(account_ref) if account_ref else None
        hints = [str(q).strip() for q in (query_hints or []) if str(q).strip()]
        gate = vertical_gate(
            topic=topic,
            query_hints=hints,
            rationale=rationale,
            project_profile=project_profile,
            account_profile=account_profile,
        )
        if not gate.get("allow"):
            raise DeliveryBusError(
                str(gate.get("reason_code") or "vertical_gate_rejected"),
                f"vertical gate rejected: {gate.get('hits')}",
                resume_action=str(gate.get("resume_action") or "rewrite in-vertical"),
            )

        proposal = {
            "id": f"sbp-{uuid.uuid4().hex[:12]}",
            "topic": topic,
            "query_hints": hints,
            "sources": [str(s).strip() for s in (sources or []) if str(s).strip()],
            "rationale": (rationale or "").strip(),
            "project_profile_ref": project_profile.get("id") or project_ref,
            "account_profile_ref": (account_profile or {}).get("id") if account_profile else account_ref,
            "provenance": (provenance or "in-vertical-fixture").strip() or "in-vertical-fixture",
            "status": "pending",
            "created_at": now_iso(),
            "updated_at": now_iso(),
            "decided_at": "",
            "actor": "",
            "decision_note": "",
        }
        return self.storage.upsert_boundary_proposal(proposal)

    def pending(self) -> list[dict[str, Any]]:
        return self.storage.list_boundary_proposals(status="pending")

    def show(self, proposal_id: str) -> dict[str, Any]:
        row = self.storage.get_boundary_proposal(proposal_id)
        if row is None:
            raise DeliveryBusError(
                "boundary_not_found",
                f"no boundary proposal {proposal_id}",
                resume_action="run `adb boundary pending`",
            )
        return row

    def list(self, *, status: str = "approved") -> list[dict[str, Any]]:
        status = (status or "approved").strip().lower()
        if status == "all":
            return self.storage.list_boundary_proposals(status=None)
        if status not in VALID_STATUSES:
            raise DeliveryBusError(
                "boundary_status_invalid",
                f"status must be one of {sorted(VALID_STATUSES)}|all",
            )
        return self.storage.list_boundary_proposals(status=status)

    def decide(
        self,
        proposal_id: str,
        *,
        actor: str,
        decision: str,
        note: str = "",
    ) -> dict[str, Any]:
        actor = (actor or "").strip()
        decision = (decision or "").strip().lower()
        if not actor:
            raise DeliveryBusError("boundary_actor_required", "decide requires --actor")
        if decision not in VALID_DECISIONS:
            raise DeliveryBusError(
                "boundary_decision_invalid",
                "decision must be approve|reject",
            )
        row = self.show(proposal_id)
        if row.get("status") != "pending":
            raise DeliveryBusError(
                "boundary_already_decided",
                f"proposal {proposal_id} is already {row.get('status')}",
                resume_action="inspect with `adb boundary show`",
            )
        new_status = "approved" if decision == "approve" else "rejected"
        updated = self.storage.update_boundary_proposal(
            proposal_id,
            status=new_status,
            actor=actor,
            decision_note=note,
            decided_at=now_iso(),
        )
        self.storage.append_boundary_decision(
            {
                "decision_id": f"sbd-{uuid.uuid4().hex[:12]}",
                "proposal_id": proposal_id,
                "actor": actor,
                "decision": decision,
                "note": note,
                "created_at": now_iso(),
            }
        )
        return updated

    def reject_illegal(self, *, action: str) -> dict[str, Any]:
        action = (action or "").strip().lower()
        if action in {"auto_approve", "activate_skip_pending", "activate_skip_awaiting", "ingest_active"}:
            return {
                "status": "blocked",
                "blocked": True,
                "reason_code": "illegal_boundary_transition",
                "resume_action": "use ingest→pending→decide(approve|reject)",
                "attempted_action": action,
            }
        raise DeliveryBusError("boundary_action_unknown", f"unknown action={action}")

    def run_tick_fixture(self, *, proposals: list[dict[str, Any]] | None = None) -> dict[str, Any]:
        """Simulate Hermes tick: ingest only, never approve."""
        seeded = proposals or kushi_daily_topic_batch()
        created = [self.ingest(**item) for item in seeded]
        assert all(item["status"] == "pending" for item in created)
        assert all(item.get("project_profile_ref") for item in created)
        assert all(item.get("account_profile_ref") for item in created)
        return {"ingested": created, "auto_approved": False}


# 库拾 · GitHub 开源 AI / AI Spec 向日批题库（按日轮转取 5 条；不爬网、不 auto-approve）
KUSHI_TOPIC_BANK: list[dict[str, Any]] = [
    {
        "topic": "本周值得盯的 GitHub 开源 AI Agent 框架更新",
        "query_hints": ["github ai agent", "开源 agent framework", "llm agent release"],
        "rationale": "库拾·AI Spec 贴图｜给建造者可执行的开源雷达",
    },
    {
        "topic": "把一条 AI Spec 画成信息图：输入/工具/护栏三块怎么拆",
        "query_hints": ["ai spec diagram", "agent spec 信息图", "tool guardrail"],
        "rationale": "库拾·贴图=image_post｜把规范变成可转发图",
    },
    {
        "topic": "开源 LLM Ops 小工具：评测/追踪/成本一眼看懂",
        "query_hints": ["llm ops opensource", "eval tracing cost", "github llm toolkit"],
        "rationale": "库拾·oss-picks｜运维向实用开源清单",
    },
    {
        "topic": "从 README 到可复现：开源 AI 库最小跑通清单",
        "query_hints": ["reproducible ai repo", "github quickstart", "oss onboarding"],
        "rationale": "库拾·开源 AI 库｜降低读者上手摩擦",
    },
    {
        "topic": "Agent 工具调用失败怎么写进 Spec：错误码与重试边界",
        "query_hints": ["agent tool error spec", "retry boundary", "ai spec failure"],
        "rationale": "库拾·AI Spec｜把失败路径画清楚",
    },
    {
        "topic": "多模型路由开源方案对比：何时该切便宜模型",
        "query_hints": ["multi model router", "github llm router", "cost aware routing"],
        "rationale": "库拾·oss-picks｜成本与质量权衡图",
    },
    {
        "topic": "开源 RAG 管线一周进展：切片、召回、引用三件套",
        "query_hints": ["opensource rag", "github retrieval", "citation pipeline"],
        "rationale": "库拾·开源 AI｜检索链路可视化贴图",
    },
    {
        "topic": "把 MCP / Tool Schema 画成人话：字段、权限、副作用",
        "query_hints": ["mcp schema", "tool schema spec", "agent permission"],
        "rationale": "库拾·AI Spec 贴图｜协议层可读化",
    },
    {
        "topic": "GitHub 上的 Prompt/Eval 数据集：怎么挑、怎么标注边界",
        "query_hints": ["prompt eval dataset", "github benchmark", "annotation boundary"],
        "rationale": "库拾·oss-picks｜数据集选型信息图",
    },
    {
        "topic": "本地可跑的开源推理栈：量化、显存、吞吐一张图说清",
        "query_hints": ["local llm inference", "quantization vram", "opensource runtime"],
        "rationale": "库拾·开源 AI 库｜硬件约束可视化",
    },
]


def kushi_daily_topic_batch(*, day_index: int | None = None, count: int = 5) -> list[dict[str, Any]]:
    """Pick `count` rotating in-vertical topics for 库拾 WeChat image_post editorial."""
    if day_index is None:
        day_index = datetime.now(timezone.utc).timetuple().tm_yday
    bank = KUSHI_TOPIC_BANK
    if not bank:
        return []
    start = int(day_index) % len(bank)
    picked: list[dict[str, Any]] = []
    for offset in range(max(1, int(count))):
        item = dict(bank[(start + offset) % len(bank)])
        item["sources"] = ["kushi://wechat-gzh/image_post", "editorial://daily-batch"]
        item["project_profile_ref"] = DEFAULT_PROJECT_PROFILE_REF
        item["account_profile_ref"] = DEFAULT_ACCOUNT_PROFILE_REF
        item["provenance"] = "in-vertical-fixture"
        picked.append(item)
    return picked


def hermes_boundary_tick_script() -> str:
    return r"""#!/bin/bash
# Hermes cron tick → 库拾公众号 AI Spec/开源选题 · 每日 5 条进 ADB 待审（不 auto-approve）
# stdout 交给 Hermes --no-agent 直推飞书
set -euo pipefail
ADB_BIN="${ADB_BIN:-adb}"
SLUG="${1:-search-boundary-curate}"
DAY_INDEX="${DAY_INDEX:-}"
FORCE_RUN="${FORCE_RUN:-0}"
PROJECT_PROFILE_REF="${PROJECT_PROFILE_REF:-proj-adb-oss-picks}"
ACCOUNT_PROFILE_REF="${ACCOUNT_PROFILE_REF:-acct-kushi-gzh}"

if [[ "$FORCE_RUN" != "1" ]] && "$ADB_BIN" schedule show "$SLUG" --json >/dev/null 2>&1; then
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
first_id = (ingested[0].get("id") if ingested else None) or "<id>"
lines = [
    f"下一步：对第 1 条拍板（约 1 分钟）",
    f"adb boundary decide {first_id} --actor you --decision approve --json",
    "",
    f"库拾选题 · 今日 {len(ingested)} 条 · {today}",
    "状态：均为 awaiting_review · 未生效",
    "",
]
for i, row in enumerate(ingested[:5], 1):
    lines.append(f"{i}. {row.get('topic') or '(no topic)'}")
    lines.append(f"   id={row.get('id') or '-'}")
lines.extend(["", "下一步：approve 或 reject 任意一条 id"])
print("\n".join(lines))
PY
"""
