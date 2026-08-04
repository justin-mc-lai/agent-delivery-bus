"""Search-boundary curation: ingest → pending → human decide → active.

ADB does not crawl the web. Hermes cron/scripts propose boundaries; humans
approve or reject before a proposal becomes active.
"""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from typing import Any

from .errors import DeliveryBusError
from .storage import Storage

VALID_DECISIONS = frozenset({"approve", "reject"})
VALID_STATUSES = frozenset({"pending", "approved", "rejected"})


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


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
        proposal = {
            "id": f"sbp-{uuid.uuid4().hex[:12]}",
            "topic": topic,
            "query_hints": [str(q).strip() for q in (query_hints or []) if str(q).strip()],
            "sources": [str(s).strip() for s in (sources or []) if str(s).strip()],
            "rationale": (rationale or "").strip(),
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
        if action in {"auto_approve", "activate_skip_pending", "ingest_active"}:
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
        seeded = proposals or [
            {
                "topic": "AI agent delivery patterns 2026",
                "query_hints": ["agent delivery bus", "hermes cron orchestration"],
                "sources": ["fixture://web-frontier"],
                "rationale": "scheduled frontier sweep (fixture)",
            }
        ]
        created = [self.ingest(**item) for item in seeded]
        assert all(item["status"] == "pending" for item in created)
        return {"ingested": created, "auto_approved": False}


def hermes_boundary_tick_script() -> str:
    return """#!/bin/bash
# Hermes cron tick → ADB search-boundary ingest (no auto-approve)
set -euo pipefail
ADB_BIN="${ADB_BIN:-adb}"
SLUG="${1:-search-boundary-curate}"
if "$ADB_BIN" schedule show "$SLUG" --json >/dev/null 2>&1; then
  "$ADB_BIN" schedule should-run "$SLUG" --json | grep -q '"action": "run"' || exit 0
fi
"$ADB_BIN" boundary ingest \\
  --topic "scheduled frontier: agent tooling $(date -u +%Y-%m-%d)" \\
  --query "agent delivery orchestration" \\
  --query "personal knowledge search boundary" \\
  --source "hermes-cron-fixture" \\
  --rationale "scheduled network-search boundary sweep" \\
  --json
"""
