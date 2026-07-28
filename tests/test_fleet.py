from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.null import NullExecutor, NullTruthGate
from agent_delivery_bus.cli import _summarize_project, render_fleet_text
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import make_project, write_registry
from .test_null_adapters import init_git


class FleetSummaryTests(unittest.TestCase):
    def test_summarize_includes_local_and_kanban_counts(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            init_git(project.repo)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            executor = NullExecutor(auto_complete=True)
            truth = NullTruthGate(auto_pass=False)
            service = DeliveryService(
                registry,
                storage,
                executor=executor,
                truth_gate=truth,
            )
            service.dispatch(project_slug="demo", stage="plan", feature="feature-a")
            dispatches = storage.list_dispatches(project_slug="demo")
            row = _summarize_project(
                project=project,
                dispatches=dispatches,
                executor=executor,
                sync_boards=True,
            )
            self.assertEqual(row["slug"], "demo")
            self.assertTrue(row["board_exists"])
            self.assertGreaterEqual(row["local"]["total"], 1)
            self.assertGreaterEqual(row["kanban"]["total"], 1)
            text = render_fleet_text(
                {
                    "executor": "null",
                    "truth_gate": "null",
                    "project_count": 1,
                    "active_projects": 1 if row["health"] == "active" else 0,
                    "attention_projects": 1 if row["health"] == "attention" else 0,
                    "idle_projects": 1 if row["health"] == "idle" else 0,
                    "projects": [row],
                }
            )
            self.assertIn("demo", text)
            storage.close()


if __name__ == "__main__":
    unittest.main()
