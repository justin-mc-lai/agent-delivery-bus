from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.memory import InMemoryMemoryAdapter, enforce_scope
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import FakeBeacon, FakeHermes, PassingPreflight, make_project, write_registry


class MemoryAdapterDispatchTests(unittest.TestCase):
    def test_recall_before_dispatch(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            memory = InMemoryMemoryAdapter()
            memory.seed("demo", "prior decision about feature")
            hermes = FakeHermes()
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                executor=hermes,
                truth_gate=FakeBeacon(),
                memory=memory,
            )
            result = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(result["status"], "dispatched")
            self.assertEqual(len(memory.recall_calls), 1)
            self.assertEqual(memory.recall_calls[0]["project_slug"], "demo")
            # Adapter must live outside core modules (boundary checked in test_boundaries style).
            core_text = "\n".join(
                (Path(__file__).resolve().parents[1] / "src" / "agent_delivery_bus" / name).read_text(
                    encoding="utf-8"
                )
                for name in ("registry.py", "storage.py", "approvals.py")
            )
            self.assertNotIn("agentmemory", core_text)
            self.assertNotIn("AgentMemoryAdapter", core_text)
            storage.close()

    def test_writeback_after_reconcile(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            memory = InMemoryMemoryAdapter()
            hermes = FakeHermes(remote_status="completed")
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                executor=hermes,
                truth_gate=FakeBeacon(closure_pass=True),
                memory=memory,
            )
            dispatched = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            dispatch_id = dispatched["dispatch"]["dispatch_id"]
            reconciled = service.reconcile(dispatch_id)
            self.assertEqual(reconciled["status"], "completed")
            self.assertTrue(reconciled["memory_writeback"]["ok"])
            self.assertEqual(len(memory.writeback_calls), 1)
            call = memory.writeback_calls[0]
            self.assertEqual(call["project_slug"], "demo")
            self.assertEqual(call["stage"], "plan")
            self.assertEqual(call["feature"], "feature")
            self.assertEqual(call["dispatch_id"], dispatch_id)

            memory.fail_writeback = True
            hermes.remote_status = "failed"
            # New dispatch path for failure writeback without erasing status.
            hermes2 = FakeHermes(remote_status="completed")
            memory2 = InMemoryMemoryAdapter()
            memory2.fail_writeback = True
            service2 = DeliveryService(
                registry,
                Storage(":memory:"),
                preflight=PassingPreflight(),
                executor=hermes2,
                truth_gate=FakeBeacon(closure_pass=True),
                memory=memory2,
            )
            d2 = service2.dispatch(project_slug="demo", stage="plan", feature="feature-wb")
            r2 = service2.reconcile(d2["dispatch"]["dispatch_id"])
            self.assertEqual(r2["status"], "completed")
            self.assertFalse(r2["memory_writeback"]["ok"])
            self.assertEqual(r2["dispatch"]["state"], "completed")
            storage.close()


class MemoryAclTests(unittest.TestCase):
    def test_cross_project_acl(self):
        memory = InMemoryMemoryAdapter()
        memory.seed("project-a", "secret from A shared-token")
        memory.seed("project-b", "note for B only")
        ok = memory.recall(project_slug="project-b", query="note for B")
        self.assertTrue(all(r["project_slug"] == "project-b" for r in ok["records"]))
        with self.assertRaises(DeliveryBusError) as raised:
            memory.recall(project_slug="project-b", query="secret from A")
        self.assertEqual(raised.exception.reason_code, "memory_acl_denied")

    def test_illegal_cross_project(self):
        with self.assertRaises(DeliveryBusError) as raised:
            enforce_scope(
                [{"project": "project-a", "content": "x"}, {"project": "project-b", "content": "y"}],
                project_slug="project-b",
            )
        self.assertEqual(raised.exception.reason_code, "memory_acl_denied")

        # writeback failure must not erase reconcile — covered via service path above;
        # illegal transition assertion: failed writeback leaves completed state.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            memory = InMemoryMemoryAdapter()
            memory.fail_writeback = True
            hermes = FakeHermes(remote_status="completed")
            service = DeliveryService(
                registry,
                Storage(":memory:"),
                preflight=PassingPreflight(),
                executor=hermes,
                truth_gate=FakeBeacon(closure_pass=True),
                memory=memory,
            )
            dispatched = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            result = service.reconcile(dispatched["dispatch"]["dispatch_id"])
            self.assertEqual(result["status"], "completed")
            self.assertFalse(result["memory_writeback"]["ok"])


if __name__ == "__main__":
    unittest.main()
