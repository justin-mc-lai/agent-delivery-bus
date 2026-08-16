from __future__ import annotations

import json
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

from agent_delivery_bus.adapters.hermes import HermesAdapter
from agent_delivery_bus.adapters.pi import PiExecutorAdapter, PiRunLedger
from agent_delivery_bus.approvals import ApprovalService
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.intent import IntentParser
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService, normalized_request, request_digest
from agent_delivery_bus.session import SessionRegistry, session_id_for
from agent_delivery_bus.storage import Storage

from .helpers import FakeHermes, make_project, write_registry


class SessionRoutingTests(unittest.TestCase):
    def _registry(self, tmp: Path, *, project=None) -> ProjectRegistry:
        return ProjectRegistry.load(
            write_registry(tmp / "projects.json", [project or make_project(tmp)]),
            validate_paths=False,
        )

    def test_session_registry_bind_resolve_list_status(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(":memory:")
            reg = SessionRegistry(storage)
            binding = reg.bind(
                channel="feishu",
                channel_thread="oc_1:om_2",
                actor_id="open_1",
                host_session="h1",
                target_executor="pi",
                target_session="sess_target",
            )
            self.assertTrue(binding["session_id"].startswith("sess_"))
            self.assertEqual(binding["state"], "bound")
            resolved = reg.resolve(binding["session_id"])
            self.assertEqual(resolved["target_executor"], "pi")
            self.assertEqual(len(reg.list(channel="feishu")), 1)
            sid = session_id_for(channel="feishu", channel_thread="oc_1:om_2", actor_id="open_1", host_session="h1")
            self.assertEqual(sid, binding["session_id"])

    def test_cli_session_bind(self):
        from agent_delivery_bus.cli import main

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config = write_registry(root / "projects.json", [make_project(root, slug="demo")])
            code = main(
                [
                    "--config",
                    str(config),
                    "--db",
                    ":memory:",
                    "session",
                    "bind",
                    "--channel",
                    "feishu",
                    "--thread",
                    "oc_1",
                    "--actor",
                    "open_1",
                    "--host-session",
                    "h1",
                    "--target",
                    "pi",
                    "--json",
                ]
            )
            self.assertEqual(code, 0)

    def test_session_stale(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(":memory:")
            reg = SessionRegistry(storage, ttl_seconds=60)
            binding = reg.bind(channel="feishu", channel_thread="t1", actor_id="a", host_session="h")
            old = (datetime.now(timezone.utc) - timedelta(hours=2)).isoformat()
            storage.conn.execute("UPDATE agent_sessions SET last_seen_at=? WHERE session_id=?", (old, binding["session_id"]))
            status = reg.status(binding["session_id"])
            self.assertTrue(status["stale"])
            with self.assertRaises(DeliveryBusError) as ctx:
                reg.resolve(binding["session_id"])
            self.assertEqual(ctx.exception.reason_code, "session_stale")

    def test_session_bind_rejects_unknown_target(self):
        storage = Storage(":memory:")
        reg = SessionRegistry(storage)
        with self.assertRaises(DeliveryBusError) as ctx:
            reg.bind(channel="feishu", channel_thread="t", target_executor="skynet")
        self.assertEqual(ctx.exception.reason_code, "session_target_unknown")

    def test_envelope_v11_and_idempotency_axes(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            base = normalized_request(project, stage="goal", feature="f", channel="feishu", channel_thread="t1", actor_id="a", target_executor="pi")
            self.assertEqual(base["schema_version"], "1.1")
            same = normalized_request(project, stage="goal", feature="f", channel="feishu", channel_thread="t1", actor_id="a", target_executor="pi")
            other_thread = normalized_request(project, stage="goal", feature="f", channel="feishu", channel_thread="t2", actor_id="a", target_executor="pi")
            other_agent = normalized_request(project, stage="goal", feature="f", channel="feishu", channel_thread="t1", actor_id="a", target_executor="codex")
            other_stage = normalized_request(project, stage="plan", feature="f", channel="feishu", channel_thread="t1", actor_id="a", target_executor="pi")
            other_feature = normalized_request(project, stage="goal", feature="g", channel="feishu", channel_thread="t1", actor_id="a", target_executor="pi")
            self.assertEqual(request_digest(base), request_digest(same))
            # Routing/session context must NOT change the business idempotency key.
            self.assertEqual(request_digest(base), request_digest(other_thread))
            self.assertEqual(request_digest(base), request_digest(other_agent))
            # Business essence still separates dispatches.
            self.assertNotEqual(request_digest(base), request_digest(other_stage))
            self.assertNotEqual(request_digest(base), request_digest(other_feature))

    def test_intent_parse_agent(self):
        with tempfile.TemporaryDirectory() as tmp:
            registry = self._registry(Path(tmp))
            parsed = IntentParser(registry).parse("demo implement feat-x", project="demo", agent="pi")
            self.assertEqual(parsed["data"]["envelope"]["target_executor"], "pi")

    def test_session_context_injection_and_pi_session_id(self):
        from agent_delivery_bus.service import task_body

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            request = normalized_request(
                project, stage="goal", feature="f", channel="feishu",
                channel_thread="oc_1", actor_id="a", host_session_ref="h",
                target_executor="pi", target_session_ref="sess_t",
            )
            body = DeliveryService._task_body_with_session(
                project, stage="goal", feature="f", memory_summary="", dispatch_id="d1",
                binding_profile="beacon", profile_config=None, request=request,
            )
            self.assertIn("### Session context", body)
            self.assertIn("channel_thread: oc_1", body)
            self.assertIn("target_session_ref: sess_t", body)
            # pi adapter passes --session-id
            from .test_pi_executor import FakeRunner, FakeResult

            runner = FakeRunner(FakeResult(stdout='{"type":"message_end","message":{"stopReason":"stop"}}'))
            pi = PiExecutorAdapter(
                runner=runner,
                which_command=lambda _name: "/usr/local/bin/pi",
                ledger=PiRunLedger(root / "ledger"),
            )
            pi.create_task(project, stage="goal", feature="f", body="b", idempotency_key="k", session_id="sess_t", wait=True)
            cmd = next(c for c in runner.calls if "-p" in c)
            self.assertIn("--session-id", cmd)
            self.assertIn("sess_t", cmd)

    def test_hermes_deliver_and_reconcile_delivery(self):
        from .test_pi_executor import FakeResult

        with tempfile.TemporaryDirectory() as tmp:
            runner = FakeHermes()
            hermes = HermesAdapter(runner=runner, which_command=lambda _n: "/usr/local/bin/hermes")
            # FakeHermes.run returns a stub; patch its run to record
            calls: list[list[str]] = []

            class RecordingRunner:
                def run(self, argv, **kw):
                    calls.append(list(argv))
                    return FakeResult()

            h2 = HermesAdapter(runner=RecordingRunner(), which_command=lambda _n: "/usr/local/bin/hermes")
            result = h2.deliver("completed", channel_thread="oc_1:om_2", channel="feishu")
            self.assertTrue(result["delivered"])
            self.assertIn("hermes", calls[0])
            self.assertIn("send", calls[0])
            self.assertIn("feishu:oc_1:om_2", calls[0])

    def test_approval_channel_actor(self):
        storage = Storage(":memory:")
        svc = ApprovalService(storage)
        issued = svc.issue(actor="you", project_slug="demo", stage="implement", feature="f", ttl_seconds=300, channel_actor="open_1")
        svc.reserve(issued["token"], dispatch_id="d1", project_slug="demo", stage="implement", feature="f", channel_actor="open_1")
        second = svc.issue(actor="you", project_slug="demo", stage="implement", feature="f", ttl_seconds=300, channel_actor="open_1")
        with self.assertRaises(DeliveryBusError) as ctx:
            svc.reserve(second["token"], dispatch_id="d2", project_slug="demo", stage="implement", feature="f", channel_actor="open_2")
        self.assertEqual(ctx.exception.reason_code, "approval_channel_actor_mismatch")

    def test_dispatch_requires_channel_thread_when_channel_given(self):
        with tempfile.TemporaryDirectory() as tmp:
            registry = self._registry(Path(tmp))
            svc = DeliveryService(
                registry,
                Storage(":memory:"),
                executor=FakeHermes(),
                truth_gate=type("G", (), {"preflight_checks": lambda self, p, stage="": [], "closure": lambda self, **k: {"pass": True}})(),
                adapter_resolver=lambda p: {"executor": FakeHermes(), "truth_gate": None, "binding_profile": "beacon", "executor_name": "hermes", "truth_gate_name": "null"},
            )
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.dispatch(project_slug="demo", stage="goal", feature="f", channel="feishu")
            self.assertEqual(ctx.exception.reason_code, "session_identity_incomplete")


if __name__ == "__main__":
    unittest.main()
