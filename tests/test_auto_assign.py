from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.assign import AssignmentScorer
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import FakeBeacon, FakeHermes, PassingPreflight, make_project, write_registry


class AutoAssignTests(unittest.TestCase):
    def test_candidates_only(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            hermes = FakeHermes()
            scorer = AssignmentScorer(registry)
            rows = scorer.candidates(stage="implement", feature="feat-x")
            scorer.assert_candidates_only(rows)
            self.assertGreaterEqual(len(rows), 1)
            self.assertNotIn("task_id", rows[0])
            self.assertEqual(hermes.create_count, 0)
            # Scoring must not touch executor.
            self.assertTrue(all("task_id" not in row for row in rows))

    def test_approve_still_required(self):
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
            scorer = AssignmentScorer(registry)
            candidates = scorer.candidates(stage="implement", feature="feature", project_slug="demo")
            self.assertTrue(candidates[0]["requires_approval"])
            waiting = service.dispatch(project_slug="demo", stage="implement", feature="feature")
            self.assertEqual(waiting["reason_code"], "approval_required")
            self.assertEqual(hermes.create_count, 0)
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
            self.assertEqual(hermes.create_count, 1)
            storage.close()

    def test_illegal_skip_approve(self):
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
            scorer = AssignmentScorer(registry)
            rows = scorer.candidates(stage="implement", feature="feature")
            # Illegal: candidates_ready -> dispatched without approve
            waiting = service.dispatch(project_slug="demo", stage="implement", feature="feature")
            self.assertEqual(waiting["status"], "awaiting_approval")
            self.assertEqual(hermes.create_count, 0)
            # Illegal: scorer trying to carry a token/task
            poisoned = [{**rows[0], "task_id": "t-1"}]
            with self.assertRaises(DeliveryBusError) as raised:
                scorer.assert_candidates_only(poisoned)
            self.assertEqual(raised.exception.reason_code, "illegal_assign_side_effect")
            poisoned_token = [{**rows[0], "approval_token": "adb1_x"}]
            with self.assertRaises(DeliveryBusError) as raised2:
                scorer.assert_candidates_only(poisoned_token)
            self.assertEqual(raised2.exception.reason_code, "illegal_assign_side_effect")
            storage.close()


if __name__ == "__main__":
    unittest.main()
