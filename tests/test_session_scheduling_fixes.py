"""Regression tests for the 1→6 session-scheduling fixes.

Covers: session-aware executor resolution + mismatch fail-closed, hermes
session receipt / assignee mapping, business-only idempotency keys,
host-session-free identity, decoupled channel delivery, and async pi runs
with durable running/failed receipts.
"""

from __future__ import annotations

import json
import subprocess
import tempfile
import time
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.channel import HermesChannelAdapter
from agent_delivery_bus.adapters.hermes import HermesAdapter
from agent_delivery_bus.adapters.pi import PiExecutorAdapter, PiRunLedger
from agent_delivery_bus.errors import CommandTimedOut, DeliveryBusError
from agent_delivery_bus.process import CommandResult
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.session import SessionRegistry, session_id_for
from agent_delivery_bus.storage import Storage

from .helpers import FakeHermes, FakeTruthGate, PassingPreflight, RecordingRunner, make_project, write_registry
from .test_pi_executor import FakeResult, FakeRunner


class SessionAwareResolverTests(unittest.TestCase):
    def _service(self, tmp: Path, *, resolver) -> DeliveryService:
        registry = ProjectRegistry.load(
            write_registry(tmp / "projects.json", [make_project(tmp)]),
            validate_paths=False,
        )
        repo = Path(registry.list()[0].repo)
        subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
        executor = FakeHermes()
        gate = FakeTruthGate(closure_pass=True)
        return DeliveryService(
            registry,
            Storage(":memory:"),
            executor=executor,
            truth_gate=gate,
            preflight=PassingPreflight(),
            adapter_resolver=resolver,
        )

    def test_bound_pi_overrides_adapter_choice(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fake = FakeHermes()
            gate = FakeTruthGate(closure_pass=True)

            def resolver(project, stage="", target_executor=""):
                if str(target_executor or "").strip().lower() == "pi":
                    return {
                        "executor": fake,
                        "truth_gate": gate,
                        "binding_profile": "beacon",
                        "executor_name": "pi",
                        "truth_gate_name": "fake",
                    }
                return {
                    "executor": fake,
                    "truth_gate": gate,
                    "binding_profile": "beacon",
                    "executor_name": "hermes",
                    "truth_gate_name": "fake",
                }

            service = self._service(root, resolver=resolver)
            sessions = SessionRegistry(service.storage)
            sessions.bind(
                channel="feishu",
                channel_thread="oc_1",
                actor_id="open_1",
                host_session="h1",
                target_executor="pi",
                target_session="fixed:pi-thread-1",
            )
            result = service.dispatch(
                project_slug="demo",
                stage="goal",
                feature="f",
                channel="feishu",
                channel_thread="oc_1",
                actor_id="open_1",
                host_session_ref="h1",
                target_session_ref="fixed:pi-thread-1",
                dry_run=True,
            )
            self.assertEqual(result["request"]["target_executor"], "pi")
            self.assertEqual(result["request"]["resolution_source"], "binding")
            self.assertEqual(result["request"]["target_session_ref"], "pi-thread-1")
            self.assertTrue(result["request"]["lease_required"])

    def test_legacy_resolver_mismatch_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fake = FakeHermes()
            gate = FakeTruthGate(closure_pass=True)

            # Legacy resolver signature: does not accept target_executor.
            def resolver(project, stage=""):
                return {
                    "executor": fake,
                    "truth_gate": gate,
                    "binding_profile": "beacon",
                    "executor_name": "hermes",
                    "truth_gate_name": "fake",
                }

            service = self._service(root, resolver=resolver)
            with self.assertRaises(DeliveryBusError) as ctx:
                service.dispatch(
                    project_slug="demo",
                    stage="goal",
                    feature="f",
                    channel="feishu",
                    channel_thread="oc_1",
                    target_executor="pi",
                )
            self.assertEqual(ctx.exception.reason_code, "executor_mismatch")


class HermesSessionAndAssigneeTests(unittest.TestCase):
    def test_create_task_returns_session_ref(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            runner = RecordingRunner(
                [
                    CommandResult(("boards",), 0, json.dumps([{"slug": "adb-demo", "archived": False}]), ""),
                    CommandResult(("create",), 0, json.dumps({"id": "task-1"}), ""),
                ]
            )
            adapter = HermesAdapter(runner, which_command=lambda _n: "/usr/local/bin/hermes")
            adapter.ensure_board(project)
            receipt = adapter.create_task(
                project,
                stage="plan",
                feature="f",
                body="b",
                idempotency_key="k",
                session_id="sess_x",
            )
            self.assertEqual(receipt["session_ref"], "sess_x")

    def test_codex_target_maps_to_assignee(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fake = FakeHermes()
            gate = FakeTruthGate(closure_pass=True)

            def resolver(project, stage="", target_executor=""):
                return {
                    "executor": fake,
                    "truth_gate": gate,
                    "binding_profile": "beacon",
                    "executor_name": "hermes",
                    "truth_gate_name": "fake",
                }

            registry = ProjectRegistry.load(
                write_registry(root / "projects.json", [make_project(root)]),
                validate_paths=False,
            )
            repo = Path(registry.list()[0].repo)
            subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
            service = DeliveryService(
                registry,
                Storage(":memory:"),
                executor=fake,
                truth_gate=gate,
                preflight=PassingPreflight(),
                adapter_resolver=resolver,
            )
            result = service.dispatch(
                project_slug="demo",
                stage="goal",
                feature="f",
                channel="feishu",
                channel_thread="oc_1",
                target_executor="codex",
            )
            self.assertEqual(result["status"], "dispatched")
            self.assertEqual(fake.last_assignee, "codex")


class IdempotencyAndIdentityTests(unittest.TestCase):
    def test_host_session_excluded_from_identity(self):
        self.assertEqual(
            session_id_for(channel="feishu", channel_thread="oc_1", actor_id="open_1", host_session="h1"),
            session_id_for(channel="feishu", channel_thread="oc_1", actor_id="open_1", host_session="h2"),
        )
        self.assertNotEqual(
            session_id_for(channel="feishu", channel_thread="oc_1", actor_id="open_1", host_session="h1"),
            session_id_for(channel="feishu", channel_thread="oc_2", actor_id="open_1", host_session="h1"),
        )

    def test_same_business_task_reuses_dispatch_across_threads(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fake = FakeHermes()
            gate = FakeTruthGate(closure_pass=True)

            def resolver(project, stage="", target_executor=""):
                return {
                    "executor": fake,
                    "truth_gate": gate,
                    "binding_profile": "beacon",
                    "executor_name": "hermes",
                    "truth_gate_name": "fake",
                }

            registry = ProjectRegistry.load(
                write_registry(root / "projects.json", [make_project(root)]),
                validate_paths=False,
            )
            repo = Path(registry.list()[0].repo)
            subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
            service = DeliveryService(
                registry,
                Storage(":memory:"),
                executor=fake,
                truth_gate=gate,
                preflight=PassingPreflight(),
                adapter_resolver=resolver,
            )
            first = service.dispatch(
                project_slug="demo", stage="goal", feature="f",
                channel="feishu", channel_thread="t1", target_executor="codex",
            )
            second = service.dispatch(
                project_slug="demo", stage="goal", feature="f",
                channel="feishu", channel_thread="t2", target_executor="claude",
            )
            self.assertEqual(first["status"], "dispatched")
            self.assertTrue(second["duplicate"])
            self.assertEqual(
                first["dispatch"]["dispatch_id"],
                second["dispatch"]["dispatch_id"],
            )
            self.assertEqual(fake.create_count, 1)


class ChannelDeliveryTests(unittest.TestCase):
    def test_hermes_channel_adapter_sends_to_thread(self):
        runner = RecordingRunner()
        adapter = HermesChannelAdapter(runner)
        result = adapter.deliver("done", channel_thread="oc_1:om_2", channel="feishu")
        self.assertTrue(result["delivered"])
        self.assertEqual(runner.calls[0][:4], ("hermes", "send", "--to", "feishu:oc_1:om_2"))

    def test_deliver_prefers_channel_adapter_over_executor(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = ProjectRegistry.load(
                write_registry(root / "projects.json", [make_project(root)]),
                validate_paths=False,
            )
            channel_runner = RecordingRunner()
            service = DeliveryService(
                registry,
                Storage(":memory:"),
                executor=FakeHermes(),
                truth_gate=FakeTruthGate(closure_pass=True),
                channel_adapter=HermesChannelAdapter(channel_runner),
            )
            dispatch = {
                "project_slug": "demo",
                "request": {"channel": "feishu", "channel_thread": "oc_1"},
            }
            result = service._deliver(dispatch, "completed")
            self.assertTrue(result["delivered"])
            self.assertIn("feishu:oc_1", channel_runner.calls[0])

    def test_pi_executor_has_no_deliver_contract(self):
        self.assertFalse(hasattr(PiExecutorAdapter(), "deliver"))


class AsyncPiReceiptTests(unittest.TestCase):
    def test_create_task_returns_running_then_done(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            runner = FakeRunner(FakeResult(stdout='{"sessionId":"s-1","type":"message_end","message":{"stopReason":"stop"}}'))
            ledger = PiRunLedger(root / "ledger")
            adapter = PiExecutorAdapter(
                runner=runner,
                which_command=lambda _n: "/usr/local/bin/pi",
                ledger=ledger,
            )
            receipt = adapter.create_task(project, stage="goal", feature="f", body="b", idempotency_key="k")
            self.assertEqual(receipt["status"], "running")
            self.assertEqual(receipt["session_ref"], "")
            ledger_receipt = ledger.get("adb-pi-demo", "k")
            self.assertEqual(ledger_receipt["status"], "running")
            deadline = time.time() + 3
            while time.time() < deadline:
                latest = ledger.get("adb-pi-demo", "k")
                if latest and latest.get("status") in {"done", "failed"}:
                    break
                time.sleep(0.02)
            self.assertEqual(latest["status"], "done")
            self.assertEqual(latest["session_ref"], "s-1")

    def test_timeout_records_failed_receipt(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)

            class TimeoutRunner:
                def run(self, command, *, cwd=None, timeout=30):
                    raise CommandTimedOut(
                        "external_command_timeout",
                        "boom",
                        resume_action="retry",
                    )

            ledger = PiRunLedger(root / "ledger")
            adapter = PiExecutorAdapter(
                runner=TimeoutRunner(),
                which_command=lambda _n: "/usr/local/bin/pi",
                ledger=ledger,
            )
            receipt = adapter.create_task(
                project, stage="goal", feature="f", body="b",
                idempotency_key="k", wait=True,
            )
            self.assertEqual(receipt["status"], "failed")
            self.assertEqual(receipt["reason_code"], "pi_timeout")
            self.assertIn("timeout", receipt["error"])

    def test_runner_failure_records_failed_receipt(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)

            class ExplodingRunner:
                def run(self, command, *, cwd=None, timeout=30):
                    raise RuntimeError("pi crashed")

            ledger = PiRunLedger(root / "ledger")
            adapter = PiExecutorAdapter(
                runner=ExplodingRunner(),
                which_command=lambda _n: "/usr/local/bin/pi",
                ledger=ledger,
            )
            receipt = adapter.create_task(
                project, stage="goal", feature="f", body="b",
                idempotency_key="k", wait=True,
            )
            self.assertEqual(receipt["status"], "failed")
            self.assertEqual(receipt["reason_code"], "pi_runner_failed")
            self.assertIn("crashed", receipt["error"])


if __name__ == "__main__":
    unittest.main()
