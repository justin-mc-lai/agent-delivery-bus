"""AC-CPH-005: `adb backup` copies ledger + configs with manifest, fail-closed."""

from __future__ import annotations

import json
import os
import sqlite3
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.backup import backup_control_plane
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.storage import Storage


ROOT = Path(__file__).resolve().parents[1]


def _make_ledger(path: Path) -> None:
    storage = Storage(str(path))
    storage.create_dispatch(
        idempotency_key="k-1",
        request_hash="h-1",
        request={"schema_version": "1.1", "stage": "plan", "feature": "f"},
        project_slug="demo",
        stage="plan",
        feature="f",
    )
    storage.close()


class BackupTests(unittest.TestCase):
    def test_backup_creates_manifest_and_consistent_copy(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            db = root / "agent-delivery-bus.sqlite3"
            _make_ledger(db)
            cfg = root / "projects.local.json"
            cfg.write_text(json.dumps({"projects": [{"slug": "demo"}]}), encoding="utf-8")
            dest = root / "backup"

            manifest = backup_control_plane(db_path=db, dest=dest, config_paths=[cfg])

            self.assertEqual(manifest["integrity_check"], "ok")
            self.assertTrue((dest / "agent-delivery-bus.sqlite3").is_file())
            self.assertTrue((dest / "projects.local.json").is_file())
            self.assertTrue((dest / "manifest.json").is_file())

            copied = sqlite3.connect(str(dest / "agent-delivery-bus.sqlite3"))
            try:
                count = copied.execute("SELECT COUNT(*) FROM dispatches").fetchone()[0]
            finally:
                copied.close()
            self.assertEqual(count, 1)

    def test_backup_fails_closed_on_missing_sources(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            dest = root / "backup"
            with self.assertRaises(DeliveryBusError) as ctx:
                backup_control_plane(
                    db_path=root / "missing.sqlite3",
                    dest=dest,
                    config_paths=[root / "projects.json"],
                )
            self.assertEqual(ctx.exception.reason_code, "backup_db_missing")

            db = root / "db.sqlite3"
            _make_ledger(db)
            with self.assertRaises(DeliveryBusError) as ctx:
                backup_control_plane(
                    db_path=db,
                    dest=dest,
                    config_paths=[root / "missing.json"],
                )
            self.assertEqual(ctx.exception.reason_code, "backup_config_missing")

    def test_backup_rejects_nonempty_dest(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            db = root / "db.sqlite3"
            _make_ledger(db)
            cfg = root / "projects.json"
            cfg.write_text("{}", encoding="utf-8")
            dest = root / "backup"
            dest.mkdir()
            (dest / "existing.txt").write_text("x", encoding="utf-8")
            with self.assertRaises(DeliveryBusError) as ctx:
                backup_control_plane(db_path=db, dest=dest, config_paths=[cfg])
            self.assertEqual(ctx.exception.reason_code, "backup_dest_not_empty")

    def test_cli_backup_smoke(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            db = root / "agent-delivery-bus.sqlite3"
            _make_ledger(db)
            cfg = root / "projects.json"
            cfg.write_text(json.dumps({"projects": []}), encoding="utf-8")
            dest = root / "backup"
            env = dict(os.environ)
            env["PYTHONPATH"] = str(ROOT / "src")
            result = subprocess.run(
                [
                    sys.executable,
                    "-m",
                    "agent_delivery_bus.cli",
                    "--config",
                    str(cfg),
                    "--db",
                    str(db),
                    "backup",
                    "--dest",
                    str(dest),
                    "--json",
                ],
                capture_output=True,
                text=True,
                env=env,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["status"], "pass")
            self.assertTrue((dest / "manifest.json").is_file())


if __name__ == "__main__":
    unittest.main()
