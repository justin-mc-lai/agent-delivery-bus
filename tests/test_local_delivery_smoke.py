"""Local delivery smoke tests (AC-NS-007): null-adapters end-to-end + hermes availability."""

from __future__ import annotations

import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.factory import AdapterResolver
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import PassingPreflight, make_project, write_registry


class LocalDeliverySmokeTests(unittest.TestCase):
    def test_null_adapters_dispatch_reconcile_completed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            subprocess.run(["git", "-C", project.repo, "init", "-q"], check=True)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            resolver = AdapterResolver(
                {
                    "adapters": {
                        "executor": "null",
                        "truth_gate": "null",
                        "binding_profile": "beacon",
                    }
                }
            )
            wired = resolver.global_adapters()
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                executor=wired["executor"],
                truth_gate=wired["truth_gate"],
                adapter_resolver=resolver.for_project,
            )
            dispatched = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(dispatched["status"], "dispatched")
            dispatch_id = dispatched["dispatch"]["dispatch_id"]
            task = next(iter(wired["executor"].tasks.values()))
            self.assertIn("### Evidence spec", task["body"])
            self.assertIn(f"dispatch_id: {dispatch_id}", task["body"])
            reconciled = service.reconcile(dispatch_id)
            self.assertEqual(reconciled["status"], "completed")
            self.assertFalse(reconciled["blocked"])
            storage.close()

    @unittest.skipIf(shutil.which("hermes") is None, "hermes CLI not installed")
    def test_hermes_cli_available_for_registered_projects(self):
        self.assertTrue(shutil.which("hermes"))


if __name__ == "__main__":
    unittest.main()
