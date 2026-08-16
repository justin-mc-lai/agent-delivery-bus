"""Agent session registry: stable channel/actor -> target executor session mapping.

Session identity has three persistent axes plus one audit axis:
- channel_thread: where the message comes from and results go back to
- actor_id: who is approving (channel identity)
- target_executor + target_session: which agent (codex/claude/pi) executes and
  into which runnable session
- host_session: audit only (which hermes/agent session parsed the intent);
  it is intentionally NOT part of the identity key so a channel thread keeps
  the same binding across host-session restarts.
"""

from __future__ import annotations

import hashlib
from datetime import datetime, timedelta, timezone
from typing import Any

from .errors import DeliveryBusError
from .storage import Storage


DEFAULT_TTL_SECONDS = 24 * 3600
ALLOWED_TARGETS = ("codex", "claude", "pi", "coding")


def _now() -> str:
    return datetime.now(timezone.utc).isoformat()


def session_id_for(*, channel: str, channel_thread: str, actor_id: str, host_session: str = "") -> str:
    # host_session is deliberately excluded from the identity key: it is a
    # transient host-side context, not a durable conversation identity.
    # It remains stored on the binding record for audit purposes.
    del host_session
    canonical = "|".join(
        [
            str(channel or "").strip().lower(),
            str(channel_thread or "").strip(),
            str(actor_id or "").strip(),
        ]
    )
    return "sess_" + hashlib.sha256(canonical.encode("utf-8")).hexdigest()[:24]


def next_task_session(*, target_executor: str, seed: str) -> str:
    return f"{str(target_executor or 'agent').strip().lower()}-{hashlib.sha256(str(seed or '').encode('utf-8')).hexdigest()[:12]}"


class SessionRegistry:
    def __init__(self, storage: Storage, *, ttl_seconds: int = DEFAULT_TTL_SECONDS):
        self.storage = storage
        self.ttl_seconds = int(ttl_seconds)

    def bind(
        self,
        *,
        channel: str,
        channel_thread: str,
        actor_id: str = "",
        host_session: str = "",
        target_executor: str = "",
        target_session: str = "",
    ) -> dict[str, Any]:
        if not str(channel or "").strip() or not str(channel_thread or "").strip():
            raise DeliveryBusError(
                "session_identity_incomplete",
                "channel and channel_thread are required for session binding",
                resume_action="pass --channel and --thread",
            )
        target = str(target_executor or "").strip().lower()
        if target and target not in ALLOWED_TARGETS:
            raise DeliveryBusError(
                "session_target_unknown",
                f"unknown target executor: {target!r}",
                resume_action=f"use one of: {', '.join(sorted(ALLOWED_TARGETS))}",
            )
        sid = session_id_for(
            channel=channel,
            channel_thread=channel_thread,
            actor_id=actor_id,
            host_session=host_session,
        )
        now = _now()
        self.storage.conn.execute(
            """
            INSERT INTO agent_sessions(
              session_id,channel,channel_thread,actor_id,host_session,
              target_executor,target_session,state,last_seen_at,created_at,updated_at
            ) VALUES(?,?,?,?,?,?,?,'bound',?,?,?)
            ON CONFLICT(session_id) DO UPDATE SET
              target_executor=excluded.target_executor,
              target_session=excluded.target_session,
              state='bound',
              last_seen_at=excluded.last_seen_at,
              updated_at=excluded.updated_at
            """,
            (sid, str(channel).strip(), str(channel_thread).strip(), str(actor_id or "").strip(),
             str(host_session or "").strip(), target, str(target_session or "").strip(), now, now, now),
        )
        return self.status(sid)

    def resolve(self, session_id: str) -> dict[str, Any]:
        row = self.storage.conn.execute(
            "SELECT * FROM agent_sessions WHERE session_id=?", (session_id,)
        ).fetchone()
        if row is None:
            raise DeliveryBusError(
                "session_not_found",
                f"no session binding for {session_id!r}",
                resume_action="run `adb session bind` first, or pass identity fields explicitly",
            )
        binding = dict(row)
        if self._is_stale(binding):
            raise DeliveryBusError(
                "session_stale",
                f"session {session_id} is stale (last_seen={binding['last_seen_at']})",
                resume_action="re-run `adb session bind` with the same identity to rebound",
                data={"binding": binding},
            )
        return binding

    def resolve_by_thread(
        self,
        *,
        channel: str,
        channel_thread: str,
        actor_id: str = "",
        host_session: str = "",
    ) -> dict[str, Any]:
        sid = session_id_for(
            channel=channel,
            channel_thread=channel_thread,
            actor_id=actor_id,
            host_session=host_session,
        )
        return self.resolve(sid)

    def list(self, *, channel: str = "") -> list[dict[str, Any]]:
        if channel:
            rows = self.storage.conn.execute(
                "SELECT * FROM agent_sessions WHERE channel=? ORDER BY updated_at DESC",
                (str(channel).strip().lower(),),
            ).fetchall()
        else:
            rows = self.storage.conn.execute(
                "SELECT * FROM agent_sessions ORDER BY updated_at DESC"
            ).fetchall()
        return [dict(row) for row in rows]

    def status(self, session_id: str) -> dict[str, Any]:
        row = self.storage.conn.execute(
            "SELECT * FROM agent_sessions WHERE session_id=?", (session_id,)
        ).fetchone()
        if row is None:
            raise DeliveryBusError(
                "session_not_found",
                f"no session binding for {session_id!r}",
                resume_action="run `adb session bind` first",
            )
        binding = dict(row)
        stale = self._is_stale(binding)
        return {
            **binding,
            "state": "stale" if stale else binding.get("state", "bound"),
            "stale": stale,
        }

    def acquire(self, session_id: str, dispatch_id: str) -> dict[str, Any]:
        if not session_id or not dispatch_id:
            raise DeliveryBusError("session_lease_invalid", "session_id and dispatch_id are required")
        row = self.storage.conn.execute(
            "SELECT dispatch_id FROM session_leases WHERE session_id=?", (session_id,)
        ).fetchone()
        if row is not None and str(row["dispatch_id"]) != str(dispatch_id):
            raise DeliveryBusError(
                "session_busy",
                f"session {session_id} is leased by dispatch {row['dispatch_id']}",
                resume_action="wait for the in-flight dispatch to complete, or use --target-session auto",
                data={"held_by": row["dispatch_id"]},
            )
        self.storage.conn.execute(
            "INSERT INTO session_leases(session_id,dispatch_id,acquired_at) VALUES(?,?,?) "
            "ON CONFLICT(session_id) DO UPDATE SET acquired_at=excluded.acquired_at",
            (session_id, dispatch_id, _now()),
        )
        return {"session_id": session_id, "dispatch_id": dispatch_id, "acquired": True}

    def release(self, session_id: str, dispatch_id: str) -> dict[str, Any]:
        row = self.storage.conn.execute(
            "SELECT dispatch_id FROM session_leases WHERE session_id=?", (session_id,)
        ).fetchone()
        if row is None:
            return {"session_id": session_id, "released": False, "reason": "not_leased"}
        if str(row["dispatch_id"]) != str(dispatch_id):
            raise DeliveryBusError(
                "session_lease_mismatch",
                "lease release dispatch mismatch",
                resume_action="only the owning dispatch may release the lease",
            )
        self.storage.conn.execute("DELETE FROM session_leases WHERE session_id=?", (session_id,))
        return {"session_id": session_id, "released": True}

    def _is_stale(self, binding: dict[str, Any]) -> bool:
        try:
            last = datetime.fromisoformat(str(binding.get("last_seen_at") or ""))
        except ValueError:
            return True
        cutoff = datetime.now(timezone.utc) - timedelta(seconds=self.ttl_seconds)
        return last < cutoff
