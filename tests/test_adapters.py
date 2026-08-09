from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.hermes import HermesAdapter
from agent_delivery_bus.process import CommandResult
from agent_delivery_bus.registry import Project

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

    def test_workspace_kind_dir_uses_main_tree_for_implement(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = root / "content-project"
            repo.mkdir()
            project = Project(
                slug="content-creator",
                title="content-creator",
                project_class="managed",
                repo=str(repo),
                aliases=("creator",),
                dispatchable=True,
                binding_profile="selfmedia-codex",
                metadata={"workspace_kind": "dir"},
            )
            runner = RecordingRunner(
                [
                    CommandResult(("boards",), 0, json.dumps([{"slug": "adb-content-creator", "archived": False}]), ""),
                    CommandResult(("create",), 0, json.dumps({"id": "task-1"}), ""),
                ]
            )
            adapter = HermesAdapter(runner)
            adapter.ensure_board(project)
            adapter.create_task(
                project,
                stage="implement",
                feature="anydoc",
                body="body",
                idempotency_key="idem",
            )
            create = runner.calls[-1]
            self.assertIn("--workspace", create)
            self.assertIn("dir:" + str(repo), create)


if __name__ == "__main__":
    unittest.main()
