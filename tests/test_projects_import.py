"""AC-001..AC-006: `adb projects import --scan` (projects-import feature package).

Behaviour covered (see docs/beacon/v0.1.5/features/projects-import/tests.md):

* TC-PI-001 scan / TC-PI-002 dry_run / TC-PI-003 apply /
  TC-PI-004 existing / TC-PI-005 kernel / TC-PI-006 errors

Every case uses a temp tree.  The kernel is never called for real: the module
level runner is injected, and the CLI path patches ``subprocess.run`` inside
``agent_delivery_bus.projects_import`` so no beacon-go binary is required.
"""

from __future__ import annotations

import contextlib
import io
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from agent_delivery_bus.cli import main
from agent_delivery_bus.projects_import import (
    NO_PROJECTS_FOUND,
    REGISTRY_INVALID,
    SCAN_DIR_MISSING,
    register_kernel,
    scan_projects,
    summary_line,
)
from agent_delivery_bus.registry import ProjectRegistry

TS = "2026-08-01"


# --- fixtures ---------------------------------------------------------------


def make_repo(root: Path, name: str, versions: list[str], *, features: bool = True) -> Path:
    """A candidate repo: ``docs/beacon/<v>/features/`` per version."""
    repo = root / name
    for version in versions:
        version_dir = repo / "docs" / "beacon" / version
        if features:
            (version_dir / "features" / "demo").mkdir(parents=True, exist_ok=True)
        else:
            version_dir.mkdir(parents=True, exist_ok=True)
    if not versions:
        repo.mkdir(parents=True, exist_ok=True)
    return repo


def write_json(path: Path, payload: dict) -> Path:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return path


def read_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def run_cli(argv: list[str]) -> tuple[int, str]:
    """Run the CLI in-process and capture stdout."""
    buffer = io.StringIO()
    with contextlib.redirect_stdout(buffer):
        code = main(argv)
    return code, buffer.getvalue()


def run_cli_json(argv: list[str]) -> tuple[int, dict, str]:
    code, output = run_cli([*argv, "--json"])
    return code, json.loads(output), output


def base_registry(projects: list[dict] | None = None) -> dict:
    return {
        "schema_version": "1.0",
        "updated": TS,
        "adapters": {"executor": "hermes", "truth_gate": "beacon", "memory": "inprocess"},
        "notes": {"knowledge_os": "keep inspiration folders out of this registry"},
        "workflows": {"beacon-goal": {"name": "Beacon Goal (private)"}},
        "projects": list(projects or []),
    }


def registry_row(slug: str, repo: Path, version: str, index: int) -> dict:
    return {
        "index": index,
        "slug": slug,
        "title": slug.title(),
        "class": "managed",
        "repo": str(repo.resolve()),
        "aliases": [f"{slug}-alias"],
        "dispatchable": True,
        "status": "active",
        "docs_root": "docs/beacon",
        "docs_version": version,
    }


# --- TC-PI-001: scanning ----------------------------------------------------


class ScanTests(unittest.TestCase):
    def test_scan_collects_only_direct_subdirs_with_features(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            scan = root / "repos"
            scan.mkdir()
            alpha = make_repo(scan, "alpha", ["v0.1.0"])
            beta = make_repo(scan, "beta", ["v0.0.9"])
            make_repo(scan, "no-truth", [])  # no docs/beacon at all
            make_repo(scan, "docs-no-features", ["v9.9.9"], features=False)
            (scan / "just-a-file.txt").write_text("x", encoding="utf-8")
            # a nested candidate must not be discovered: scanning is one level deep
            deep = scan / "nested" / "inner"
            (deep / "docs" / "beacon" / "v1.0.0" / "features").mkdir(parents=True)

            entries = scan_projects(scan)
            self.assertEqual([entry["slug"] for entry in entries], ["alpha", "beta"])
            self.assertEqual(entries[0]["repo"], str(alpha.resolve()))
            self.assertEqual(entries[1]["repo"], str(beta.resolve()))
            self.assertEqual(entries[0]["docs_version"], "v0.1.0")

            code, payload, _ = run_cli_json(
                ["--config", str(root / "projects.json"), "projects", "import", "--scan", str(scan)]
            )
            self.assertEqual(code, 0)
            self.assertEqual(payload["status"], "pass")
            self.assertEqual(payload["data"]["counts"]["found"], 2)
            self.assertIn("found=2", payload["data"]["text"])

    def test_scan_picks_highest_version_numeric_aware(self):
        with tempfile.TemporaryDirectory() as tmp:
            scan = Path(tmp) / "repos"
            scan.mkdir()
            make_repo(scan, "numeric", ["v0.1.9", "v0.1.10", "v0.1.2"])
            make_repo(scan, "nofeature-newer", ["v0.1.10"], features=False)
            make_repo(scan, "nofeature-newer", ["v0.1.9"])

            entries = {entry["slug"]: entry for entry in scan_projects(scan)}
            self.assertEqual(sorted(entries), ["nofeature-newer", "numeric"])
            # v0.1.10 has features and beats v0.1.9 / v0.1.2 numerically
            self.assertEqual(entries["numeric"]["docs_version"], "v0.1.10")
            # a newer version dir *without* features does not win: the highest
            # version that actually carries a features tree is the answer
            self.assertEqual(entries["nofeature-newer"]["docs_version"], "v0.1.9")


# --- TC-PI-002: dry run -----------------------------------------------------


class DryRunTests(unittest.TestCase):
    def test_dry_run_writes_nothing_and_prints_would_be_entries(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            scan = root / "repos"
            scan.mkdir()
            alpha = make_repo(scan, "alpha", ["v0.1.0"])
            registry_path = write_json(
                root / "config" / "projects.json",
                base_registry([registry_row("demo-app", make_repo(root, "demo-app", ["v0.0.1"]), "v0.0.1", 1)]),
            )
            before = registry_path.read_bytes()

            code, payload, output = run_cli_json(
                ["--config", str(registry_path), "projects", "import", "--scan", str(scan)]
            )

            self.assertEqual(code, 0)
            self.assertTrue(payload["data"]["dry_run"])
            self.assertEqual(payload["data"]["written"], [])
            self.assertEqual(registry_path.read_bytes(), before)  # untouched bytes
            self.assertIn("alpha", output)
            self.assertIn(str(alpha.resolve()), output)
            self.assertIn("v0.1.0", output)
            self.assertEqual(payload["data"]["counts"]["new"], 1)

            text_lines = payload["data"]["text"].splitlines()
            self.assertEqual(
                text_lines[0], "found=1 new=1 existing=0 updated=0 kernel_failed=0"
            )

    def test_dry_run_creates_no_registry_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            scan = root / "repos"
            scan.mkdir()
            make_repo(scan, "alpha", ["v0.1.0"])
            registry_path = root / "config" / "projects.json"
            self.assertFalse(registry_path.exists())

            code, payload, _ = run_cli_json(
                ["--config", str(registry_path), "projects", "import", "--scan", str(scan)]
            )

            self.assertEqual(code, 0)
            self.assertFalse(registry_path.exists())
            self.assertEqual(payload["data"]["written"], [])


# --- TC-PI-003: apply ------------------------------------------------------


class ApplyTests(unittest.TestCase):
    def test_apply_writes_registry_and_preserves_other_fields(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            scan = root / "repos"
            scan.mkdir()
            alpha = make_repo(scan, "alpha", ["v0.1.0"])
            beta = make_repo(scan, "beta", ["v0.2.3"])
            demo = make_repo(root, "demo-app", ["v0.0.1"])
            original = base_registry([registry_row("demo-app", demo, "v0.0.1", 1)])
            registry_path = write_json(root / "config" / "projects.json", original)

            code, payload, _ = run_cli_json(
                [
                    "--config",
                    str(registry_path),
                    "projects",
                    "import",
                    "--scan",
                    str(scan),
                    "--apply",
                ]
            )
            self.assertEqual(code, 0)
            self.assertFalse(payload["data"]["dry_run"])
            self.assertEqual(payload["data"]["written"], [str(registry_path)])
            self.assertEqual(payload["data"]["counts"]["new"], 2)

            written = read_json(registry_path)
            # AC-003: every pre-existing top-level key survives verbatim
            for key in ("schema_version", "updated", "adapters", "notes", "workflows"):
                self.assertEqual(written[key], original[key], key)
            slugs = [row["slug"] for row in written["projects"]]
            self.assertEqual(slugs, ["demo-app", "alpha", "beta"])
            by_slug = {row["slug"]: row for row in written["projects"]}
            self.assertEqual(by_slug["alpha"]["repo"], str(alpha.resolve()))
            self.assertEqual(by_slug["alpha"]["docs_version"], "v0.1.0")
            self.assertEqual(by_slug["alpha"]["class"], "managed")
            self.assertEqual(by_slug["alpha"]["docs_root"], "docs/beacon")
            self.assertEqual(by_slug["beta"]["docs_version"], "v0.2.3")
            self.assertEqual(by_slug["demo-app"], original["projects"][0])
            # index auto-assignment never reuses an existing number
            self.assertEqual(by_slug["alpha"]["index"], 2)
            self.assertEqual(by_slug["beta"]["index"], 3)

            # the written registry is a real registry: the strict loader accepts it
            registry = ProjectRegistry.load(registry_path)
            self.assertEqual([item.slug for item in registry.list()], ["alpha", "beta", "demo-app"])

            code, listed, _ = run_cli_json(
                ["--config", str(registry_path), "projects", "list"]
            )
            self.assertEqual(code, 0)
            self.assertEqual(len(listed["data"]["projects"]), 3)


# --- TC-PI-004: existing / overwrite --------------------------------------


class ExistingTests(unittest.TestCase):
    def test_existing_slug_is_skipped_unless_overwrite(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            scan = root / "repos"
            scan.mkdir()
            scanned_alpha = make_repo(scan, "alpha", ["v0.2.0"])
            make_repo(scan, "beta", ["v0.1.0"])
            # the registered alpha points somewhere else, at an older version
            old_alpha = make_repo(root / "old", "alpha", ["v0.1.0"])
            row = registry_row("alpha", old_alpha, "v0.1.0", 4)
            row["title"] = "Alpha legacy"
            row["aliases"] = ["a-legacy"]
            registry_path = write_json(
                root / "config" / "projects.json", base_registry([row])
            )

            # --- default: skip, keep the registered row exactly as it was
            code, payload, _ = run_cli_json(
                ["--config", str(registry_path), "projects", "import", "--scan", str(scan), "--apply"]
            )
            self.assertEqual(code, 0)
            counts = payload["data"]["counts"]
            self.assertEqual((counts["found"], counts["new"], counts["existing"], counts["updated"]), (2, 1, 1, 0))
            self.assertEqual([item["slug"] for item in payload["data"]["existing"]], ["alpha"])
            written = read_json(registry_path)
            kept = {item["slug"]: item for item in written["projects"]}
            self.assertEqual(kept["alpha"], row)  # untouched: repo/version/index/title
            self.assertIn("alpha", [item["slug"] for item in written["projects"]])
            self.assertNotEqual(kept["alpha"]["repo"], str(scanned_alpha.resolve()))

            # --- --overwrite: refresh repo + docs_version, keep identity fields
            code, payload, _ = run_cli_json(
                [
                    "--config",
                    str(registry_path),
                    "projects",
                    "import",
                    "--scan",
                    str(scan),
                    "--apply",
                    "--overwrite",
                ]
            )
            self.assertEqual(code, 0)
            counts = payload["data"]["counts"]
            self.assertEqual((counts["found"], counts["new"], counts["existing"], counts["updated"]), (2, 0, 0, 2))
            written = read_json(registry_path)
            refreshed = {item["slug"]: item for item in written["projects"]}
            self.assertEqual(refreshed["alpha"]["repo"], str(scanned_alpha.resolve()))
            self.assertEqual(refreshed["alpha"]["docs_version"], "v0.2.0")
            self.assertEqual(refreshed["alpha"]["index"], 4)
            self.assertEqual(refreshed["alpha"]["title"], "Alpha legacy")
            self.assertEqual(refreshed["alpha"]["aliases"], ["a-legacy"])
            self.assertEqual(len(written["projects"]), 2)  # nothing duplicated


# --- TC-PI-005: kernel registration ---------------------------------------


class KernelTests(unittest.TestCase):
    def test_kernel_failure_does_not_block_json_and_counts_failures(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            scan = root / "repos"
            scan.mkdir()
            make_repo(scan, "alpha", ["v0.1.0"])
            make_repo(scan, "beta", ["v0.1.0"])
            registry_path = write_json(root / "config" / "projects.json", base_registry())

            calls: list[list[str]] = []

            class _Failing:
                returncode = 1
                stderr = "kernel exploded"

            def fake_run(argv, **kwargs):  # no real beacon-go in tests
                calls.append(list(argv))
                return _Failing() if "beta" in " ".join(argv) else type(
                    "_Ok", (), {"returncode": 0, "stderr": ""}
                )()

            with mock.patch("agent_delivery_bus.projects_import.subprocess.run", fake_run):
                code, payload, output = run_cli_json(
                    [
                        "--config",
                        str(registry_path),
                        "projects",
                        "import",
                        "--scan",
                        str(scan),
                        "--apply",
                        "--register-kernel",
                    ]
                )

            self.assertEqual(code, 0)
            counts = payload["data"]["counts"]
            self.assertEqual(counts["kernel_failed"], 1)
            self.assertIn("kernel_failed=1", payload["data"]["text"])
            # AC-005: the JSON write happened anyway
            written = read_json(registry_path)
            self.assertEqual([row["slug"] for row in written["projects"]], ["alpha", "beta"])
            # both entries were attempted with the frozen command shape
            self.assertEqual(len(calls), 2)
            for argv in calls:
                self.assertEqual(argv[0], "beacon-go")
                self.assertEqual(argv[1:3], ["projects", "register"])
                self.assertIn("--project", argv)
                self.assertIn("--version", argv)
            self.assertIn("beta", output)

    def test_register_kernel_never_raises(self):
        entries = [
            {"slug": "alpha", "repo": "/tmp/alpha", "docs_version": "v1.0.0"},
            {"slug": "beta", "repo": "/tmp/beta", "docs_version": "v1.0.0"},
            {"slug": "gamma", "repo": "", "docs_version": "v1.0.0"},
        ]

        def raising_runner(argv):
            raise OSError("beacon-go not installed")

        ok, failed = register_kernel(entries, runner=raising_runner)
        self.assertEqual(ok, [])
        self.assertEqual(len(failed), 3)
        self.assertEqual(failed[0]["reason"], "kernel_call_failed")
        self.assertEqual(failed[2]["reason"], "repo_missing")

        def mixed_runner(argv):
            return 0 if "alpha" in " ".join(argv) else 2

        ok, failed = register_kernel(entries[:2], runner=mixed_runner)
        self.assertEqual([row["slug"] for row in ok], ["alpha"])
        self.assertEqual([row["slug"] for row in failed], ["beta"])
        self.assertEqual(failed[0]["reason"], "kernel_exit_nonzero")
        self.assertEqual(
            ok[0]["command"],
            ["beacon-go", "projects", "register", "--project", "/tmp/alpha", "--version", "v1.0.0"],
        )


# --- TC-PI-006: reason codes ----------------------------------------------


class ErrorsTests(unittest.TestCase):
    def test_errors_report_reason_codes_with_exit_zero(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)

            # 1) scan dir missing
            missing = root / "nope"
            registry_path = write_json(root / "config" / "projects.json", base_registry())
            before = registry_path.read_bytes()
            code, payload, _ = run_cli_json(
                ["--config", str(registry_path), "projects", "import", "--scan", str(missing), "--apply"]
            )
            self.assertEqual(code, 0)
            self.assertEqual(payload["status"], "pass")
            self.assertEqual(payload["reason_code"], SCAN_DIR_MISSING)
            self.assertEqual(registry_path.read_bytes(), before)
            self.assertEqual(summary_line(payload["data"]["counts"]), "found=0 new=0 existing=0 updated=0 kernel_failed=0")

            # 2) nothing to find
            empty = root / "empty"
            empty.mkdir()
            make_repo(empty, "plain-repo", [])  # no beacon docs tree
            code, payload, _ = run_cli_json(
                ["--config", str(registry_path), "projects", "import", "--scan", str(empty), "--apply"]
            )
            self.assertEqual(code, 0)
            self.assertEqual(payload["reason_code"], NO_PROJECTS_FOUND)
            self.assertEqual(registry_path.read_bytes(), before)

            # 3) registry invalid
            scan = root / "repos"
            scan.mkdir()
            make_repo(scan, "alpha", ["v0.1.0"])
            broken = root / "config" / "broken.json"
            broken.write_text("{ not json", encoding="utf-8")
            code, payload, _ = run_cli_json(
                ["--config", str(broken), "projects", "import", "--scan", str(scan), "--apply"]
            )
            self.assertEqual(code, 0)
            self.assertEqual(payload["reason_code"], REGISTRY_INVALID)
            self.assertEqual(broken.read_text(encoding="utf-8"), "{ not json")

            # 3b) a structurally broken registry is refused the same way
            wrong_shape = write_json(root / "config" / "wrong.json", {"projects": "nope"})
            code, payload, _ = run_cli_json(
                ["--config", str(wrong_shape), "projects", "import", "--scan", str(scan)]
            )
            self.assertEqual(code, 0)
            self.assertEqual(payload["reason_code"], REGISTRY_INVALID)

            # 3c) duplicate slugs are a broken registry, not an import source
            dupe = write_json(
                root / "config" / "dupe.json",
                base_registry(
                    [
                        registry_row("alpha", scan / "alpha", "v0.1.0", 1),
                        registry_row("alpha", scan / "alpha", "v0.1.0", 2),
                    ]
                ),
            )
            code, payload, _ = run_cli_json(
                ["--config", str(dupe), "projects", "import", "--scan", str(scan), "--apply"]
            )
            self.assertEqual(code, 0)
            self.assertEqual(payload["reason_code"], REGISTRY_INVALID)

            # the human-readable form still carries the count line
            code, output = run_cli(
                ["--config", str(registry_path), "projects", "import", "--scan", str(missing)]
            )
            self.assertEqual(code, 0)
            self.assertIn("scan_dir_missing", output)


if __name__ == "__main__":  # pragma: no cover
    unittest.main()
