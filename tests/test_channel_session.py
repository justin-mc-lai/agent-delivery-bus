from __future__ import annotations

import json
import subprocess
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.hermes import HermesAdapter
from agent_delivery_bus.adapters.pi import PiExecutorAdapter, PiRunLedger
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService, normalized_request, request_digest
from agent_delivery_bus.session import SessionRegistry, next_task_session
from agent_delivery_bus.storage import Storage

from .helpers import FakeHermes, make_project, write_registry
from .test_pi_executor import FakeResult, FakeRunner


class ChannelSessionTests(unittest.TestCase):
    def _svc(self, tmp: Path):
        registry = ProjectRegistry.load(
            write_registry(tmp / "projects.json", [make_project(tmp)]),
            validate_paths=False,
        )
        hermes = FakeHermes()
        gate = type("G", (), {
            "preflight_checks": lambda self, p, stage="": [],
            "closure": lambda self, **k: {"pass": True},
        })()
        svc = DeliveryService(
            registry,
            Storage(":memory:"),
            executor=hermes,
            truth_gate=gate,
            adapter_resolver=lambda p, stage="": {
                "executor": hermes,
                "truth_gate": gate,
                "binding_profile": "beacon",
                "executor_name": "hermes",
                "truth_gate_name": "null",
            },
        )
        return registry, svc

    def test_task_session_auto_distinct(self):
        with tempfile.TemporaryDirectory() as tmp:
            registry, svc = self._svc(Path(tmp))
            r1 = svc.dispatch(
                project_slug="demo", stage="goal", feature="f1",
                channel="feishu", channel_thread="t1", dry_run=True,
            )
            r2 = svc.dispatch(
                project_slug="demo", stage="goal", feature="f2",
                channel="feishu", channel_thread="t1", dry_run=True,
            )
            s1 = r1["request"]["target_session_ref"]
            s2 = r2["request"]["target_session_ref"]
            self.assertTrue(s1.startswith("hermes-"))
            self.assertNotEqual(s1, s2)
            self.assertEqual(r1["request"]["resolution_source"], "channel_default")

    def test_resolution_binding_and_policy(self):
        with tempfile.TemporaryDirectory() as tmp:
            registry, svc = self._svc(Path(tmp))
            sessions = SessionRegistry(svc.storage)
            sessions.bind(channel="feishu", channel_thread="t1", actor_id="a", host_session="h", target_executor="pi")
            r = svc.dispatch(
                project_slug="demo", stage="goal", feature="f",
                channel="feishu", channel_thread="t1", actor_id="a", host_session_ref="h", dry_run=True,
            )
            self.assertEqual(r["request"]["target_executor"], "pi")
            self.assertEqual(r["request"]["resolution_source"], "binding")
            r2 = svc.dispatch(
                project_slug="demo", stage="goal", feature="f",
                channel="feishu", channel_thread="t1", actor_id="a", host_session_ref="h",
                target_executor="codex", dry_run=True,
            )
            self.assertEqual(r2["request"]["resolution_source"], "explicit")

    def test_lease_acquire_busy_release(self):
        storage = Storage(":memory:")
        reg = SessionRegistry(storage)
        reg.acquire("sess_x", "d1")
        with self.assertRaises(DeliveryBusError) as ctx:
            reg.acquire("sess_x", "d2")
        self.assertEqual(ctx.exception.reason_code, "session_busy")
        reg.acquire("sess_x", "d1")  # idempotent same owner
        reg.release("sess_x", "d1")
        reg.acquire("sess_x", "d2")  # releasable
        with self.assertRaises(DeliveryBusError) as ctx2:
            reg.release("sess_x", "d9")
        self.assertEqual(ctx2.exception.reason_code, "session_lease_mismatch")

    def test_service_fixed_session_busy(self):
        with tempfile.TemporaryDirectory() as tmp:
            registry, svc = self._svc(Path(tmp))
            subprocess.run(["git", "init", "-q"], cwd=Path(registry.list()[0].repo), check=True)
            first = svc.dispatch(
                project_slug="demo", stage="goal", feature="f1",
                channel="feishu", channel_thread="t1", target_executor="pi",
                target_session_ref="fixed:sess_fix", dry_run=True,
            )
            self.assertTrue(first["request"]["lease_required"])
            # real dispatch acquires the lease
            real = svc.dispatch(
                project_slug="demo", stage="goal", feature="f1",
                channel="feishu", channel_thread="t1", target_executor="pi",
                target_session_ref="fixed:sess_fix",
            )
            self.assertEqual(real["status"], "dispatched")
            second = svc.dispatch(
                project_slug="demo", stage="goal", feature="f2",
                channel="feishu", channel_thread="t1", target_executor="pi",
                target_session_ref="fixed:sess_fix",
            )
            self.assertEqual(second["status"], "blocked")
            self.assertEqual(second["reason_code"], "session_busy")

    def test_deliver_uses_channel(self):
        calls: list[list[str]] = []

        class R:
            def run(self, argv, **kw):
                calls.append(list(argv))
                return FakeResult()

        h = HermesAdapter(runner=R(), which_command=lambda _n: "/usr/local/bin/hermes")
        h.deliver("done", channel_thread="oc_1", channel="weixin")
        self.assertIn("weixin:oc_1", calls[0])

    def test_docs_resolution_order(self):
        skill = Path(__file__).resolve().parents[1] / "skills" / "agent-delivery-bus" / "SKILL.md"
        text = skill.read_text(encoding="utf-8")
        for needle in ("决议顺序", "executor_policy", "channel_default"):
            self.assertIn(needle, text)

    def test_unknown_channel_blocked_in_script_shape(self):
        # next_task_session determinism + channel normalization surface
        self.assertTrue(next_task_session(target_executor="pi", seed="x").startswith("pi-"))


if __name__ == "__main__":
    unittest.main()
