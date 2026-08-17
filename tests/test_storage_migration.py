"""AC-CPH-003/004: schema_version migration framework, legacy upgrade, idempotency."""

from __future__ import annotations

import sqlite3
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.storage import SCHEMA_VERSION, Storage


class StorageMigrationTests(unittest.TestCase):
    def test_fresh_schema_is_up_to_date_with_audit_rows(self):
        storage = Storage(":memory:")
        status = storage.schema_status()
        self.assertTrue(status["up_to_date"])
        self.assertEqual(status["user_version"], SCHEMA_VERSION)
        self.assertEqual(
            [row["version"] for row in status["applied"]],
            list(range(1, SCHEMA_VERSION + 1)),
        )
        columns = {
            row["name"]
            for row in storage.conn.execute("PRAGMA table_info(dispatches)").fetchall()
        }
        self.assertIn("executor_board", columns)
        self.assertIn("executor_task_id", columns)
        approval_columns = {
            row["name"]
            for row in storage.conn.execute("PRAGMA table_info(approvals)").fetchall()
        }
        self.assertIn("channel_actor", approval_columns)
        storage.close()

    def test_legacy_database_upgrades_and_is_idempotent(self):
        with tempfile.TemporaryDirectory() as tmp:
            db_path = Path(tmp) / "legacy.sqlite3"
            conn = sqlite3.connect(str(db_path))
            conn.executescript(
                """
                CREATE TABLE dispatches (
                    dispatch_id TEXT PRIMARY KEY,
                    idempotency_key TEXT NOT NULL UNIQUE,
                    request_hash TEXT NOT NULL,
                    request_json TEXT NOT NULL,
                    project_slug TEXT NOT NULL,
                    stage TEXT NOT NULL,
                    feature TEXT NOT NULL,
                    state TEXT NOT NULL,
                    hermes_board TEXT,
                    hermes_task_id TEXT,
                    last_reason_code TEXT NOT NULL DEFAULT '',
                    resume_action TEXT NOT NULL DEFAULT '',
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL
                );
                CREATE TABLE approvals (
                    approval_id TEXT PRIMARY KEY,
                    token_hash TEXT NOT NULL UNIQUE,
                    actor TEXT NOT NULL,
                    project_slug TEXT NOT NULL,
                    stage TEXT NOT NULL,
                    feature TEXT NOT NULL,
                    expires_at TEXT NOT NULL,
                    state TEXT NOT NULL,
                    created_at TEXT NOT NULL
                );
                CREATE TABLE boundary_proposals (
                    id TEXT PRIMARY KEY,
                    topic TEXT NOT NULL,
                    query_hints_json TEXT NOT NULL DEFAULT '[]',
                    sources_json TEXT NOT NULL DEFAULT '[]',
                    rationale TEXT NOT NULL DEFAULT '',
                    status TEXT NOT NULL,
                    actor TEXT NOT NULL DEFAULT '',
                    decision_note TEXT NOT NULL DEFAULT '',
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    decided_at TEXT NOT NULL DEFAULT ''
                );
                """
            )
            conn.commit()
            conn.close()

            storage = Storage(str(db_path))
            status = storage.schema_status()
            self.assertTrue(status["up_to_date"])
            self.assertEqual(status["user_version"], SCHEMA_VERSION)
            dispatch_columns = {
                row["name"] for row in storage.conn.execute("PRAGMA table_info(dispatches)").fetchall()
            }
            self.assertNotIn("hermes_board", dispatch_columns)
            self.assertIn("executor_board", dispatch_columns)
            self.assertNotIn("hermes_task_id", dispatch_columns)
            self.assertIn("executor_task_id", dispatch_columns)
            approval_columns = {
                row["name"] for row in storage.conn.execute("PRAGMA table_info(approvals)").fetchall()
            }
            self.assertIn("channel_actor", approval_columns)
            boundary_columns = {
                row["name"] for row in storage.conn.execute("PRAGMA table_info(boundary_proposals)").fetchall()
            }
            self.assertIn("project_profile_ref", boundary_columns)
            self.assertIn("libraries_json", boundary_columns)
            storage.close()

            # Reopen: idempotent, no duplicate audit rows, same version.
            storage = Storage(str(db_path))
            status = storage.schema_status()
            self.assertEqual(status["user_version"], SCHEMA_VERSION)
            self.assertEqual(len(status["applied"]), SCHEMA_VERSION)
            storage.close()

    def test_migration_gap_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            db_path = Path(tmp) / "gapped.sqlite3"
            conn = sqlite3.connect(str(db_path))
            conn.execute("PRAGMA user_version = 5")
            conn.commit()
            conn.close()
            with self.assertRaises(DeliveryBusError) as ctx:
                Storage(str(db_path))
            self.assertEqual(ctx.exception.reason_code, "schema_migration_gap")


if __name__ == "__main__":
    unittest.main()
