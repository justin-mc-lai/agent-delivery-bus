from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.cli import main
from agent_delivery_bus.pending import pending_approval_views, render_pending_channel
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import FakeBeacon, FakeHermes, PassingPreflight, make_project, write_registry


class PendingApprovalsTests(unittest.TestCase):
    def test_pending_list_and_feishu_payload(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                executor=FakeHermes(),
                truth_gate=FakeBeacon(),
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
            views = pending_approval_views(storage, project_slug="demo")
            self.assertTrue(any(v["kind"] == "awaiting_dispatch" for v in views))
            self.assertTrue(any(v["kind"] == "issued_token" for v in views))
            for item in views:
                self.assertIn("project", item)
                self.assertIn("stage", item)
                self.assertIn("feature", item)
                self.assertIn("expires_at", item)
            feishu = render_pending_channel(views, channel="feishu")
            self.assertEqual(feishu["channel"], "feishu")
            self.assertTrue(feishu["text"].startswith("下一步："))
            self.assertIn("状态：待拍板", feishu["text"])
            self.assertEqual(issued["state"], "issued")
            storage.close()

    def test_post_approve_dispatch(self):
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
            service.dispatch(project_slug="demo", stage="implement", feature="feature")
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

    def test_cli_assign_candidates_json(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            config = write_registry(root / "projects.json", [project])
            db = root / "bus.sqlite3"
            code = main(
                [
                    "--config",
                    str(config),
                    "--db",
                    str(db),
                    "assign",
                    "candidates",
                    "--feature",
                    "feat",
                    "--json",
                ]
            )
            self.assertEqual(code, 0)


if __name__ == "__main__":
    unittest.main()
