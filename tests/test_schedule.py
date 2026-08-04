"""Tests for vision-flywheel schedule heartbeat layer (AC-FLY-001..007)."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.schedule import ScheduleService, hermes_cron_tick_script
from agent_delivery_bus.storage import Storage


class ScheduleRegisterTests(unittest.TestCase):
    def test_register_writes_entry_and_is_idempotent(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = ScheduleService(storage)
            first = svc.register(
                slug="daily-oss-pick",
                command="bash scripts/daily-trending.sh",
                engine="hermes",
                cron_expr="0 10 * * *",
                quota_limit=10,
            )
            self.assertEqual(first["slug"], "daily-oss-pick")
            self.assertEqual(first["engine"], "hermes")
            second = svc.register(
                slug="daily-oss-pick",
                command="bash scripts/daily-trending.sh --writeback",
                engine="hermes",
                cron_expr="0 11 * * *",
                quota_limit=5,
            )
            self.assertEqual(second["command"], "bash scripts/daily-trending.sh --writeback")
            self.assertEqual(second["quota_limit"], 5)
            self.assertEqual(len(svc.list_entries()), 1)
            storage.close()

    def test_register_rejects_unknown_engine(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = ScheduleService(storage)
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.register(
                    slug="x",
                    command="echo hi",
                    engine="cloud-cron",
                    cron_expr="* * * * *",
                    quota_limit=1,
                )
            self.assertEqual(ctx.exception.reason_code, "schedule_engine_unknown")
            storage.close()


class ScheduleListShowTests(unittest.TestCase):
    def test_list_show_includes_quota_status(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = ScheduleService(storage)
            svc.register(
                slug="a",
                command="true",
                engine="hermes",
                cron_expr="0 * * * *",
                quota_limit=3,
            )
            entries = svc.list_entries()
            self.assertEqual(len(entries), 1)
            self.assertIn("quota", entries[0])
            self.assertEqual(entries[0]["quota"]["slots_allowed"], 3)
            shown = svc.show("a")
            self.assertEqual(shown["slug"], "a")
            self.assertEqual(shown["quota"]["remaining"], 3)
            storage.close()


class ScheduleShouldRunTests(unittest.TestCase):
    def test_should_run_quota_and_health_gates(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = ScheduleService(storage)
            svc.register(
                slug="job",
                command="true",
                engine="hermes",
                cron_expr="0 * * * *",
                quota_limit=1,
            )
            ok = svc.should_run("job")
            self.assertEqual(ok["action"], "run")
            self.assertFalse(ok["blocked"])

            run = svc.begin_run("job", source="heartbeat")
            svc.complete_run(run["run_id"], evidence_refs=[".beacon/evidence/x.json"])
            throttled = svc.should_run("job")
            self.assertEqual(throttled["action"], "blocked")
            self.assertEqual(throttled["reason_code"], "quota_exhausted")
            self.assertEqual(throttled["status"], "throttled")

            svc.register(
                slug="sick",
                command="true",
                engine="hermes",
                cron_expr="0 * * * *",
                quota_limit=2,
                health="unhealthy",
            )
            unhealthy = svc.should_run("sick")
            self.assertEqual(unhealthy["action"], "blocked")
            self.assertEqual(unhealthy["reason_code"], "health_gate_failed")
            # deterministic: no LLM fields
            self.assertNotIn("llm", unhealthy)
            storage.close()


class ScheduleQuotaTests(unittest.TestCase):
    def test_quota_spend_after_evidence_only_whitelist_source(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = ScheduleService(storage)
            svc.register(
                slug="q",
                command="true",
                engine="hermes",
                cron_expr="0 * * * *",
                quota_limit=2,
            )
            run = svc.begin_run("q", source="controller")
            pending = svc.complete_run(run["run_id"], evidence_refs=[])
            self.assertEqual(pending["status"], "reconciling")
            self.assertEqual(pending["quota_spent"], 0)
            self.assertEqual(svc.show("q")["quota"]["slots_spent"], 0)

            done = svc.complete_run(run["run_id"], evidence_refs=["ev/1.json"])
            self.assertEqual(done["status"], "completed")
            self.assertEqual(done["quota_spent"], 1)
            self.assertEqual(svc.show("q")["quota"]["slots_spent"], 1)

            with self.assertRaises(DeliveryBusError) as ctx:
                svc.begin_run("q", source="random-bot")
            self.assertEqual(ctx.exception.reason_code, "schedule_source_not_allowed")
            storage.close()


class ScheduleLedgerTests(unittest.TestCase):
    def test_ledger_appends_event_stream(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = ScheduleService(storage)
            svc.register(
                slug="led",
                command="true",
                engine="hermes",
                cron_expr="0 * * * *",
                quota_limit=5,
            )
            run = svc.begin_run("led", source="heartbeat")
            svc.complete_run(run["run_id"], evidence_refs=["ev.json"])
            rows = svc.ledger(slug="led")
            self.assertEqual(len(rows), 1)
            self.assertEqual(rows[0]["entry_slug"], "led")
            self.assertEqual(rows[0]["status"], "completed")
            self.assertEqual(rows[0]["evidence_refs"], ["ev.json"])
            self.assertEqual(rows[0]["quota_spent"], 1)
            script = hermes_cron_tick_script().lower()
            self.assertIn("hermes", script)
            self.assertIn("should-run", script)
            storage.close()


class ScheduleReconcileTests(unittest.TestCase):
    def test_reconcile_keeps_reconciling_until_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = ScheduleService(storage)
            svc.register(
                slug="rec",
                command="true",
                engine="hermes",
                cron_expr="0 * * * *",
                quota_limit=3,
            )
            run = svc.begin_run("rec", source="heartbeat")
            first = svc.reconcile_run(run["run_id"], evidence_refs=[])
            self.assertEqual(first["status"], "reconciling")
            self.assertTrue(first["blocked"])
            second = svc.reconcile_run(run["run_id"], evidence_refs=[".beacon/evidence/ok.json"])
            self.assertEqual(second["status"], "completed")
            self.assertFalse(second["blocked"])
            storage.close()


class ScheduleNoAutoTests(unittest.TestCase):
    def test_no_auto_approve_or_dispatch_from_heartbeat(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = ScheduleService(storage)
            blocked = svc.reject_illegal(action="dispatch")
            self.assertTrue(blocked["blocked"])
            self.assertEqual(blocked["reason_code"], "illegal_heartbeat_action")
            blocked2 = svc.reject_illegal(action="approve", from_state="running")
            self.assertEqual(blocked2["reason_code"], "illegal_heartbeat_action")
            storage.close()


class ScheduleIllegalDispatchTests(unittest.TestCase):
    def test_illegal_dispatch_from_checking(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = ScheduleService(storage)
            result = svc.reject_illegal(action="auto_dispatch", from_state="checking")
            self.assertEqual(result["fsm_state"], "blocked")
            self.assertEqual(result["attempted_action"], "auto_dispatch")
            storage.close()


class ScheduleSkipShouldRunTests(unittest.TestCase):
    def test_skip_should_run_is_illegal(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = ScheduleService(storage)
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.assert_not_skip_should_run(skipped_should_run=True)
            self.assertEqual(ctx.exception.reason_code, "illegal_skip_should_run")
            storage.close()


if __name__ == "__main__":
    unittest.main()
