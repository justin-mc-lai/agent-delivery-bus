"""AC-CPH-001/002: version-truth-catalog is the single source; projections align."""

from __future__ import annotations

import json
import subprocess
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.version_truth import (
    check_alignment,
    git_tag_exists,
    parse_catalog,
)


ROOT = Path(__file__).resolve().parents[1]


class VersionTruthTests(unittest.TestCase):
    def test_catalog_parse_and_alignment_pass(self):
        catalog = parse_catalog(ROOT / "docs/beacon/governance/version-truth-catalog.md")
        self.assertEqual(catalog["package_version"], "0.1.4")
        self.assertEqual(catalog["active_docs_line"], "v0.1.4")
        self.assertFalse(check_alignment(ROOT))

    def test_catalog_missing_keys_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "catalog.md"
            path.write_text("---\npackage_version: \"0.1.4\"\n---\n", encoding="utf-8")
            with self.assertRaises(ValueError):
                parse_catalog(path)

    def test_drift_detected_across_surfaces(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            docs = root / "docs" / "beacon" / "governance"
            docs.mkdir(parents=True)
            (docs / "version-truth-catalog.md").write_text(
                (ROOT / "docs/beacon/governance/version-truth-catalog.md").read_text(encoding="utf-8"),
                encoding="utf-8",
            )
            (root / "pyproject.toml").write_text(
                '[project]\nname = "agent-delivery-bus"\nversion = "0.0.0"\n',
                encoding="utf-8",
            )
            (root / "AGENTS.md").write_text(
                "Runtime target version: `v9.9.9`\nDocs target version: `v0.0.0`\n",
                encoding="utf-8",
            )
            (root / "CLAUDE.md").write_text(
                "Runtime target version: `v9.9.9`\nDocs target version: `v0.0.0`\n",
                encoding="utf-8",
            )
            problems = check_alignment(root)
            self.assertTrue(any("pyproject.version" in p for p in problems))
            self.assertTrue(any("AGENTS.md" in p for p in problems))
            self.assertTrue(any("CLAUDE.md" in p for p in problems))

    def test_onboarding_state_checked_when_present(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            docs = root / "docs" / "beacon" / "governance"
            docs.mkdir(parents=True)
            (docs / "version-truth-catalog.md").write_text(
                (ROOT / "docs/beacon/governance/version-truth-catalog.md").read_text(encoding="utf-8"),
                encoding="utf-8",
            )
            (root / "pyproject.toml").write_text(
                '[project]\nname = "agent-delivery-bus"\nversion = "0.1.4"\n',
                encoding="utf-8",
            )
            (root / "AGENTS.md").write_text(
                "Runtime target version: `v1.6.12`\nDocs target version: `v0.1.4`\n",
                encoding="utf-8",
            )
            (root / "CLAUDE.md").write_text(
                "Runtime target version: `v1.6.12`\nDocs target version: `v0.1.4`\n",
                encoding="utf-8",
            )
            state = root / ".beacon" / "state"
            state.mkdir(parents=True)
            (state / "project-onboarding.json").write_text(
                json.dumps({"docs_version": "v0.0.5", "runtime_version": "v1.6.11"}),
                encoding="utf-8",
            )
            problems = check_alignment(root)
            self.assertTrue(any("onboarding" in p for p in problems))

    def test_git_tag_exists_and_check_tag_missing(self):
        self.assertFalse(git_tag_exists(ROOT, "v0.1.4-definitely-missing"))
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            subprocess.run(["git", "-C", str(root), "init", "-q"], check=True)
            subprocess.run(
                ["git", "-C", str(root), "config", "user.email", "t@example.com"],
                check=True,
            )
            subprocess.run(
                ["git", "-C", str(root), "config", "user.name", "Test"],
                check=True,
            )
            (root / "placeholder.txt").write_text("x", encoding="utf-8")
            subprocess.run(
                ["git", "-C", str(root), "add", "placeholder.txt"],
                check=True,
            )
            subprocess.run(
                ["git", "-C", str(root), "commit", "-m", "init"],
                check=True,
                capture_output=True,
            )
            subprocess.run(["git", "-C", str(root), "tag", "v9.9.9"], check=True)
            self.assertTrue(git_tag_exists(root, "v9.9.9"))
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            docs = root / "docs" / "beacon" / "governance"
            docs.mkdir(parents=True)
            (docs / "version-truth-catalog.md").write_text(
                (ROOT / "docs/beacon/governance/version-truth-catalog.md").read_text(encoding="utf-8"),
                encoding="utf-8",
            )
            (root / "pyproject.toml").write_text(
                '[project]\nname = "agent-delivery-bus"\nversion = "0.1.4"\n',
                encoding="utf-8",
            )
            (root / "AGENTS.md").write_text(
                "Runtime target version: `v1.6.12`\nDocs target version: `v0.1.4`\n",
                encoding="utf-8",
            )
            (root / "CLAUDE.md").write_text(
                "Runtime target version: `v1.6.12`\nDocs target version: `v0.1.4`\n",
                encoding="utf-8",
            )
            problems = check_alignment(root, check_tag=True)
            self.assertTrue(any("git tag v0.1.4 missing" in p for p in problems))


if __name__ == "__main__":
    unittest.main()
