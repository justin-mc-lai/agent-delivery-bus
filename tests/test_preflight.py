from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.preflight import Preflight
from agent_delivery_bus.process import CommandResult

from .helpers import RecordingRunner, make_project


class FakeTruthGate:
    def __init__(self, passed=True, docs_version="v1.0.0", reason="beacon_context_invalid"):
        self.passed = passed
        self.docs_version = docs_version
        self.reason = reason

    def preflight_checks(self, project, *, stage):
        if self.docs_version != project.docs_version:
            return [{
                "name": "truth_declared_version",
                "passed": False,
                "reason_code": "truth_version_mismatch",
                "resume_action": "update registry docs_version",
                "detail": {"registered": project.docs_version, "project_reported": self.docs_version},
            }]
        if not self.passed:
            return [{
                "name": "beacon_context_strict",
                "passed": False,
                "reason_code": self.reason,
                "resume_action": f"run `beacon doctor setup-context --project-root {project.repo}`",
                "detail": {},
            }]
        return [{"name": "truth_ok", "passed": True, "reason_code": "", "resume_action": "", "detail": {}}]

    def closure(self, project, *, stage, feature):
        return {"pass": True, "evidence": []}


class FakeExecutor:
    def __init__(self, gateway=True, profile=True, cli_missing=False):
        self.gateway = gateway
        self.profile = profile
        self.cli_missing = cli_missing

    def preflight_checks(self, project, *, stage):
        if self.cli_missing:
            return [{
                "name": "hermes_cli",
                "passed": False,
                "reason_code": "hermes_cli_unavailable",
                "resume_action": "install hermes",
                "detail": {},
            }]
        checks = []
        if not self.gateway:
            checks.append({
                "name": "hermes_gateway",
                "passed": False,
                "reason_code": "hermes_gateway_unavailable",
                "resume_action": "start gateway",
                "detail": {},
            })
        if not self.profile:
            checks.append({
                "name": "hermes_profile",
                "passed": False,
                "reason_code": "hermes_profile_missing",
                "resume_action": "create profile",
                "detail": {},
            })
        if not checks:
            checks.append({"name": "executor_ok", "passed": True, "reason_code": "", "resume_action": "", "detail": {}})
        return checks

    def board_for(self, project):
        return f"adb-{project.slug}"

    def workspace_for(self, project, *, stage):
        return f"dir:{project.repo}"

    def ensure_board(self, project):
        return {"slug": self.board_for(project)}

    def create_task(self, *args, **kwargs):
        raise NotImplementedError

    def show_task(self, board, task_id):
        raise NotImplementedError

    def find_by_idempotency(self, board, key):
        return None


class PreflightTests(unittest.TestCase):
    def test_strict_preflight_passes_without_write_commands(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            runner = RecordingRunner([CommandResult(("git",), 0, "true\n", "")])
            result = Preflight(FakeTruthGate(), FakeExecutor(), runner).run(project, stage="plan")
            self.assertFalse(result["blocked"])
            joined = " ".join(" ".join(call) for call in runner.calls)
            self.assertNotIn("setup-context", joined)
            self.assertNotIn("sync-materials", joined)

    def test_context_failure_has_reason_and_resume_route(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            runner = RecordingRunner([CommandResult(("git",), 0, "true\n", "")])
            result = Preflight(FakeTruthGate(False), FakeExecutor(), runner).run(project, stage="implement")
            self.assertTrue(result["blocked"])
            self.assertEqual(result["reason_code"], "beacon_context_invalid")
            self.assertIn("setup-context", result["resume_action"])

    def test_registered_docs_version_drift_blocks(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            runner = RecordingRunner([CommandResult(("git",), 0, "true\n", "")])
            result = Preflight(FakeTruthGate(True, docs_version="v1.1.0"), FakeExecutor(), runner).run(project, stage="plan")
            self.assertTrue(result["blocked"])
            self.assertEqual(result["reason_code"], "truth_version_mismatch")

    def test_missing_executor_cli_returns_stable_reason_code(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            runner = RecordingRunner([CommandResult(("git",), 0, "true\n", "")])
            result = Preflight(FakeTruthGate(), FakeExecutor(cli_missing=True), runner).run(project, stage="plan")
            self.assertTrue(result["blocked"])
            self.assertEqual(result["reason_code"], "hermes_cli_unavailable")


if __name__ == "__main__":
    unittest.main()
