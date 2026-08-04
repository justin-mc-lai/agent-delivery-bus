"""Tests for search-boundary-curation (AC-SBC-001..007)."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.boundary import BoundaryService, hermes_boundary_tick_script
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.pending import pending_approval_views
from agent_delivery_bus.storage import Storage


class BoundaryIngestTests(unittest.TestCase):
    def test_ingest_lands_pending_and_rejects_empty_topic(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            row = svc.ingest(
                topic="agent delivery frontiers",
                query_hints=["adb schedule", "hermes cron"],
                sources=["fixture://web"],
                rationale="sweep",
            )
            self.assertEqual(row["status"], "pending")
            self.assertEqual(row["topic"], "agent delivery frontiers")
            self.assertEqual(row["query_hints"], ["adb schedule", "hermes cron"])
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.ingest(topic="  ")
            self.assertEqual(ctx.exception.reason_code, "boundary_topic_required")
            storage.close()


class BoundaryPendingShowTests(unittest.TestCase):
    def test_pending_show(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            first = svc.ingest(topic="a", query_hints=["q1"])
            second = svc.ingest(topic="b")
            pending = svc.pending()
            self.assertEqual({p["id"] for p in pending}, {first["id"], second["id"]})
            shown = svc.show(first["id"])
            self.assertEqual(shown["topic"], "a")
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.show("sbp-missing")
            self.assertEqual(ctx.exception.reason_code, "boundary_not_found")
            storage.close()


class BoundaryDecideTests(unittest.TestCase):
    def test_decide_approve_and_reject(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            ok = svc.ingest(topic="approve-me")
            no = svc.ingest(topic="reject-me")
            approved = svc.decide(ok["id"], actor="apple", decision="approve", note="lgtm")
            self.assertEqual(approved["status"], "approved")
            self.assertEqual(approved["actor"], "apple")
            rejected = svc.decide(no["id"], actor="apple", decision="reject")
            self.assertEqual(rejected["status"], "rejected")
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.decide(ok["id"], actor="apple", decision="approve")
            self.assertEqual(ctx.exception.reason_code, "boundary_already_decided")
            storage.close()


class BoundaryListStatusTests(unittest.TestCase):
    def test_list_status_defaults_to_approved(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            p = svc.ingest(topic="pending-only")
            a = svc.ingest(topic="will-approve")
            r = svc.ingest(topic="will-reject")
            svc.decide(a["id"], actor="apple", decision="approve")
            svc.decide(r["id"], actor="apple", decision="reject")
            active = svc.list()
            self.assertEqual([row["id"] for row in active], [a["id"]])
            self.assertEqual({row["id"] for row in svc.list(status="pending")}, {p["id"]})
            self.assertEqual({row["id"] for row in svc.list(status="rejected")}, {r["id"]})
            self.assertEqual(len(svc.list(status="all")), 3)
            storage.close()


class BoundaryAwaitingViewTests(unittest.TestCase):
    def test_awaiting_view_includes_boundary_pending(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            row = svc.ingest(topic="needs-review")
            views = pending_approval_views(storage)
            match = [v for v in views if v.get("kind") == "boundary_pending"]
            self.assertEqual(len(match), 1)
            self.assertEqual(match[0]["proposal_id"], row["id"])
            self.assertEqual(match[0]["topic"], "needs-review")
            storage.close()


class BoundaryScheduleTickTests(unittest.TestCase):
    def test_schedule_tick_fixture_ingests_without_approve(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            result = svc.run_tick_fixture()
            self.assertFalse(result["auto_approved"])
            self.assertTrue(result["ingested"])
            self.assertTrue(all(item["status"] == "pending" for item in result["ingested"]))
            self.assertEqual(svc.list(), [])
            script = hermes_boundary_tick_script()
            self.assertIn("boundary ingest", script)
            self.assertNotIn("boundary decide", script)
            storage.close()


class BoundaryNoAutoTests(unittest.TestCase):
    def test_no_auto_approve_on_ingest(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.ingest(topic="x", auto_activate=True)
            self.assertEqual(ctx.exception.reason_code, "illegal_boundary_auto_activate")
            blocked = svc.reject_illegal(action="auto_approve")
            self.assertTrue(blocked["blocked"])
            self.assertEqual(blocked["reason_code"], "illegal_boundary_transition")
            storage.close()


class BoundaryIllegalActivateTests(unittest.TestCase):
    def test_illegal_activate_paths(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            blocked = svc.reject_illegal(action="ingest_active")
            self.assertEqual(blocked["status"], "blocked")
            skip = svc.reject_illegal(action="activate_skip_pending")
            self.assertTrue(skip["blocked"])
            storage.close()


class BoundarySkipPendingTests(unittest.TestCase):
    def test_skip_pending_is_illegal(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            blocked = svc.reject_illegal(action="activate_skip_pending")
            self.assertEqual(blocked["reason_code"], "illegal_boundary_transition")
            # Cannot force approved without decide
            row = svc.ingest(topic="still-pending")
            self.assertEqual(row["status"], "pending")
            self.assertEqual(svc.list(status="approved"), [])
            storage.close()


if __name__ == "__main__":
    unittest.main()
