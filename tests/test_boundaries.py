from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import FakeBeacon, FakeHermes, PassingPreflight, make_project, write_registry


class BoundaryTests(unittest.TestCase):
    def test_release_is_disabled_before_hermes_side_effect(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            hermes = FakeHermes()
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                executor=hermes,
                truth_gate=FakeBeacon(),
            )
            with self.assertRaises(DeliveryBusError) as raised:
                service.dispatch(project_slug="demo", stage="release", feature="feature")
            self.assertEqual(raised.exception.reason_code, "stage_not_enabled")
            self.assertEqual(hermes.create_count, 0)
            storage.close()

    def test_source_has_no_hermes_db_or_auto_repair_command(self):
        source = Path(__file__).resolve().parents[1] / "src" / "agent_delivery_bus"
        text = "\n".join(path.read_text(encoding="utf-8") for path in source.rglob("*.py"))
        self.assertNotIn("kanban.db", text)
        self.assertNotIn('"setup-context"', text)
        self.assertNotIn('"sync-materials"', text)


if __name__ == "__main__":
    unittest.main()
