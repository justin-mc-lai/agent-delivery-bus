"""Per-project adapter routing tests (AC-NS-003 / AC-NS-008)."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.factory import AdapterResolver
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.worker_binding import resolve_worker_binding

from .helpers import make_project


def registry_payload(project_rows: list[dict]) -> dict:
    return {
        "schema_version": "1.0",
        "adapters": {"executor": "null", "truth_gate": "null", "binding_profile": "beacon"},
        "projects": project_rows,
    }


class RegistryRoutingTests(unittest.TestCase):
    def test_per_project_adapters_parsed_and_serialized(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            row = project.to_dict()
            row["truth_gate"] = "beacon"
            row["executor"] = "hermes"
            row["binding_profile"] = "generic"
            path = root / "projects.json"
            path.write_text(json.dumps(registry_payload([row])), encoding="utf-8")
            registry = ProjectRegistry.load(path)
            loaded = registry.resolve(slug="demo")
            self.assertEqual(loaded.truth_gate, "beacon")
            self.assertEqual(loaded.executor, "hermes")
            self.assertEqual(loaded.binding_profile, "generic")
            self.assertEqual(loaded.to_dict()["truth_gate"], "beacon")
            self.assertEqual(loaded.to_dict()["executor"], "hermes")
            self.assertEqual(loaded.to_dict()["binding_profile"], "generic")

    def test_global_fallback_and_per_project_override(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            path = root / "projects.json"
            path.write_text(json.dumps(registry_payload([project.to_dict()])), encoding="utf-8")
            registry = ProjectRegistry.load(path)
            resolver = AdapterResolver(registry.raw)
            routed = resolver.for_project(project)
            self.assertEqual(routed["executor_name"], "null")
            self.assertEqual(routed["truth_gate_name"], "null")
            self.assertEqual(routed["binding_profile"], "beacon")

            override = make_project(root, slug="override")
            routed_override = resolver.for_project(override)
            self.assertEqual(routed_override["executor_name"], "null")

            project_with_overrides = ProjectRegistry.load(path).resolve(slug="demo")
            self.assertEqual(project_with_overrides.executor, "")

    def test_unknown_adapter_fail_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            row = project.to_dict()
            row["executor"] = "bogus"
            path = root / "projects.json"
            path.write_text(json.dumps(registry_payload([row])), encoding="utf-8")
            registry = ProjectRegistry.load(path)
            resolver = AdapterResolver(registry.raw)
            with self.assertRaises(DeliveryBusError) as ctx:
                resolver.for_project(registry.resolve(slug="demo"))
            self.assertEqual(ctx.exception.reason_code, "executor_adapter_unknown")

            row["executor"] = ""
            row["truth_gate"] = "bogus"
            path.write_text(json.dumps(registry_payload([row])), encoding="utf-8")
            registry = ProjectRegistry.load(path)
            resolver = AdapterResolver(registry.raw)
            with self.assertRaises(DeliveryBusError) as ctx:
                resolver.for_project(registry.resolve(slug="demo"))
            self.assertEqual(ctx.exception.reason_code, "truth_gate_adapter_unknown")

    def test_unknown_binding_profile_fail_closed(self):
        with self.assertRaises(DeliveryBusError) as ctx:
            resolve_worker_binding(
                stage="plan",
                feature="feature",
                binding_profile="not-a-profile",
                profile_config=None,
            )
        self.assertEqual(ctx.exception.reason_code, "binding_profile_unknown")

    def test_custom_profile_requires_evidence_spec(self):
        config = {
            "stages": {
                "plan": {"skill": "custom-plan", "command": "run-plan {feature}", "public_harness": "plan"}
            }
        }
        with self.assertRaises(DeliveryBusError) as ctx:
            resolve_worker_binding(
                stage="plan",
                feature="feature",
                binding_profile="generic",
                profile_config=config,
            )
        self.assertEqual(ctx.exception.reason_code, "binding_profile_evidence_spec_required")


if __name__ == "__main__":
    unittest.main()
