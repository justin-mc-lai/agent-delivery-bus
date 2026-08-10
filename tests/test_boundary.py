"""Tests for search-boundary-curation (AC-SBC-001..011)."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.boundary import (
    DEFAULT_ACCOUNT_PROFILE_REF,
    DEFAULT_PROJECT_PROFILE_REF,
    BoundaryService,
    hermes_boundary_tick_script,
    daily_topic_batch,
    load_vertical_profile,
)
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.pending import pending_approval_views
from agent_delivery_bus.storage import Storage

_PROFILE_KW = {
    "project_profile_ref": DEFAULT_PROJECT_PROFILE_REF,
    "account_profile_ref": DEFAULT_ACCOUNT_PROFILE_REF,
    "rationale": "示例号·开源 AI / AI Spec 价值选题",
}


class BoundaryIngestTests(unittest.TestCase):
    def test_ingest_lands_pending_and_rejects_empty_topic(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            row = svc.ingest(
                topic="GitHub 开源 AI agent 边界整理",
                query_hints=["adb schedule", "hermes cron", "github ai"],
                sources=["fixture://web"],
                **_PROFILE_KW,
            )
            self.assertEqual(row["status"], "pending")
            self.assertEqual(row["topic"], "GitHub 开源 AI agent 边界整理")
            self.assertEqual(row["project_profile_ref"], DEFAULT_PROJECT_PROFILE_REF)
            self.assertEqual(row["account_profile_ref"], DEFAULT_ACCOUNT_PROFILE_REF)
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.ingest(topic="  ", **_PROFILE_KW)
            self.assertEqual(ctx.exception.reason_code, "boundary_topic_required")
            storage.close()


class BoundaryPendingShowTests(unittest.TestCase):
    def test_pending_show(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            first = svc.ingest(topic="开源 AI 库 A", query_hints=["github"], **_PROFILE_KW)
            second = svc.ingest(topic="AI Spec B", query_hints=["spec"], **_PROFILE_KW)
            pending = svc.pending()
            self.assertEqual({p["id"] for p in pending}, {first["id"], second["id"]})
            shown = svc.show(first["id"])
            self.assertEqual(shown["topic"], "开源 AI 库 A")
            self.assertEqual(shown["project_profile_ref"], DEFAULT_PROJECT_PROFILE_REF)
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.show("sbp-missing")
            self.assertEqual(ctx.exception.reason_code, "boundary_not_found")
            storage.close()

    def test_list_awaiting_show(self):
        # alias for TC-SBC-002 filter name
        self.test_pending_show()


class BoundaryDecideTests(unittest.TestCase):
    def test_decide_approve_and_reject(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            ok = svc.ingest(topic="approve-me github ai", **_PROFILE_KW)
            no = svc.ingest(topic="reject-me llm ops", **_PROFILE_KW)
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
            p = svc.ingest(topic="pending-only github ai", **_PROFILE_KW)
            a = svc.ingest(topic="will-approve ai spec", **_PROFILE_KW)
            r = svc.ingest(topic="will-reject opensource llm", **_PROFILE_KW)
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
            row = svc.ingest(topic="needs-review github agent", **_PROFILE_KW)
            views = pending_approval_views(storage)
            match = [v for v in views if v.get("kind") == "boundary_pending"]
            self.assertEqual(len(match), 1)
            self.assertEqual(match[0]["proposal_id"], row["id"])
            self.assertEqual(match[0]["topic"], "needs-review github agent")
            storage.close()


class BoundaryScheduleTickTests(unittest.TestCase):
    def test_schedule_tick_fixture_ingests_without_approve(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            result = svc.run_tick_fixture()
            self.assertFalse(result["auto_approved"])
            self.assertEqual(len(result["ingested"]), 5)
            self.assertTrue(all(item["status"] == "pending" for item in result["ingested"]))
            self.assertEqual(svc.list(), [])
            script = hermes_boundary_tick_script()
            self.assertIn("ingest", script)
            self.assertIn("示例号", script)
            self.assertNotIn("表情包", script)
            self.assertNotIn('[adb, "boundary", "decide"', script)
            self.assertNotIn('adb, "boundary", "decide"', script)
            storage.close()

    def test_tick_script_resolves_python_and_fails_closed_no_fallback(self):
        script = hermes_boundary_tick_script()
        # Cron PATH puts the Hermes venv first (no adb package); the script must
        # resolve an interpreter that can import agent_delivery_bus.
        self.assertIn('PYTHON_BIN="${PYTHON_BIN:-}"', script)
        self.assertIn('"$PYTHON_BIN" - <<', script)
        # Silent hardcoded fallback was the daily-duplicate root cause; it must
        # not exist anymore and failure must exit non-zero.
        self.assertNotIn("本周值得盯的 GitHub 开源 AI Agent 框架更新", script)
        self.assertIn("refusing to send fallback duplicates", script)
        self.assertIn("sys.exit(1)", script)


class BoundaryNoAutoTests(unittest.TestCase):
    def test_no_auto_approve_on_ingest(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.ingest(topic="x github ai", auto_activate=True, **_PROFILE_KW)
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
            row = svc.ingest(topic="still-pending github ai", **_PROFILE_KW)
            self.assertEqual(row["status"], "pending")
            self.assertEqual(svc.list(status="approved"), [])
            storage.close()

    def test_skip_awaiting_is_illegal(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            blocked = svc.reject_illegal(action="activate_skip_awaiting")
            self.assertEqual(blocked["reason_code"], "illegal_boundary_transition")
            storage.close()


class BoundaryProfileRefsRequiredTests(unittest.TestCase):
    def test_profile_refs_required(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.ingest(topic="github ai without refs", rationale="value")
            self.assertEqual(ctx.exception.reason_code, "boundary_profile_ref_required")
            with self.assertRaises(DeliveryBusError) as ctx2:
                svc.ingest(
                    topic="github ai missing account",
                    project_profile_ref=DEFAULT_PROJECT_PROFILE_REF,
                    rationale="value",
                )
            self.assertEqual(ctx2.exception.reason_code, "boundary_profile_ref_required")
            storage.close()


class BoundaryVerticalProfilesAuditableTests(unittest.TestCase):
    def test_vertical_profiles_auditable(self):
        project = load_vertical_profile(DEFAULT_PROJECT_PROFILE_REF)
        account = load_vertical_profile(DEFAULT_ACCOUNT_PROFILE_REF)
        self.assertEqual(project["id"], DEFAULT_PROJECT_PROFILE_REF)
        self.assertIn("github-oss-ai", project["themes"])
        self.assertEqual(account["vertical"], "oss-picks")
        self.assertEqual(account["draft_tab_meaning"], "image_post")
        self.assertIn("表情包", account["out_of_scope"])


class BoundaryVerticalGateTests(unittest.TestCase):
    def test_vertical_gate_rejects_sticker_drift(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.ingest(
                    topic="周一情绪贴图：打工人开工防崩溃表情包合集",
                    query_hints=["打工人表情包"],
                    **_PROFILE_KW,
                )
            self.assertEqual(ctx.exception.reason_code, "vertical_gate_rejected")
            with self.assertRaises(DeliveryBusError) as ctx2:
                svc.ingest(
                    topic="情侣情感漫连载选题",
                    query_hints=["情感漫"],
                    **_PROFILE_KW,
                )
            self.assertEqual(ctx2.exception.reason_code, "vertical_gate_rejected")
            storage.close()


class BoundaryDemoTopicsInVerticalTests(unittest.TestCase):
    def test_demo_topics_in_vertical(self):
        batch = daily_topic_batch(day_index=1, count=5)
        self.assertEqual(len(batch), 5)
        blob = " ".join(
            f"{item['topic']} {' '.join(item['query_hints'])} {item['rationale']}" for item in batch
        )
        for bad in ("表情包", "情侣", "宠物", "闺蜜"):
            self.assertNotIn(bad, blob)
        self.assertTrue(any(tok in blob.lower() for tok in ("github", "ai", "spec", "开源", "llm", "agent")))
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            created = [svc.ingest(**item) for item in batch]
            self.assertTrue(all(row.get("provenance") == "in-vertical-fixture" for row in created))
            self.assertTrue(all(row.get("account_profile_ref") == DEFAULT_ACCOUNT_PROFILE_REF for row in created))
            storage.close()


if __name__ == "__main__":
    unittest.main()
