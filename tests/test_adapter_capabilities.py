"""AC-CPH-006: explicit capabilities; no TypeError fallback chain."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.factory import AdapterResolver
from agent_delivery_bus.adapters.hermes import HermesAdapter
from agent_delivery_bus.adapters.null import NullExecutor
from agent_delivery_bus.adapters.pi import PiExecutorAdapter
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import FakeExecutor, FakeTruthGate, PassingPreflight, make_project, write_registry


class CapabilityGapExecutor(FakeExecutor):
    """Executor that declares no capabilities (legacy third-party shape)."""

    def __init__(self, *, remote_status: str = "running"):
        super().__init__(remote_status=remote_status)
        self.capabilities = {}


class AdapterCapabilitiesTests(unittest.TestCase):
    def test_all_builtin_adapters_declare_capabilities(self):
        for adapter in (NullExecutor(), HermesAdapter(), PiExecutorAdapter()):
            self.assertIn("task_skills", adapter.capabilities)
            self.assertIn("task_session", adapter.capabilities)
            self.assertIsInstance(adapter.capabilities["task_skills"], bool)
            self.assertIsInstance(adapter.capabilities["task_session"], bool)

    def _service(self, executor):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            return (
                registry,
                DeliveryService(
                    registry,
                    Storage(":memory:"),
                    preflight=PassingPreflight(),
                    executor=executor,
                    truth_gate=FakeTruthGate(),
                ),
                project,
            )

    def test_dispatch_passes_skills_only_when_declared(self):
        executor = FakeExecutor()
        registry, service, project = self._service(executor)
        result = service.dispatch(project_slug=project.slug, stage="plan", feature="feature")
        self.assertEqual(result["status"], "dispatched")
        self.assertTrue(executor.last_skills)

    def test_dispatch_without_capability_skips_skill_injection(self):
        executor = CapabilityGapExecutor()
        registry, service, project = self._service(executor)
        result = service.dispatch(project_slug=project.slug, stage="plan", feature="feature")
        self.assertEqual(result["status"], "dispatched")
        self.assertEqual(executor.last_skills, [])

    def test_resolver_capabilities_drive_full_signature_call(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            resolver = AdapterResolver({"adapters": {"executor": "null", "truth_gate": "null"}})
            service = DeliveryService(
                registry,
                Storage(":memory:"),
                preflight=PassingPreflight(),
                executor=resolver.global_adapters()["executor"],
                truth_gate=resolver.global_adapters()["truth_gate"],
                adapter_resolver=resolver.for_project,
            )
            adapters = service._adapters_for(project, stage="plan", target_executor="pi")
            self.assertEqual(adapters["executor_name"], "pi")

    def test_legacy_lambda_resolver_still_works(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            executor = NullExecutor()
            gate = FakeTruthGate()
            service = DeliveryService(
                registry,
                Storage(":memory:"),
                preflight=PassingPreflight(),
                executor=executor,
                truth_gate=gate,
                adapter_resolver=lambda p: {
                    "executor": executor,
                    "truth_gate": gate,
                    "binding_profile": "beacon",
                    "executor_name": "null",
                    "truth_gate_name": "null",
                },
            )
            adapters = service._adapters_for(project, stage="plan", target_executor="")
            self.assertEqual(adapters["executor_name"], "null")

    def test_stage_only_resolver_gets_stage(self):
        seen = {}

        def resolver(project, *, stage=""):
            seen["stage"] = stage
            return {
                "executor": NullExecutor(),
                "truth_gate": FakeTruthGate(),
                "binding_profile": "beacon",
                "executor_name": "null",
                "truth_gate_name": "null",
            }

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            service = DeliveryService(
                registry,
                Storage(":memory:"),
                preflight=PassingPreflight(),
                executor=NullExecutor(),
                truth_gate=FakeTruthGate(),
                adapter_resolver=resolver,
            )
            service._adapters_for(project, stage="qa", target_executor="")
            self.assertEqual(seen["stage"], "qa")

    def test_executor_target_mismatch_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            executor = NullExecutor()
            gate = FakeTruthGate()

            class FixedResolver:
                resolver_capabilities = {"target_executor": True, "stage": True}

                def __call__(self, project, *, stage="", target_executor=""):
                    return {
                        "executor": executor,
                        "truth_gate": gate,
                        "binding_profile": "beacon",
                        "executor_name": "hermes",
                        "truth_gate_name": "null",
                    }

            service = DeliveryService(
                registry,
                Storage(":memory:"),
                preflight=PassingPreflight(),
                executor=executor,
                truth_gate=gate,
                adapter_resolver=FixedResolver(),
            )
            with self.assertRaises(DeliveryBusError) as ctx:
                service._adapters_for(project, stage="plan", target_executor="pi")
            self.assertEqual(ctx.exception.reason_code, "executor_mismatch")


if __name__ == "__main__":
    unittest.main()
