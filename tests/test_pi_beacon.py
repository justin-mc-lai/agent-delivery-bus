from __future__ import annotations

import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.pi import PiExecutorAdapter, PiRunLedger

from .helpers import make_project
from .test_pi_executor import FakeRunner, FakeResult


class PiBeaconTests(unittest.TestCase):
    def test_extension_registers_bridge_tools(self):
        ext = Path(__file__).resolve().parents[1] / "skills" / "pi-beacon" / "extension.ts"
        self.assertTrue(ext.is_file())
        text = ext.read_text(encoding="utf-8")
        self.assertIn('registerTool({', text)
        self.assertIn('name: "adb_dispatch"', text)
        self.assertIn('registerCommand("prism"', text)
        self.assertIn('pi.on("session_start"', text)

    def test_installer_dry_run_idempotent(self):
        script = Path(__file__).resolve().parents[1] / "skills" / "pi-beacon" / "install.sh"
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp) / "home"
            home.mkdir()
            env = dict(os.environ)
            env["HOME"] = str(home)
            env["PI_CODING_AGENT_DIR"] = str(home / ".pi" / "agent")
            env["PRISM_SKILLS_DIR"] = str(home / "prism" / "skills")
            first = subprocess.run(["bash", str(script), "--dry-run"], capture_output=True, text=True, env=env)
            self.assertEqual(first.returncode, 0, first.stderr)
            second = subprocess.run(["bash", str(script), "--dry-run"], capture_output=True, text=True, env=env)
            self.assertEqual(second.returncode, 0, second.stderr)
            self.assertIn("dry-run", first.stdout)

    def test_driver_pi_bounded_body(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            runner = FakeRunner(FakeResult(stdout='{"type":"message_end","message":{"stopReason":"stop"}}'))
            adapter = PiExecutorAdapter(
                runner=runner,
                which_command=lambda _name: "/usr/local/bin/pi",
                ledger=PiRunLedger(root / "ledger"),
            )
            adapter.create_task(project, stage="goal", feature="f", body="### Evidence spec\n- dispatch_id_binding: true", idempotency_key="k")
            body_cmd = next(cmd for cmd in runner.calls if "-p" in cmd)
            self.assertIn("### Bounded task", body_cmd[-1])
            self.assertIn("Execute ONLY the single concrete deliverable", body_cmd[-1])


if __name__ == "__main__":
    unittest.main()
