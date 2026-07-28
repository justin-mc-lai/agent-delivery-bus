from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.hermes import HermesAdapter
from agent_delivery_bus.process import CommandResult

from .helpers import RecordingRunner, make_project


class HermesAdapterTests(unittest.TestCase):
    def test_task_contract_uses_public_cli_and_required_limits(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            runner = RecordingRunner(
                [
                    CommandResult(("boards",), 0, json.dumps([{"slug": "adb-demo", "archived": False}]), ""),
                    CommandResult(("create",), 0, json.dumps({"id": "task-1"}), ""),
                ]
            )
            adapter = HermesAdapter(runner)
            adapter.ensure_board(project)
            receipt = adapter.create_task(
                project,
                stage="implement",
                feature="demo-feature",
                body="body",
                idempotency_key="idem",
            )
            self.assertEqual(receipt["task_id"], "task-1")
            create = runner.calls[-1]
            self.assertIn("worktree:" + project.repo, create)
            self.assertIn("agent-delivery-bus", create)
            self.assertIn("2h", create)
            self.assertIn("2", create)
            self.assertIn("idem", create)
            self.assertNotIn("kanban.db", " ".join(create))


if __name__ == "__main__":
    unittest.main()
