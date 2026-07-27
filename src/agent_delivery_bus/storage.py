from __future__ import annotations

import json
import sqlite3
import uuid
from contextlib import contextmanager
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Iterator

from .errors import DeliveryBusError


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


class Storage:
    def __init__(self, path: str | Path):
        self.path = str(path)
        if self.path != ":memory:":
            Path(self.path).expanduser().resolve().parent.mkdir(parents=True, exist_ok=True)
        self.conn = sqlite3.connect(self.path, timeout=10, isolation_level=None)
        self.conn.row_factory = sqlite3.Row
        self.conn.execute("PRAGMA foreign_keys=ON")
        self.conn.execute("PRAGMA busy_timeout=10000")
        self.initialize()

    def close(self) -> None:
        self.conn.close()

    @contextmanager
    def transaction(self) -> Iterator[sqlite3.Connection]:
        self.conn.execute("BEGIN IMMEDIATE")
        try:
            yield self.conn
        except Exception:
            self.conn.execute("ROLLBACK")
            raise
        else:
            self.conn.execute("COMMIT")

    def initialize(self) -> None:
        self.conn.executescript(
            """
            CREATE TABLE IF NOT EXISTS project_snapshots (
                slug TEXT PRIMARY KEY,
                payload_json TEXT NOT NULL,
                captured_at TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS approvals (
                approval_id TEXT PRIMARY KEY,
                token_hash TEXT NOT NULL UNIQUE,
                actor TEXT NOT NULL,
                project_slug TEXT NOT NULL,
                stage TEXT NOT NULL,
                feature TEXT NOT NULL,
                expires_at TEXT NOT NULL,
                state TEXT NOT NULL CHECK(state IN ('issued','reserved','consumed','revoked')),
                reserved_by TEXT,
                reserved_at TEXT,
                consumed_at TEXT,
                created_at TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS dispatches (
                dispatch_id TEXT PRIMARY KEY,
                idempotency_key TEXT NOT NULL UNIQUE,
                request_hash TEXT NOT NULL,
                request_json TEXT NOT NULL,
                project_slug TEXT NOT NULL,
                stage TEXT NOT NULL,
                feature TEXT NOT NULL,
                state TEXT NOT NULL,
                approval_id TEXT,
                hermes_board TEXT,
                hermes_task_id TEXT,
                last_reason_code TEXT NOT NULL DEFAULT '',
                resume_action TEXT NOT NULL DEFAULT '',
                created_at TEXT NOT NULL,
                updated_at TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS dispatch_events (
                dispatch_id TEXT NOT NULL,
                sequence INTEGER NOT NULL,
                event_type TEXT NOT NULL,
                from_state TEXT NOT NULL,
                to_state TEXT NOT NULL,
                reason_code TEXT NOT NULL DEFAULT '',
                payload_json TEXT NOT NULL,
                created_at TEXT NOT NULL,
                PRIMARY KEY(dispatch_id, sequence),
                FOREIGN KEY(dispatch_id) REFERENCES dispatches(dispatch_id)
            );
            """
        )

    def snapshot_project(self, slug: str, payload: dict[str, Any]) -> None:
        self.conn.execute(
            """
            INSERT INTO project_snapshots(slug, payload_json, captured_at)
            VALUES(?,?,?)
            ON CONFLICT(slug) DO UPDATE SET
              payload_json=excluded.payload_json,
              captured_at=excluded.captured_at
            """,
            (slug, json.dumps(payload, ensure_ascii=False, sort_keys=True), now_iso()),
        )

    def create_dispatch(
        self,
        *,
        idempotency_key: str,
        request_hash: str,
        request: dict[str, Any],
        project_slug: str,
        stage: str,
        feature: str,
    ) -> tuple[dict[str, Any], bool]:
        with self.transaction():
            existing = self.conn.execute(
                "SELECT * FROM dispatches WHERE idempotency_key=?",
                (idempotency_key,),
            ).fetchone()
            if existing:
                row = dict(existing)
                if row["request_hash"] != request_hash:
                    raise DeliveryBusError(
                        "idempotency_conflict",
                        "The idempotency key already belongs to a different normalized request",
                    )
                return self.get_dispatch(row["dispatch_id"]), False
            dispatch_id = f"adb_{uuid.uuid4().hex[:20]}"
            timestamp = now_iso()
            self.conn.execute(
                """
                INSERT INTO dispatches(
                  dispatch_id,idempotency_key,request_hash,request_json,
                  project_slug,stage,feature,state,created_at,updated_at
                ) VALUES(?,?,?,?,?,?,?,'draft',?,?)
                """,
                (
                    dispatch_id,
                    idempotency_key,
                    request_hash,
                    json.dumps(request, ensure_ascii=False, sort_keys=True),
                    project_slug,
                    stage,
                    feature,
                    timestamp,
                    timestamp,
                ),
            )
            self._append_event_locked(
                dispatch_id,
                event_type="created",
                from_state="",
                to_state="draft",
                payload={"request": request},
            )
            return self.get_dispatch(dispatch_id), True

    def _append_event_locked(
        self,
        dispatch_id: str,
        *,
        event_type: str,
        from_state: str,
        to_state: str,
        reason_code: str = "",
        payload: dict[str, Any] | None = None,
    ) -> None:
        next_sequence = self.conn.execute(
            "SELECT COALESCE(MAX(sequence), 0) + 1 FROM dispatch_events WHERE dispatch_id=?",
            (dispatch_id,),
        ).fetchone()[0]
        self.conn.execute(
            """
            INSERT INTO dispatch_events(
              dispatch_id,sequence,event_type,from_state,to_state,reason_code,payload_json,created_at
            ) VALUES(?,?,?,?,?,?,?,?)
            """,
            (
                dispatch_id,
                next_sequence,
                event_type,
                from_state,
                to_state,
                reason_code,
                json.dumps(payload or {}, ensure_ascii=False, sort_keys=True),
                now_iso(),
            ),
        )

    def transition(
        self,
        dispatch_id: str,
        *,
        expected_from: str | tuple[str, ...],
        to_state: str,
        event_type: str,
        reason_code: str = "",
        resume_action: str = "",
        payload: dict[str, Any] | None = None,
        approval_id: str | None = None,
        hermes_board: str | None = None,
        hermes_task_id: str | None = None,
    ) -> dict[str, Any]:
        allowed = (expected_from,) if isinstance(expected_from, str) else expected_from
        with self.transaction():
            row = self.conn.execute(
                "SELECT * FROM dispatches WHERE dispatch_id=?",
                (dispatch_id,),
            ).fetchone()
            if row is None:
                raise DeliveryBusError("dispatch_not_found", f"Dispatch not found: {dispatch_id}")
            current = str(row["state"])
            if current not in allowed:
                raise DeliveryBusError(
                    "invalid_transition",
                    f"Cannot transition {dispatch_id} from {current} to {to_state}",
                    data={"expected_from": list(allowed), "actual": current, "to": to_state},
                )
            fields = [
                "state=?",
                "last_reason_code=?",
                "resume_action=?",
                "updated_at=?",
            ]
            values: list[Any] = [to_state, reason_code, resume_action, now_iso()]
            for column, value in (
                ("approval_id", approval_id),
                ("hermes_board", hermes_board),
                ("hermes_task_id", hermes_task_id),
            ):
                if value is not None:
                    fields.append(f"{column}=?")
                    values.append(value)
            values.append(dispatch_id)
            self.conn.execute(
                f"UPDATE dispatches SET {', '.join(fields)} WHERE dispatch_id=?",
                values,
            )
            self._append_event_locked(
                dispatch_id,
                event_type=event_type,
                from_state=current,
                to_state=to_state,
                reason_code=reason_code,
                payload=payload,
            )
        return self.get_dispatch(dispatch_id)

    def get_dispatch(self, dispatch_id: str) -> dict[str, Any]:
        row = self.conn.execute(
            "SELECT * FROM dispatches WHERE dispatch_id=?",
            (dispatch_id,),
        ).fetchone()
        if row is None:
            raise DeliveryBusError("dispatch_not_found", f"Dispatch not found: {dispatch_id}")
        payload = dict(row)
        payload["request"] = json.loads(payload.pop("request_json"))
        events = self.conn.execute(
            "SELECT * FROM dispatch_events WHERE dispatch_id=? ORDER BY sequence",
            (dispatch_id,),
        ).fetchall()
        payload["events"] = [
            {
                **dict(event),
                "payload": json.loads(event["payload_json"]),
            }
            for event in events
        ]
        for event in payload["events"]:
            event.pop("payload_json", None)
        return payload

    def list_dispatches(self, *, project_slug: str | None = None) -> list[dict[str, Any]]:
        if project_slug:
            rows = self.conn.execute(
                "SELECT dispatch_id FROM dispatches WHERE project_slug=? ORDER BY created_at DESC",
                (project_slug,),
            ).fetchall()
        else:
            rows = self.conn.execute(
                "SELECT dispatch_id FROM dispatches ORDER BY created_at DESC"
            ).fetchall()
        return [self.get_dispatch(row["dispatch_id"]) for row in rows]
