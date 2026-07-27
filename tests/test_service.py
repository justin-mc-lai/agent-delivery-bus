from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import FakeBeacon, FakeHermes, PassingPreflight, make_project, write_registry


class DispatchIdempotencyTests(unittest.TestCase):
    def test_duplicate_request_creates_one_hermes_task(self):
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
                hermes=hermes,
                beacon=FakeBeacon(),
            )
            first = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            second = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(first["dispatch"]["dispatch_id"], second["dispatch"]["dispatch_id"])
            self.assertEqual(hermes.create_count, 1)
            storage.close()

    def test_forced_key_conflict_blocks(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                hermes=FakeHermes(),
                beacon=FakeBeacon(),
            )
            service.dispatch(
                project_slug="demo",
                stage="plan",
                feature="one",
                forced_idempotency_key="fixed",
            )
            with self.assertRaises(DeliveryBusError) as raised:
                service.dispatch(
                    project_slug="demo",
                    stage="plan",
                    feature="two",
                    forced_idempotency_key="fixed",
                )
            self.assertEqual(raised.exception.reason_code, "idempotency_conflict")
            storage.close()


class DispatchStateMachineTests(unittest.TestCase):
    def test_implement_requires_approval_and_consumes_once(self):
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
                hermes=hermes,
                beacon=FakeBeacon(),
            )
            waiting = service.dispatch(project_slug="demo", stage="implement", feature="feature")
            self.assertEqual(waiting["status"], "awaiting_approval")
            issued = service.approvals.issue(
                actor="apple",
                project_slug="demo",
                stage="implement",
                feature="feature",
                ttl_seconds=300,
            )
            dispatched = service.dispatch(
                project_slug="demo",
                stage="implement",
                feature="feature",
                approval_token=issued["token"],
            )
            self.assertEqual(dispatched["status"], "dispatched")
            self.assertEqual(service.approvals.get(issued["approval_id"])["state"], "consumed")
            storage.close()

    def test_invalid_transition_is_rejected(self):
        storage = Storage(":memory:")
        row, _ = storage.create_dispatch(
            idempotency_key="k",
            request_hash="h",
            request={},
            project_slug="demo",
            stage="plan",
            feature="feature",
        )
        with self.assertRaises(DeliveryBusError) as raised:
            storage.transition(
                row["dispatch_id"],
                expected_from="queued",
                to_state="completed",
                event_type="skip",
            )
        self.assertEqual(raised.exception.reason_code, "invalid_transition")
        storage.close()

    def test_blocked_request_retries_same_dispatch_after_preflight_repair(self):
        class RepairablePreflight:
            blocked = True

            def run(self, project, *, stage):
                return {
                    "status": "blocked" if self.blocked else "pass",
                    "blocked": self.blocked,
                    "reason_code": "beacon_context_invalid" if self.blocked else "",
                    "resume_action": "repair context" if self.blocked else "",
                    "checks": [],
                }

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            hermes = FakeHermes()
            preflight = RepairablePreflight()
            service = DeliveryService(
                registry,
                storage,
                preflight=preflight,
                hermes=hermes,
                beacon=FakeBeacon(),
            )
            blocked = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            preflight.blocked = False
            retried = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(blocked["dispatch"]["dispatch_id"], retried["dispatch"]["dispatch_id"])
            self.assertEqual(retried["status"], "dispatched")
            self.assertEqual(hermes.create_count, 1)
            event_types = [event["event_type"] for event in retried["dispatch"]["events"]]
            self.assertIn("retry", event_types)
            storage.close()


if __name__ == "__main__":
    unittest.main()
