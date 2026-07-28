from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.null import NullExecutor, NullTruthGate
from agent_delivery_bus.cli import build_board_status, render_board_status_text
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import make_project, write_registry
from .test_null_adapters import init_git


class BoardStatusTests(unittest.TestCase):
    def test_expands_hermes_style_columns(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            init_git(project.repo)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            executor = NullExecutor(auto_complete=False)
            truth = NullTruthGate(auto_pass=True)
            service = DeliveryService(
                registry,
                storage,
                executor=executor,
                truth_gate=truth,
            )
            service.dispatch(project_slug="demo", stage="plan", feature="alpha")
            # synthesize mixed kanban columns on the null board
            board = executor.board_for(project)
            executor.tasks.clear()
            samples = [
                ("t1", "todo", "Plan alpha"),
                ("t2", "ready", "Ready beta"),
                ("t3", "running", "Implement gamma"),
                ("t4", "blocked", "Blocked delta"),
                ("t5", "done", "Done epsilon"),
            ]
            for task_id, status, title in samples:
                executor.tasks[task_id] = {
                    "id": task_id,
                    "board": board,
                    "status": status,
                    "state": status,
                    "title": title,
                    "assignee": "coding",
                }
            row = build_board_status(
                project=project,
                executor=executor,
                dispatches=storage.list_dispatches(project_slug="demo"),
                sync_board=True,
                limit=10,
            )
            self.assertEqual(row["simplified"]["todo"], 1)
            self.assertEqual(row["simplified"]["doing"], 2)
            self.assertEqual(row["simplified"]["blocked"], 1)
            self.assertEqual(row["simplified"]["done"], 1)
            self.assertEqual(row["columns"]["running"]["count"], 1)
            self.assertEqual(row["columns"]["blocked"]["tasks"][0]["title"], "Blocked delta")
            text = render_board_status_text(
                {
                    "executor": "null",
                    "truth_gate": "null",
                    "boards": [row],
                }
            )
            self.assertIn("[blocked]", text)
            self.assertIn("Blocked delta", text)
            storage.close()


if __name__ == "__main__":
    unittest.main()
