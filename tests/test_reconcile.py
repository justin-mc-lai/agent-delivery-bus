from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import FakeBeacon, FakeHermes, PassingPreflight, make_project, write_registry


class ReconcileTests(unittest.TestCase):
    def test_worker_success_requires_beacon_closure(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            hermes = FakeHermes(remote_status="done")
            beacon = FakeBeacon(closure_pass=False)
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                hermes=hermes,
                beacon=beacon,
            )
            dispatched = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            first = service.reconcile(dispatched["dispatch"]["dispatch_id"])
            self.assertEqual(first["status"], "reconciling")
            self.assertEqual(first["reason_code"], "beacon_evidence_incomplete")
            beacon.closure_pass = True
            second = service.reconcile(dispatched["dispatch"]["dispatch_id"])
            self.assertEqual(second["status"], "completed")
            storage.close()


if __name__ == "__main__":
    unittest.main()
