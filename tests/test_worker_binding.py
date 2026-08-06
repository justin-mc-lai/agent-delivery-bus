"""Contract tests for worker-beacon-binding (Wave 2)."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from typing import Any

from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService, task_body
from agent_delivery_bus.storage import Storage
from agent_delivery_bus.worker_binding import (
    DEFAULT_RUNNER_PROFILE,
    ENABLED_STAGES,
    resolve_worker_binding,
)

from .helpers import FakeBeacon, FakeHermes, PassingPreflight, make_project, write_registry


class BlockedPreflight:
    def __init__(self, reason_code: str = "workspace_admission_failed"):
        self.reason_code = reason_code

    def run(self, project, *, stage: str) -> dict[str, Any]:
        return {
            "status": "blocked",
            "blocked": True,
            "reason_code": self.reason_code,
            "resume_action": "repair workspace admission, then retry dispatch",
            "project": project.slug,
            "stage": stage,
            "checks": [],
        }


class WorkerBindingContractTests(unittest.TestCase):
    def test_evidence_spec_in_task_body(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            body = task_body(
                project,
                stage="plan",
                feature="feature",
                dispatch_id="adb_evidence_spec_check",
            )
            self.assertIn("### Evidence spec", body)
            self.assertIn("evidence_dir:", body)
            self.assertIn("glob: *.json", body)
            self.assertIn("required_files: manifest.json", body)
            self.assertIn("dispatch_id_binding: true", body)
            self.assertIn("dispatch_id: adb_evidence_spec_check", body)

    def test_schema_version_1_1(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            binding = resolve_worker_binding(
                stage="plan",
                feature="feature",
                docs_version=project.docs_version or "",
            )
            body = task_body(project, stage="plan", feature="feature")
            self.assertEqual(binding["schema_version"], "1.1")
            self.assertIn("schema_version: 1.1", body)
            self.assertIn("binding_profile: beacon", body)

    def test_missing_evidence_spec_rejected(self):
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

    def test_custom_profile_binding_without_beacon_fields(self):
        config = {
            "stages": {
                "plan": {"skill": "custom-plan", "command": "run-plan {feature}", "public_harness": "plan"}
            },
            "runner": {"runner_kind": "local_agent", "hermes_assignee": "coding"},
            "evidence_spec": {
                "evidence_dir": ".adb/evidence/{stage}/{feature}",
                "glob": "*.json",
                "dispatch_id_binding": True,
            },
        }
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            binding = resolve_worker_binding(
                stage="plan",
                feature="feature",
                binding_profile="generic",
                profile_config=config,
                project_repo=project.repo,
                dispatch_id="adb_custom",
            )
            self.assertEqual(binding["binding_profile"], "generic")
            self.assertNotIn("beacon_skill", binding)
            self.assertNotIn("beacon_command", binding)
            self.assertEqual(binding["skill"], "custom-plan")
            self.assertEqual(binding["command"], "run-plan feature")
            self.assertEqual(binding["evidence_spec"]["dispatch_id"], "adb_custom")
            body = task_body(
                project,
                stage="plan",
                feature="feature",
                dispatch_id="adb_custom",
                binding_profile="generic",
                profile_config=config,
            )
            self.assertIn("### Worker binding", body)
            self.assertNotIn("### Beacon worker binding", body)
            self.assertNotIn("beacon_skill", body)
            self.assertNotIn("beacon_command", body)
            self.assertIn("skill: custom-plan", body)
            self.assertIn("command: run-plan feature", body)
            self.assertIn(".adb/evidence/plan/feature", body)

    def test_task_body_plan_contains_beacon_skill_binding(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            body = task_body(project, stage="plan", feature="worker-beacon-binding")
            binding = resolve_worker_binding(
                stage="plan",
                feature="worker-beacon-binding",
                docs_version=project.docs_version or "",
            )
            self.assertIn("### Beacon worker binding", body)
            self.assertIn("beacon_skill: beacon-plan", body)
            self.assertIn("beacon_command:", body)
            self.assertIn(binding["beacon_command"], body)
            self.assertEqual(binding["beacon_skill"], "beacon-plan")
            self.assertEqual(binding["public_harness"], "plan")

    def test_runner_profile_is_explicit_local_coding(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            body = task_body(project, stage="plan", feature="feature")
            binding = resolve_worker_binding(stage="plan", feature="feature", docs_version="v1.0.0")
            self.assertEqual(binding["runner_profile"], "coding")
            self.assertEqual(binding["runner"]["runner_kind"], "local_agent")
            self.assertIn("coding", binding["runner"]["allowed_profiles"])
            self.assertIn("codex", binding["runner"]["allowed_profiles"])
            self.assertTrue(binding["runner"]["cloud_scheduler_forbidden"])
            self.assertIn("runner_profile: coding", body)
            self.assertIn("runner_kind: local_agent", body)
            self.assertIn("cloud_scheduler_forbidden: true", body)
            self.assertEqual(DEFAULT_RUNNER_PROFILE["hermes_assignee"], "coding")

    def test_admission_fail_closed_blocks_dispatch(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            hermes = FakeHermes()
            service = DeliveryService(
                registry,
                storage,
                preflight=BlockedPreflight("workspace_admission_failed"),
                executor=hermes,
                truth_gate=FakeBeacon(),
            )
            result = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(result["status"], "blocked")
            self.assertTrue(result["blocked"])
            self.assertEqual(result["reason_code"], "workspace_admission_failed")
            self.assertEqual(hermes.create_count, 0)
            storage.close()

    def test_approve_still_required_for_implement(self):
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
            waiting = service.dispatch(project_slug="demo", stage="implement", feature="feature")
            self.assertEqual(waiting["status"], "awaiting_approval")
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
            self.assertIn("beacon_skill: beacon-implement", hermes.last_body)
            storage.close()

    def test_goal_deferred_from_enabled_stages(self):
        self.assertNotIn("goal", ENABLED_STAGES)
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
            with self.assertRaises(DeliveryBusError) as raised:
                service.dispatch(project_slug="demo", stage="goal", feature="feature")
            self.assertEqual(raised.exception.reason_code, "goal_stage_deferred")
            self.assertEqual(hermes.create_count, 0)
            storage.close()

    def test_illegal_skip_approve_fail_closed(self):
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
            for stage in ("implement", "freeze"):
                result = service.dispatch(project_slug="demo", stage=stage, feature="feature")
                self.assertEqual(result["status"], "awaiting_approval", stage)
                self.assertTrue(result["blocked"], stage)
                self.assertEqual(result["reason_code"], "approval_required", stage)
            self.assertEqual(hermes.create_count, 0)
            storage.close()

    def test_illegal_goal_enable_without_promote(self):
        with self.assertRaises(DeliveryBusError) as raised:
            resolve_worker_binding(stage="goal", feature="feature", docs_version="v0.0.3")
        self.assertEqual(raised.exception.reason_code, "goal_stage_deferred")
        self.assertNotIn("goal", ENABLED_STAGES)
