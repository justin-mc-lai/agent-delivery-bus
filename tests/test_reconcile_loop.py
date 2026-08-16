"""Tests for the periodic reconcile loop and its Hermes cron template."""

from __future__ import annotations

import io
import json
import sqlite3
import subprocess
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

from agent_delivery_bus.cli import main
from agent_delivery_bus.adapters.null import NullExecutor
from agent_delivery_bus.storage import Storage


def _build(tmp: Path) -> tuple[Path, Path]:
    repo = tmp / "demo-app"
    repo.mkdir(parents=True)
    subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
    (repo / "docs" / "v1.0.0").mkdir(parents=True)
    config = tmp / "projects.json"
    config.write_text(
        json.dumps(
            {
                "schema_version": "1.0",
                "adapters": {
                    "executor": "null",
                    "truth_gate": "null",
                    "memory": "inprocess",
                },
                "projects": [
                    {
                        "slug": "demo-app",
                        "title": "Demo App",
                        "class": "managed",
                        "repo": str(repo.resolve()),
                        "docs_root": str((repo / "docs").resolve()),
                        "docs_version": "v1.0.0",
                        "aliases": ["demo"],
                        "dispatchable": True,
                    }
                ],
            },
            ensure_ascii=False,
            indent=2,
        ),
        encoding="utf-8",
    )
    db = tmp / "db.sqlite3"
    return config, db


class ReconcileLoopTests(unittest.TestCase):
    def test_cron_template_prints_register_and_silent_tick(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config, db = _build(root)
            buf = io.StringIO()
            with redirect_stdout(buf):
                code = main(
                    [
                        "--config", str(config),
                        "--db", str(db),
                        "reconcile-loop", "cron-template", "--json",
                    ]
                )
            self.assertEqual(code, 0)
            payload = json.loads(buf.getvalue())
            self.assertEqual(payload["status"], "pass")
            self.assertIn('hermes cron create "every 1m" --name adb-reconcile', payload["data"]["register"])
            self.assertIn("adb reconcile-loop --once --interval 0", payload["data"]["text"])
            self.assertIn(">>\"$LOG\" 2>&1", payload["data"]["text"])

    def test_once_reconciles_pending_to_completed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config, db = _build(root)
            shared_executor = NullExecutor()
            with mock.patch(
                "agent_delivery_bus.adapters.factory.create_executor",
                return_value=shared_executor,
            ):
                self.assertEqual(
                    main(["--config", str(config), "--db", str(db), "dispatch", "--project", "demo", "--stage", "plan", "--feature", "f1"]),
                    0,
                )
            storage = Storage(db)
            self.assertEqual(storage.list_dispatches()[0]["state"], "dispatched")
            buf = io.StringIO()
            with mock.patch(
                "agent_delivery_bus.adapters.factory.create_executor",
                return_value=shared_executor,
            ):
                with redirect_stdout(buf):
                    code = main(
                        [
                            "--config", str(config),
                            "--db", str(db),
                            "reconcile-loop", "--once", "--interval", "0", "--json",
                        ]
                    )
            self.assertEqual(code, 0)
            payload = json.loads(buf.getvalue())
            self.assertEqual(payload["status"], "pass")
            self.assertEqual(payload["data"]["rounds"], 1)
            self.assertEqual(payload["data"]["results"][0]["status"], "completed")
            self.assertEqual(storage.list_dispatches()[0]["state"], "completed")

    def test_max_runs_single_round_without_once(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config, db = _build(root)
            shared_executor = NullExecutor()
            with mock.patch(
                "agent_delivery_bus.adapters.factory.create_executor",
                return_value=shared_executor,
            ):
                main(["--config", str(config), "--db", str(db), "dispatch", "--project", "demo", "--stage", "plan", "--feature", "f2"])
            buf = io.StringIO()
            with mock.patch(
                "agent_delivery_bus.adapters.factory.create_executor",
                return_value=shared_executor,
            ):
                with redirect_stdout(buf):
                    code = main(
                        [
                            "--config", str(config),
                            "--db", str(db),
                            "reconcile-loop", "--max-runs", "1", "--interval", "0", "--json",
                        ]
                    )
            self.assertEqual(code, 0)
            payload = json.loads(buf.getvalue())
            self.assertEqual(payload["data"]["rounds"], 1)
            self.assertEqual(payload["data"]["results"][0]["status"], "completed")

    def test_idempotent_rerun_sends_nothing_new(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config, db = _build(root)
            shared_executor = NullExecutor()
            with mock.patch(
                "agent_delivery_bus.adapters.factory.create_executor",
                return_value=shared_executor,
            ):
                main(["--config", str(config), "--db", str(db), "dispatch", "--project", "demo", "--stage", "plan", "--feature", "f3"])
                main(["--config", str(config), "--db", str(db), "reconcile-loop", "--once"])
            conn = sqlite3.connect(db)
            events = conn.execute(
                "SELECT count(*) FROM dispatch_events WHERE event_type='closure_verified'"
            ).fetchone()[0]
            conn.close()
            with mock.patch(
                "agent_delivery_bus.adapters.factory.create_executor",
                return_value=shared_executor,
            ):
                main(["--config", str(config), "--db", str(db), "reconcile-loop", "--once"])
            conn = sqlite3.connect(db)
            events_after = conn.execute(
                "SELECT count(*) FROM dispatch_events WHERE event_type='closure_verified'"
            ).fetchone()[0]
            conn.close()
            self.assertEqual(events, 1)
            self.assertEqual(events_after, 1)

    def test_orphan_dispatch_parks_blocked_and_skips_next_round(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config, db = _build(root)
            shared_executor = NullExecutor()
            with mock.patch(
                "agent_delivery_bus.adapters.factory.create_executor",
                return_value=shared_executor,
            ):
                main(["--config", str(config), "--db", str(db), "dispatch", "--project", "demo", "--stage", "plan", "--feature", "f4"])
            # Registry no longer knows the dispatched project -> orphan dispatch.
            (root / "other-app").mkdir(parents=True)
            config.write_text(
                json.dumps(
                    {
                        "schema_version": "1.0",
                        "adapters": {
                            "executor": "null",
                            "truth_gate": "null",
                            "memory": "inprocess",
                        },
                        "projects": [
                            {
                                "slug": "other-app",
                                "title": "Other App",
                                "class": "managed",
                                "repo": str((root / "other-app").resolve()),
                                "docs_version": "v1.0.0",
                                "aliases": ["other"],
                                "dispatchable": True,
                            }
                        ],
                    },
                    ensure_ascii=False,
                ),
                encoding="utf-8",
            )
            buf = io.StringIO()
            with mock.patch(
                "agent_delivery_bus.adapters.factory.create_executor",
                return_value=shared_executor,
            ):
                with redirect_stdout(buf):
                    code = main(
                        [
                            "--config", str(config),
                            "--db", str(db),
                            "reconcile-loop", "--once", "--json",
                        ]
                    )
            payload = json.loads(buf.getvalue())
            self.assertEqual(payload["status"], "blocked")
            self.assertEqual(payload["data"]["results"][0]["reason_code"], "project_not_found")
            storage = Storage(db)
            self.assertEqual(storage.list_dispatches()[0]["state"], "blocked")
            # Second round: the parked dispatch is no longer pending.
            buf2 = io.StringIO()
            with mock.patch(
                "agent_delivery_bus.adapters.factory.create_executor",
                return_value=shared_executor,
            ):
                with redirect_stdout(buf2):
                    main(
                        [
                            "--config", str(config),
                            "--db", str(db),
                            "reconcile-loop", "--once", "--json",
                        ]
                    )
            payload2 = json.loads(buf2.getvalue())
            self.assertEqual(payload2["status"], "pass")
            self.assertEqual(payload2["data"]["results"], [])


if __name__ == "__main__":
    unittest.main()
