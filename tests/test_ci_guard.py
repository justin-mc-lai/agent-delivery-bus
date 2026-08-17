"""AC-CPH-007: CI runs tests + version check only; no release steps."""

from __future__ import annotations

import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class CiGuardTests(unittest.TestCase):
    def test_ci_contains_no_release_steps(self):
        text = (ROOT / ".github" / "workflows" / "ci.yml").read_text(encoding="utf-8").lower()
        for forbidden in ("release", "publish", "deploy"):
            self.assertNotIn(forbidden, text, f"CI must not contain {forbidden!r}")

    def test_ci_runs_tests_and_version_alignment(self):
        text = (ROOT / ".github" / "workflows" / "ci.yml").read_text(encoding="utf-8")
        self.assertIn("pytest", text)
        self.assertIn("verify-version-alignment", text)


if __name__ == "__main__":
    unittest.main()
