"""Schedule heartbeat layer: register / should-run / quota / evidence ledger.

ADB does not embed a cron daemon. Hermes cron (or CLI) triggers; ADB decides
whether a registered entry may run, records quota after validated evidence, and
keeps an append-only heartbeat dispatch ledger.
"""

from __future__ import annotations

import json
import uuid
from datetime import datetime, timezone
from typing import Any, Callable

from .errors import DeliveryBusError
from .storage import Storage

ALLOWED_ENGINES = frozenset({"hermes"})
QUOTA_SOURCES = frozenset({"heartbeat", "controller"})
ILLEGAL_HEARTBEAT_ACTIONS = frozenset({"dispatch", "approve", "auto_dispatch", "auto_approve"})

# Spend-after-validated-writeback: quota only after evidence refs are present.
REQUIRED_EVIDENCE_KEYS = ("evidence_refs",)


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _window_day(ts: str | None = None) -> str:
    raw = ts or now_iso()
    return raw[:10]


class ScheduleService:
    def __init__(
        self,
        storage: Storage,
        *,
        health_probe: Callable[[dict[str, Any]], bool] | None = None,
        evidence_validator: Callable[[list[str]], bool] | None = None,
    ):
        self.storage = storage
        self.health_probe = health_probe or (lambda entry: str(entry.get("health") or "healthy") == "healthy")
        self.evidence_validator = evidence_validator or (lambda refs: bool(refs) and all(str(r).strip() for r in refs))

    def register(
        self,
        *,
        slug: str,
        command: str,
        engine: str,
        cron_expr: str,
        quota_limit: int,
        health: str = "healthy",
    ) -> dict[str, Any]:
        slug = (slug or "").strip()
        command = (command or "").strip()
        engine = (engine or "").strip().lower()
        cron_expr = (cron_expr or "").strip()
        if not slug:
            raise DeliveryBusError("schedule_slug_required", "schedule register requires --slug")
        if not command:
            raise DeliveryBusError("schedule_command_required", "schedule register requires --command")
        if engine not in ALLOWED_ENGINES:
            raise DeliveryBusError(
                "schedule_engine_unknown",
                f"engine '{engine}' is not registered; allowed={sorted(ALLOWED_ENGINES)}",
                resume_action="use --engine hermes",
            )
        if not cron_expr:
            raise DeliveryBusError("schedule_cron_required", "schedule register requires --cron")
        if quota_limit < 0:
            raise DeliveryBusError("schedule_quota_invalid", "quota-limit must be >= 0")
        entry = {
            "slug": slug,
            "command": command,
            "engine": engine,
            "cron_expr": cron_expr,
            "quota_limit": int(quota_limit),
            "health": health if health in {"healthy", "unhealthy"} else "healthy",
            "updated_at": now_iso(),
        }
        stored = self.storage.upsert_schedule_entry(entry)
        self.storage.ensure_quota_ledger(slug, window=_window_day(), slots_allowed=int(quota_limit))
        return stored

    def list_entries(self) -> list[dict[str, Any]]:
        rows = self.storage.list_schedule_entries()
        return [self._with_quota(row) for row in rows]

    def show(self, slug: str) -> dict[str, Any]:
        entry = self.storage.get_schedule_entry(slug)
        if entry is None:
            raise DeliveryBusError(
                "schedule_entry_not_found",
                f"no schedule entry for slug={slug}",
                resume_action="run `adb schedule register` first",
            )
        return self._with_quota(entry)

    def should_run(self, slug: str) -> dict[str, Any]:
        """Deterministic guard chain: quota gate → health gate. No LLM."""
        try:
            entry = self.show(slug)
        except DeliveryBusError as exc:
            return {
                "action": "blocked",
                "status": "blocked",
                "blocked": True,
                "reason_code": exc.reason_code,
                "resume_action": exc.resume_action,
                "fsm_state": "blocked",
                "entry_slug": slug,
            }

        quota = entry.get("quota") or {}
        spent = int(quota.get("slots_spent") or 0)
        allowed = int(quota.get("slots_allowed") or entry.get("quota_limit") or 0)
        if spent >= allowed:
            return {
                "action": "blocked",
                "status": "throttled",
                "blocked": True,
                "reason_code": "quota_exhausted",
                "resume_action": "raise --quota-limit or wait for next window / human override",
                "fsm_state": "blocked",
                "entry": entry,
            }

        if not self.health_probe(entry):
            return {
                "action": "blocked",
                "status": "blocked",
                "blocked": True,
                "reason_code": "health_gate_failed",
                "resume_action": "restore entry health then retry should-run",
                "fsm_state": "blocked",
                "entry": entry,
            }

        return {
            "action": "run",
            "status": "pass",
            "blocked": False,
            "reason_code": "",
            "resume_action": "",
            "fsm_state": "checking",
            "entry": entry,
        }

    def begin_run(self, slug: str, *, source: str) -> dict[str, Any]:
        """Start a heartbeat run only after should-run pass. Illegal if skipped."""
        source = (source or "").strip().lower()
        if source not in QUOTA_SOURCES:
            raise DeliveryBusError(
                "schedule_source_not_allowed",
                f"source '{source}' not in whitelist {sorted(QUOTA_SOURCES)}",
                resume_action="use source=heartbeat or source=controller",
            )
        decision = self.should_run(slug)
        if decision.get("action") != "run":
            raise DeliveryBusError(
                str(decision.get("reason_code") or "should_run_blocked"),
                "should-run did not allow execution",
                resume_action=str(decision.get("resume_action") or ""),
                data=decision,
            )
        run = {
            "run_id": f"hbr-{uuid.uuid4().hex[:12]}",
            "entry_slug": slug,
            "source": source,
            "status": "running",
            "evidence_refs": [],
            "quota_spent": 0,
            "created_at": now_iso(),
            "updated_at": now_iso(),
        }
        return self.storage.append_heartbeat_run(run)

    def reject_illegal(self, *, action: str, from_state: str = "checking") -> dict[str, Any]:
        """Heartbeat must not auto approve/dispatch (AC-FLY-007 / illegal transitions)."""
        action = (action or "").strip().lower()
        if action in ILLEGAL_HEARTBEAT_ACTIONS or action in {"dispatch", "approve"}:
            return {
                "status": "blocked",
                "blocked": True,
                "reason_code": "illegal_heartbeat_action",
                "resume_action": "heartbeat may only report/account; dispatch/approve via explicit operator path",
                "fsm_state": "blocked",
                "from_state": from_state,
                "attempted_action": action,
            }
        raise DeliveryBusError("schedule_action_unknown", f"unknown action={action}")

    def assert_not_skip_should_run(self, *, skipped_should_run: bool) -> None:
        if skipped_should_run:
            raise DeliveryBusError(
                "illegal_skip_should_run",
                "idle → running without should-run is illegal",
                resume_action="call `adb schedule should-run <slug>` before execution",
            )

    def complete_run(
        self,
        run_id: str,
        *,
        evidence_refs: list[str] | None = None,
        force_status: str | None = None,
    ) -> dict[str, Any]:
        """Close a heartbeat run; spend quota only after validated evidence writeback."""
        run = self.storage.get_heartbeat_run(run_id)
        if run is None:
            raise DeliveryBusError("heartbeat_run_not_found", f"no heartbeat run {run_id}")
        refs = list(evidence_refs if evidence_refs is not None else (run.get("evidence_refs") or []))
        if force_status == "failed":
            return self.storage.update_heartbeat_run(
                run_id,
                status="blocked",
                evidence_refs=refs,
                quota_spent=0,
                reason_code="heartbeat_run_failed",
            )

        if not self.evidence_validator(refs):
            return self.storage.update_heartbeat_run(
                run_id,
                status="reconciling",
                evidence_refs=refs,
                quota_spent=0,
                reason_code="truth_evidence_incomplete",
            )

        # spend-after-validated-writeback
        spent = self.storage.spend_quota_slot(run["entry_slug"], window=_window_day(), slots=1)
        return self.storage.update_heartbeat_run(
            run_id,
            status="completed",
            evidence_refs=refs,
            quota_spent=int(spent.get("spent_this_call") or 1),
            reason_code="",
            quota_snapshot=spent,
        )

    def reconcile_run(self, run_id: str, *, evidence_refs: list[str] | None = None) -> dict[str, Any]:
        """Reuse truth-gate style closure: missing evidence stays reconciling."""
        run = self.storage.get_heartbeat_run(run_id)
        if run is None:
            raise DeliveryBusError("heartbeat_run_not_found", f"no heartbeat run {run_id}")
        refs = list(evidence_refs if evidence_refs is not None else (run.get("evidence_refs") or []))
        if run.get("status") == "completed":
            return {"status": "completed", "blocked": False, "run": run}
        if not self.evidence_validator(refs):
            updated = self.storage.update_heartbeat_run(
                run_id,
                status="reconciling",
                evidence_refs=refs,
                quota_spent=0,
                reason_code="truth_evidence_incomplete",
            )
            return {
                "status": "reconciling",
                "blocked": True,
                "reason_code": "truth_evidence_incomplete",
                "resume_action": "attach evidence refs then reconcile again",
                "run": updated,
            }
        completed = self.complete_run(run_id, evidence_refs=refs)
        return {"status": completed.get("status"), "blocked": completed.get("status") != "completed", "run": completed}

    def ledger(self, *, slug: str | None = None) -> list[dict[str, Any]]:
        return self.storage.list_heartbeat_runs(entry_slug=slug)

    def _with_quota(self, entry: dict[str, Any]) -> dict[str, Any]:
        window = _window_day()
        ledger = self.storage.ensure_quota_ledger(
            entry["slug"],
            window=window,
            slots_allowed=int(entry.get("quota_limit") or 0),
        )
        out = dict(entry)
        out["quota"] = {
            "window": ledger.get("window"),
            "slots_spent": int(ledger.get("slots_spent") or 0),
            "slots_allowed": int(ledger.get("slots_allowed") or 0),
            "next_eligible_at": ledger.get("next_eligible_at") or "",
            "remaining": max(0, int(ledger.get("slots_allowed") or 0) - int(ledger.get("slots_spent") or 0)),
        }
        return out


def hermes_cron_tick_script() -> str:
    """Fixture template isomorphic with ops-digest-cron (trigger outside ADB)."""
    return """#!/bin/bash
# Hermes cron tick → ADB should-run gate (no embedded ADB daemon)
set -euo pipefail
SLUG="${1:?slug required}"
adb schedule should-run "$SLUG" --json | grep -q '"action": "run"' || exit 0
# Operator/controller path executes command; heartbeat itself must not auto-dispatch.
"""


def hermes_reconcile_tick_script() -> str:
    """Hermes cron fixture: reconcile all pending dispatches on a timer.

    ADB itself delivers terminal results back to the originating channel
    (ChannelAdapter), so the cron script keeps stdout empty (silent job) and
    writes diagnostics to a local log. Register with:

    hermes cron create "every 1m" --name adb-reconcile --no-agent \
      --script adb-reconcile-tick.sh
    """
    return """#!/bin/bash
# Hermes cron tick → ADB reconcile one-pass.
# Results are delivered back to the originating channel by ADB itself,
# so this script stays silent (empty stdout) and only logs diagnostics.
set -euo pipefail
LOG="${ADB_RECONCILE_LOG:-$HOME/.adb/reconcile-loop.log}"
mkdir -p "$(dirname "$LOG")"
{
  echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] adb reconcile-loop --once"
  adb reconcile-loop --once --interval 0
} >>"$LOG" 2>&1
"""
