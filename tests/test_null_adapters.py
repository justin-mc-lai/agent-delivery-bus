from __future__ import annotations

import json
import subprocess
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.null import NullExecutor, NullTruthGate
from agent_delivery_bus.preflight import Preflight
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import make_project, write_registry


def init_git(repo: str) -> None:
    subprocess.run(["git", "init"], cwd=repo, check=True, capture_output=True)


class NullAdapterTests(unittest.TestCase):
    def test_hermes_free_dispatch_and_reconcile(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            init_git(project.repo)
            registry_path = write_registry(root / "projects.json", [project])
            data = json.loads(registry_path.read_text(encoding="utf-8"))
            data["adapters"] = {"executor": "null", "truth_gate": "null"}
            registry_path.write_text(json.dumps(data), encoding="utf-8")
            registry = ProjectRegistry.load(registry_path)
            storage = Storage(":memory:")
            executor = NullExecutor(auto_complete=True)
            truth = NullTruthGate(auto_pass=False)
            service = DeliveryService(
                registry,
                storage,
                preflight=Preflight(truth, executor),
                executor=executor,
                truth_gate=truth,
            )
            dry = service.dispatch(project_slug="demo", stage="plan", feature="feature", dry_run=True)
            self.assertFalse(dry["blocked"], dry)
            dispatched = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(dispatched["status"], "dispatched", dispatched)
            self.assertEqual(executor.create_count, 1)
            completed = service.reconcile(dispatched["dispatch"]["dispatch_id"])
            self.assertEqual(completed["status"], "completed", completed)
            again = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(again["dispatch"]["dispatch_id"], dispatched["dispatch"]["dispatch_id"])
            self.assertEqual(executor.create_count, 1)
            storage.close()

    def test_implement_still_requires_approval_with_null_backend(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            init_git(project.repo)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            executor = NullExecutor()
            truth = NullTruthGate(auto_pass=True)
            service = DeliveryService(
                registry,
                storage,
                preflight=Preflight(truth, executor),
                executor=executor,
                truth_gate=truth,
            )
            waiting = service.dispatch(project_slug="demo", stage="implement", feature="feature")
            self.assertEqual(waiting["status"], "awaiting_approval", waiting)
            storage.close()


if __name__ == "__main__":
    unittest.main()
