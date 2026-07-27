from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.preflight import Preflight
from agent_delivery_bus.process import CommandResult

from .helpers import RecordingRunner, make_project


class FakeBeacon:
    def __init__(self, passed=True, docs_version="v1.0.0"):
        self.passed = passed
        self.docs_version = docs_version

    def verify_context(self, project):
        return {
            "pass": self.passed,
            "payload": {
                "status": "PASS" if self.passed else "BLOCKED",
                "docs_version": self.docs_version,
            },
        }


class FakeHermes:
    def __init__(self, gateway=True, profile=True):
        self.gateway = gateway
        self.profile = profile

    def health(self, profile="coding"):
        return {"gateway_pass": self.gateway, "profile_pass": self.profile, "profiles": ["coding"]}


class PreflightTests(unittest.TestCase):
    def test_strict_preflight_passes_without_write_commands(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            runner = RecordingRunner(
                [CommandResult(("git",), 0, "true\n", "")]
            )
            result = Preflight(
                FakeBeacon(),
                FakeHermes(),
                runner,
                which_command=lambda name: f"/usr/bin/{name}",
            ).run(project, stage="plan")
            self.assertFalse(result["blocked"])
            joined = " ".join(" ".join(call) for call in runner.calls)
            self.assertNotIn("setup-context", joined)
            self.assertNotIn("sync-materials", joined)

    def test_context_failure_has_reason_and_resume_route(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            runner = RecordingRunner(
                [CommandResult(("git",), 0, "true\n", "")]
            )
            result = Preflight(
                FakeBeacon(False),
                FakeHermes(),
                runner,
                which_command=lambda name: f"/usr/bin/{name}",
            ).run(project, stage="implement")
            self.assertTrue(result["blocked"])
            self.assertEqual(result["reason_code"], "beacon_context_invalid")
            self.assertIn("setup-context", result["resume_action"])

    def test_registered_docs_version_drift_blocks(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            runner = RecordingRunner(
                [CommandResult(("git",), 0, "true\n", "")]
            )
            result = Preflight(
                FakeBeacon(True, docs_version="v1.1.0"),
                FakeHermes(),
                runner,
                which_command=lambda name: f"/usr/bin/{name}",
            ).run(project, stage="plan")
            self.assertTrue(result["blocked"])
            self.assertEqual(result["reason_code"], "beacon_version_mismatch")

    def test_missing_cli_returns_stable_reason_code(self):
        with tempfile.TemporaryDirectory() as tmp:
            project = make_project(Path(tmp))
            runner = RecordingRunner(
                [CommandResult(("git",), 0, "true\n", "")]
            )
            result = Preflight(
                FakeBeacon(),
                FakeHermes(),
                runner,
                which_command=lambda name: None,
            ).run(project, stage="plan")
            self.assertTrue(result["blocked"])
            self.assertEqual(result["reason_code"], "beacon_cli_unavailable")


if __name__ == "__main__":
    unittest.main()
