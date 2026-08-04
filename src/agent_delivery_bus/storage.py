"""SQLite ledger for approvals, dispatches, and audit events."""

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
                executor_board TEXT,
                executor_task_id TEXT,
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
            CREATE TABLE IF NOT EXISTS schedule_entries (
                slug TEXT PRIMARY KEY,
                command TEXT NOT NULL,
                engine TEXT NOT NULL,
                cron_expr TEXT NOT NULL,
                quota_limit INTEGER NOT NULL,
                health TEXT NOT NULL DEFAULT 'healthy',
                updated_at TEXT NOT NULL,
                created_at TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS quota_ledgers (
                slug TEXT NOT NULL,
                window TEXT NOT NULL,
                slots_spent INTEGER NOT NULL DEFAULT 0,
                slots_allowed INTEGER NOT NULL,
                next_eligible_at TEXT NOT NULL DEFAULT '',
                updated_at TEXT NOT NULL,
                PRIMARY KEY(slug, window)
            );
            CREATE TABLE IF NOT EXISTS heartbeat_runs (
                run_id TEXT PRIMARY KEY,
                entry_slug TEXT NOT NULL,
                source TEXT NOT NULL,
                status TEXT NOT NULL,
                evidence_refs_json TEXT NOT NULL DEFAULT '[]',
                quota_spent INTEGER NOT NULL DEFAULT 0,
                reason_code TEXT NOT NULL DEFAULT '',
                payload_json TEXT NOT NULL DEFAULT '{}',
                created_at TEXT NOT NULL,
                updated_at TEXT NOT NULL
            );
            """
        )
        self._migrate_legacy_columns()

    def _migrate_legacy_columns(self) -> None:
        columns = {
            row["name"]
            for row in self.conn.execute("PRAGMA table_info(dispatches)").fetchall()
        }
        if "hermes_board" in columns and "executor_board" not in columns:
            self.conn.execute("ALTER TABLE dispatches RENAME COLUMN hermes_board TO executor_board")
            columns.discard("hermes_board")
            columns.add("executor_board")
        if "hermes_task_id" in columns and "executor_task_id" not in columns:
            self.conn.execute("ALTER TABLE dispatches RENAME COLUMN hermes_task_id TO executor_task_id")
            columns.discard("hermes_task_id")
            columns.add("executor_task_id")
        if "executor_board" not in columns:
            self.conn.execute("ALTER TABLE dispatches ADD COLUMN executor_board TEXT")
        if "executor_task_id" not in columns:
            self.conn.execute("ALTER TABLE dispatches ADD COLUMN executor_task_id TEXT")

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
        executor_board: str | None = None,
        executor_task_id: str | None = None,
        # Backward-compatible aliases
        hermes_board: str | None = None,
        hermes_task_id: str | None = None,
    ) -> dict[str, Any]:
        if executor_board is None:
            executor_board = hermes_board
        if executor_task_id is None:
            executor_task_id = hermes_task_id
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
                ("executor_board", executor_board),
                ("executor_task_id", executor_task_id),
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
        # Compatibility mirrors for older skill/docs consumers.
        payload["hermes_board"] = payload.get("executor_board")
        payload["hermes_task_id"] = payload.get("executor_task_id")
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

    # --- schedule / quota / heartbeat ledger (vision-flywheel) ---

    def upsert_schedule_entry(self, entry: dict[str, Any]) -> dict[str, Any]:
        slug = str(entry["slug"])
        timestamp = now_iso()
        existing = self.conn.execute(
            "SELECT created_at FROM schedule_entries WHERE slug=?",
            (slug,),
        ).fetchone()
        created_at = str(existing["created_at"]) if existing else timestamp
        self.conn.execute(
            """
            INSERT INTO schedule_entries(
              slug, command, engine, cron_expr, quota_limit, health, updated_at, created_at
            ) VALUES(?,?,?,?,?,?,?,?)
            ON CONFLICT(slug) DO UPDATE SET
              command=excluded.command,
              engine=excluded.engine,
              cron_expr=excluded.cron_expr,
              quota_limit=excluded.quota_limit,
              health=excluded.health,
              updated_at=excluded.updated_at
            """,
            (
                slug,
                str(entry["command"]),
                str(entry["engine"]),
                str(entry["cron_expr"]),
                int(entry["quota_limit"]),
                str(entry.get("health") or "healthy"),
                str(entry.get("updated_at") or timestamp),
                created_at,
            ),
        )
        row = self.get_schedule_entry(slug)
        assert row is not None
        return row

    def get_schedule_entry(self, slug: str) -> dict[str, Any] | None:
        row = self.conn.execute(
            "SELECT * FROM schedule_entries WHERE slug=?",
            (slug,),
        ).fetchone()
        return dict(row) if row else None

    def list_schedule_entries(self) -> list[dict[str, Any]]:
        rows = self.conn.execute(
            "SELECT * FROM schedule_entries ORDER BY slug ASC"
        ).fetchall()
        return [dict(row) for row in rows]

    def ensure_quota_ledger(self, slug: str, *, window: str, slots_allowed: int) -> dict[str, Any]:
        existing = self.conn.execute(
            "SELECT * FROM quota_ledgers WHERE slug=? AND window=?",
            (slug, window),
        ).fetchone()
        if existing:
            if int(existing["slots_allowed"]) != int(slots_allowed):
                self.conn.execute(
                    "UPDATE quota_ledgers SET slots_allowed=?, updated_at=? WHERE slug=? AND window=?",
                    (int(slots_allowed), now_iso(), slug, window),
                )
                existing = self.conn.execute(
                    "SELECT * FROM quota_ledgers WHERE slug=? AND window=?",
                    (slug, window),
                ).fetchone()
            return dict(existing)
        self.conn.execute(
            """
            INSERT INTO quota_ledgers(slug, window, slots_spent, slots_allowed, next_eligible_at, updated_at)
            VALUES(?,?,0,?,?,?)
            """,
            (slug, window, int(slots_allowed), "", now_iso()),
        )
        row = self.conn.execute(
            "SELECT * FROM quota_ledgers WHERE slug=? AND window=?",
            (slug, window),
        ).fetchone()
        return dict(row)

    def spend_quota_slot(self, slug: str, *, window: str, slots: int = 1) -> dict[str, Any]:
        with self.transaction():
            row = self.conn.execute(
                "SELECT * FROM quota_ledgers WHERE slug=? AND window=?",
                (slug, window),
            ).fetchone()
            if row is None:
                raise DeliveryBusError("quota_ledger_missing", f"no quota ledger for {slug}/{window}")
            spent = int(row["slots_spent"])
            allowed = int(row["slots_allowed"])
            if spent + slots > allowed:
                raise DeliveryBusError(
                    "quota_exhausted",
                    f"quota exhausted for {slug} in window {window}",
                    resume_action="raise quota-limit or wait for next window",
                )
            new_spent = spent + slots
            self.conn.execute(
                "UPDATE quota_ledgers SET slots_spent=?, updated_at=? WHERE slug=? AND window=?",
                (new_spent, now_iso(), slug, window),
            )
        ledger = self.ensure_quota_ledger(slug, window=window, slots_allowed=allowed)
        return {**ledger, "spent_this_call": slots}

    def append_heartbeat_run(self, run: dict[str, Any]) -> dict[str, Any]:
        self.conn.execute(
            """
            INSERT INTO heartbeat_runs(
              run_id, entry_slug, source, status, evidence_refs_json, quota_spent,
              reason_code, payload_json, created_at, updated_at
            ) VALUES(?,?,?,?,?,?,?,?,?,?)
            """,
            (
                run["run_id"],
                run["entry_slug"],
                run["source"],
                run["status"],
                json.dumps(list(run.get("evidence_refs") or []), ensure_ascii=False),
                int(run.get("quota_spent") or 0),
                str(run.get("reason_code") or ""),
                json.dumps(run.get("payload") or {}, ensure_ascii=False, sort_keys=True),
                str(run.get("created_at") or now_iso()),
                str(run.get("updated_at") or now_iso()),
            ),
        )
        stored = self.get_heartbeat_run(run["run_id"])
        assert stored is not None
        return stored

    def get_heartbeat_run(self, run_id: str) -> dict[str, Any] | None:
        row = self.conn.execute(
            "SELECT * FROM heartbeat_runs WHERE run_id=?",
            (run_id,),
        ).fetchone()
        if row is None:
            return None
        payload = dict(row)
        payload["evidence_refs"] = json.loads(payload.pop("evidence_refs_json") or "[]")
        payload["payload"] = json.loads(payload.pop("payload_json") or "{}")
        return payload

    def update_heartbeat_run(
        self,
        run_id: str,
        *,
        status: str,
        evidence_refs: list[str] | None = None,
        quota_spent: int | None = None,
        reason_code: str = "",
        quota_snapshot: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        current = self.get_heartbeat_run(run_id)
        if current is None:
            raise DeliveryBusError("heartbeat_run_not_found", f"no heartbeat run {run_id}")
        refs = list(evidence_refs if evidence_refs is not None else current.get("evidence_refs") or [])
        spent = int(quota_spent if quota_spent is not None else current.get("quota_spent") or 0)
        extra = dict(current.get("payload") or {})
        if quota_snapshot:
            extra["quota_snapshot"] = quota_snapshot
        self.conn.execute(
            """
            UPDATE heartbeat_runs
            SET status=?, evidence_refs_json=?, quota_spent=?, reason_code=?, payload_json=?, updated_at=?
            WHERE run_id=?
            """,
            (
                status,
                json.dumps(refs, ensure_ascii=False),
                spent,
                reason_code,
                json.dumps(extra, ensure_ascii=False, sort_keys=True),
                now_iso(),
                run_id,
            ),
        )
        updated = self.get_heartbeat_run(run_id)
        assert updated is not None
        return updated

    def list_heartbeat_runs(self, *, entry_slug: str | None = None) -> list[dict[str, Any]]:
        if entry_slug:
            rows = self.conn.execute(
                "SELECT run_id FROM heartbeat_runs WHERE entry_slug=? ORDER BY created_at ASC",
                (entry_slug,),
            ).fetchall()
        else:
            rows = self.conn.execute(
                "SELECT run_id FROM heartbeat_runs ORDER BY created_at ASC"
            ).fetchall()
        return [self.get_heartbeat_run(row["run_id"]) for row in rows]  # type: ignore[misc]
