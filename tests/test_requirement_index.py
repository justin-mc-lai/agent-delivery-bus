"""AC-001..AC-009: requirement-index projection (beacon truth + ADB exec state)."""

from __future__ import annotations

import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.requirement_index import (
    BUDGET_BYTES,
    EXEC_STATES,
    LIFECYCLE_SOURCE_MISSING,
    RECORD_FIELDS,
    SCHEMA_VERSION,
    SKIP_STORE_NOT_REGISTERED,
    TRUTH_STAGES,
    RequirementSource,
    build_index,
    choose_format,
    default_lifecycle_provider,
    describe_index,
    dumps,
    enumerate_sources,
    generate_index,
    parse_frontmatter,
    req_id_for,
    resolve_version,
    set_exec_state,
    shard_path_for,
    write_index,
)
from agent_delivery_bus.storage import Storage

STAMP = "2026-11-01T07:41:10+00:00"

ROOT = Path(__file__).resolve().parents[1]
CLI = [sys.executable, "-m", "agent_delivery_bus.cli"]


def _sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def _package(
    root: Path,
    project: str,
    feature: str,
    *,
    stage: str = "truth",
    truth_ref: str = "",
    evidence_ref: str = "",
    padding: str = "",
) -> RequirementSource:
    """Write one beacon package projection and return its source descriptor."""
    packages = root / "packages"
    packages.mkdir(parents=True, exist_ok=True)
    declared_truth = truth_ref or f"docs/beacon/v0.1.5/features/{feature}/truth.md"
    path = packages / f"{project}.{feature}.json"
    payload = {
        "lifecycle": {"stage": stage},
        "truth_ref": declared_truth,
        "evidence_ref": evidence_ref,
    }
    if padding:
        payload["padding"] = padding
    path.write_text(json.dumps(payload), encoding="utf-8")
    return RequirementSource(
        req_id=req_id_for(project, feature),
        project=project,
        payload_path=str(path),
        truth_ref=declared_truth,
        evidence_ref=evidence_ref,
    )


def _by_req_id(document: dict) -> dict:
    return {record["req_id"]: record for record in document["requirements"]}


class RequirementIndexSchemaTests(unittest.TestCase):
    """AC-001: schema_version + generated_from[] + the 11 contract fields."""

    def test_schema_contract_and_parseable_index(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            sources = [
                _package(root, "agent-delivery-bus", "requirement-index", stage="implement"),
                _package(root, "shopxo-canada", "order-sync", stage="qa"),
            ]
            bundle = build_index(sources, generated_at=STAMP)
            out = root / "out" / "requirement-index.json"
            write_index(bundle, out)

            with out.open("r", encoding="utf-8") as handle:
                document = json.load(handle)

            self.assertEqual(document["schema_version"], SCHEMA_VERSION)
            self.assertEqual(document["format"], "full")
            self.assertEqual(
                document["generated_from"], sorted(source.payload_path for source in sources)
            )
            self.assertEqual(len(RECORD_FIELDS), 11)
            self.assertGreater(len(document["requirements"]), 0)
            for record in document["requirements"]:
                self.assertEqual(set(record), set(RECORD_FIELDS))
                self.assertIn(record["truth_stage"], TRUTH_STAGES)
                self.assertIn(record["exec_state"], EXEC_STATES)
                self.assertEqual(record["updated_at"], STAMP)
                self.assertEqual(record["source_status"], "ok")
                self.assertTrue(record["truth_ref"])
            self.assertEqual(
                [record["req_id"] for record in document["requirements"]],
                sorted(record["req_id"] for record in document["requirements"]),
            )
            stages = {record["project"]: record["truth_stage"] for record in document["requirements"]}
            self.assertEqual(stages["agent-delivery-bus"], "implement")
            self.assertEqual(stages["shopxo-canada"], "qa")
            self.assertEqual(dumps(document), out.read_text(encoding="utf-8"))


class RequirementIndexIdempotencyTests(unittest.TestCase):
    """AC-002: same input -> byte identical output, no duplicate rows."""

    def test_idempotent_generation_is_byte_identical(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            sources = [
                _package(root, "alpha", "feature-one", stage="truth"),
                _package(root, "beta", "feature-two", stage="qa"),
            ]
            first = build_index(sources, generated_at=STAMP)
            second = build_index(sources, generated_at=STAMP)
            self.assertEqual(dumps(first.document), dumps(second.document))
            self.assertEqual(
                first.document["content_hash"], second.document["content_hash"]
            )

            out = root / "out" / "requirement-index.json"
            result_first = write_index(first, out)
            bytes_first = out.read_bytes()
            self.assertIn(str(out), result_first["written"])

            result_second = write_index(second, out)
            self.assertEqual(out.read_bytes(), bytes_first)
            self.assertIn(str(out), result_second["skipped"])

            # A different clock changes the stamps but not the content hash.
            later = build_index(sources, generated_at="2027-01-01T00:00:00+00:00")
            self.assertEqual(first.document["content_hash"], later.document["content_hash"])
            self.assertEqual(
                len(first.document["requirements"]), len(later.document["requirements"])
            )

            # Re-feeding an already indexed package must not duplicate rows.
            duplicated = build_index(sources + sources, generated_at=STAMP)
            self.assertEqual(len(duplicated.document["requirements"]), 2)
            self.assertEqual(
                sorted(entry["reason"] for entry in duplicated.document["missing"]),
                ["duplicate_source", "duplicate_source"],
            )

    def test_serialisation_is_key_sorted(self):
        with tempfile.TemporaryDirectory() as tmp:
            source = _package(Path(tmp), "alpha", "feature-one")
            text = dumps(build_index([source], generated_at=STAMP).document)
            self.assertEqual(
                text, json.dumps(json.loads(text), ensure_ascii=False, sort_keys=True, indent=2) + "\n"
            )


class RequirementIndexMissingSourceTests(unittest.TestCase):
    """AC-003: the three source anomalies are recorded, never papered over."""

    def _unreadable_payload(self, root: Path) -> Path:
        path = root / "packages" / "broken-unreadable.json"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text('{"lifecycle": {"stage": "truth"}}', encoding="utf-8")
        os.chmod(path, 0o000)
        self.addCleanup(lambda: os.chmod(path, 0o644) if path.exists() else None)
        if os.access(path, os.R_OK):  # running as root: a directory is unreadable
            path.unlink()
            path.mkdir()
        return path

    def test_missing_sources_are_recorded_not_filled(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            healthy = _package(root, "alpha", "healthy", stage="truth")
            absent = RequirementSource(
                req_id=req_id_for("alpha", "absent"),
                project="alpha",
                payload_path=str(root / "packages" / "never-written.json"),
                truth_ref="docs/beacon/v0.1.5/features/absent/truth.md",
            )
            unreadable = RequirementSource(
                req_id=req_id_for("alpha", "unreadable"),
                project="alpha",
                payload_path=str(self._unreadable_payload(root)),
            )
            corrupt = root / "packages" / "corrupt.json"
            corrupt.write_text("{ this is not json", encoding="utf-8")
            invalid = RequirementSource(
                req_id=req_id_for("alpha", "corrupt"),
                project="alpha",
                payload_path=str(corrupt),
            )

            bundle = build_index(
                [healthy, absent, unreadable, invalid], generated_at=STAMP
            )
            document = bundle.document
            self.assertEqual(len(document["requirements"]), 4)  # nothing deleted

            records = _by_req_id(document)
            for req_id in (
                absent.req_id,
                unreadable.req_id,
                invalid.req_id,
            ):
                record = records[req_id]
                self.assertEqual(record["source_status"], "unknown")
                self.assertEqual(record["truth_stage"], "unknown")
                # no default value is invented for the writable columns
                self.assertEqual(record["thread_id"], "")
                self.assertEqual(record["worker"], "")
                self.assertEqual(record["blockers"], [])
                self.assertEqual(record["evidence_ref"], "")

            self.assertEqual(records[healthy.req_id]["source_status"], "ok")
            self.assertEqual(records[healthy.req_id]["truth_stage"], "truth")

            self.assertEqual(
                [(entry["reason"], entry["req_id"]) for entry in document["missing"]],
                [
                    ("source_invalid_json", invalid.req_id),
                    ("source_missing", absent.req_id),
                    ("source_unreadable", unreadable.req_id),
                ],
            )
            for entry in document["missing"]:
                self.assertEqual(set(entry), {"source", "reason", "req_id"})
            # declared pointers survive the broken read for chain recall
            self.assertEqual(records[absent.req_id]["truth_ref"], absent.truth_ref)
            self.assertEqual(
                records[absent.req_id]["req_id"], "req:alpha:absent"
            )
            self.assertEqual(document["requirements"], sorted(
                document["requirements"], key=lambda item: item["req_id"]
            ))

    def test_lifecycle_missing_is_not_guessed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            packages = root / "packages"
            packages.mkdir(parents=True, exist_ok=True)
            path = packages / "no-lifecycle.json"
            path.write_text(json.dumps({"truth_ref": "docs/truth.md"}), encoding="utf-8")
            source = RequirementSource(
                req_id=req_id_for("alpha", "no-lifecycle"),
                project="alpha",
                payload_path=str(path),
            )
            document = build_index([source], generated_at=STAMP).document
            record = document["requirements"][0]
            self.assertEqual(record["source_status"], "unknown")
            self.assertEqual(record["truth_stage"], "unknown")
            self.assertEqual(record["truth_ref"], "docs/truth.md")
            # The payload loaded but carries no lifecycle answer (AC-009).
            self.assertEqual(document["missing"][0]["reason"], LIFECYCLE_SOURCE_MISSING)


class RequirementIndexSelfContainedTests(unittest.TestCase):
    """AC-004: one file is enough to read the whole index."""

    def test_selfcontained_single_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            sources = [
                _package(root, "alpha", "one", stage="plan"),
                _package(root, "beta", "two", stage="implement"),
            ]
            out = root / "out" / "requirement-index.json"
            write_index(build_index(sources, generated_at=STAMP), out)

            detached = root / "elsewhere"
            detached.mkdir()
            copy = detached / "requirement-index.json"
            shutil.copy(out, copy)

            with copy.open("r", encoding="utf-8") as handle:
                document = json.load(handle)
            self.assertEqual(document["schema_version"], SCHEMA_VERSION)
            self.assertEqual(len(document["requirements"]), 2)
            for record in document["requirements"]:
                self.assertEqual(set(record), set(RECORD_FIELDS))

    def test_selfcontained_layered_summary_points_at_shards(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            sources = [
                _package(root, "alpha", f"feature-{index}", padding="x" * 400)
                for index in range(20)
            ]
            out = root / "out" / "requirement-index.json"
            result = write_index(
                build_index(sources, generated_at=STAMP, budget=600), out
            )
            self.assertEqual(result["format"], "summary")

            with out.open("r", encoding="utf-8") as handle:
                summary = json.load(handle)
            self.assertEqual(summary["format"], "summary")
            self.assertEqual(summary["missing"], [])
            self.assertEqual(len(summary["requirements"]), len(sources))
            for entry in summary["requirements"]:
                for key in ("req_id", "truth_stage", "exec_state", "truth_ref"):
                    self.assertIn(key, entry)
                shard = out.parent / entry["shard"]
                self.assertTrue(shard.is_file())
                with shard.open("r", encoding="utf-8") as handle:
                    shard_document = json.load(handle)
                self.assertEqual(shard_document["project"], "alpha")
                shard_records = _by_req_id(shard_document)
                self.assertEqual(set(shard_records[entry["req_id"]]), set(RECORD_FIELDS))
                self.assertEqual(
                    shard_records[entry["req_id"]]["truth_ref"], entry["truth_ref"]
                )


class RequirementIndexWriteAuthorityTests(unittest.TestCase):
    """AC-005: exec_state is writable, truth_stage is not."""

    def test_write_authority(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            storage = Storage(str(root / "adb.sqlite3"))
            self.addCleanup(storage.close)
            sources = [_package(root, "alpha", "one", stage="truth")]
            out = root / "out" / "requirement-index.json"
            write_index(generate_index(sources, storage=storage, generated_at=STAMP), out)
            before = _sha256(out)

            row = set_exec_state(
                storage,
                sources[0].req_id,
                {
                    "exec_state": "running",
                    "thread_id": "thread-42",
                    "worker": "pi@mac-mini",
                    "blockers": ["waiting on vpn"],
                },
                actor="ops",
            )
            self.assertEqual(row["exec_state"], "running")
            self.assertEqual(row["blockers"], ["waiting on vpn"])
            stored = storage.get_requirement_exec_state(sources[0].req_id)
            self.assertIsNotNone(stored)
            self.assertEqual(stored["worker"], "pi@mac-mini")
            self.assertEqual(stored["thread_id"], "thread-42")
            events = storage.list_requirement_exec_events(sources[0].req_id)
            self.assertEqual([(event["from_state"], event["to_state"]) for event in events], [("", "running")])

            record = _by_req_id(
                generate_index(sources, storage=storage, generated_at=STAMP).document
            )[sources[0].req_id]
            self.assertEqual(record["exec_state"], "running")
            self.assertEqual(record["thread_id"], "thread-42")
            self.assertEqual(record["worker"], "pi@mac-mini")
            self.assertEqual(record["blockers"], ["waiting on vpn"])
            # exec_state never lifts truth_stage.
            self.assertEqual(record["truth_stage"], "truth")

            write_index(generate_index(sources, storage=storage, generated_at=STAMP), out)
            after_successful_write = _sha256(out)
            # The ADB-owned write is visible in the derived view ...
            self.assertNotEqual(after_successful_write, before)

            with self.assertRaises(DeliveryBusError) as ctx:
                set_exec_state(storage, sources[0].req_id, {"truth_stage": "release"})
            self.assertEqual(ctx.exception.reason_code, "truth_stage_readonly")
            write_index(generate_index(sources, storage=storage, generated_at=STAMP), out)
            # ... while a refused truth write leaves the index byte-identical.
            self.assertEqual(_sha256(out), after_successful_write)
            self.assertEqual(
                _by_req_id(generate_index(sources, storage=storage, generated_at=STAMP).document)[
                    sources[0].req_id
                ]["truth_stage"],
                "truth",
            )

            columns = {
                info["name"]
                for info in storage.conn.execute(
                    "PRAGMA table_info(requirement_exec_state)"
                ).fetchall()
            }
            self.assertNotIn("truth_stage", columns)

    def test_illegal_truth_stage_write_leaves_index_untouched(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            storage = Storage(str(root / "adb.sqlite3"))
            self.addCleanup(storage.close)
            source = _package(root, "alpha", "one", stage="truth")
            out = root / "out" / "requirement-index.json"
            write_index(generate_index([source], storage=storage, generated_at=STAMP), out)
            before = _sha256(out)

            for field in ("truth_stage", "truth_ref", "evidence_ref", "project", "source_status"):
                with self.assertRaises(DeliveryBusError) as ctx:
                    set_exec_state(storage, source.req_id, {field: "release"})
                self.assertEqual(ctx.exception.reason_code, "truth_stage_readonly")
            with self.assertRaises(DeliveryBusError) as ctx:
                set_exec_state(storage, source.req_id, {"exec_state": "degraded"})
            self.assertEqual(ctx.exception.reason_code, "exec_state_invalid")

            self.assertIsNone(storage.get_requirement_exec_state(source.req_id))
            write_index(generate_index([source], storage=storage, generated_at=STAMP), out)
            self.assertEqual(_sha256(out), before)


class RequirementIndexLayeringTests(unittest.TestCase):
    """AC-006: above 10240 bytes the index becomes summary + shards."""

    def test_layering_above_10kb_budget(self):
        self.assertEqual(choose_format(BUDGET_BYTES), "full")
        self.assertEqual(choose_format(BUDGET_BYTES + 1), "summary")
        self.assertEqual(choose_format(1), "full")

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            small = [_package(root, "alpha", "one")]
            small_bundle = build_index(small, generated_at=STAMP)
            self.assertEqual(small_bundle.format, "full")
            self.assertEqual(small_bundle.shards, {})
            small_out = root / "small" / "requirement-index.json"
            write_index(small_bundle, small_out)
            self.assertFalse((small_out.parent / "requirements").exists())

            big = [
                _package(
                    root,
                    f"project-{index:02d}",
                    f"feature-{index:02d}",
                    padding="pad-" * 60,
                )
                for index in range(40)
            ]
            unbounded = build_index(big, generated_at=STAMP, budget=10**9)
            self.assertEqual(unbounded.format, "full")
            self.assertGreater(unbounded.size_bytes, BUDGET_BYTES)

            layered = build_index(big, generated_at=STAMP)
            self.assertEqual(layered.format, "summary")
            self.assertEqual(layered.document["format"], "summary")
            self.assertEqual(len(layered.shards), len(big))
            for relative in layered.shards:
                self.assertTrue(relative.startswith("requirements/"))
                self.assertTrue(relative.endswith(".json"))
            self.assertEqual(layered.document["missing"], [])
            self.assertEqual(len(layered.document["requirements"]), len(big))
            for entry in layered.document["requirements"]:
                for key in ("req_id", "truth_stage", "exec_state", "truth_ref"):
                    self.assertIn(key, entry)
                self.assertEqual(entry["shard"], shard_path_for(entry["project"]))

            out = root / "layered" / "requirement-index.json"
            result = write_index(layered, out)
            self.assertEqual(result["format"], "summary")
            self.assertEqual(len(result["shards"]), len(big))
            with out.open("r", encoding="utf-8") as handle:
                summary = json.load(handle)
            shard_total = 0
            for relative, document in layered.shards.items():
                self.assertTrue((out.parent / relative).is_file())
                self.assertEqual(document["format"], "shard")
                shard_total += len(document["requirements"])
            self.assertEqual(shard_total, len(big))
            self.assertEqual(summary["format"], "summary")


class RequirementIndexIllegalSourceTests(unittest.TestCase):
    """AC-003 illegal slot: a corrupt source JSON must not abort generation."""

    def test_illegal_source(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            healthy = _package(root, "alpha", "healthy")
            broken = root / "packages" / "broken.json"
            broken.write_text("[[[", encoding="utf-8")
            document = build_index(
                [
                    healthy,
                    RequirementSource(
                        req_id=req_id_for("alpha", "broken"),
                        project="alpha",
                        payload_path=str(broken),
                    ),
                ],
                generated_at=STAMP,
            ).document
            self.assertEqual(len(document["requirements"]), 2)
            self.assertEqual(
                document["missing"],
                [
                    {
                        "source": str(broken),
                        "reason": "source_invalid_json",
                        "req_id": "req:alpha:broken",
                    }
                ],
            )
            self.assertEqual(
                _by_req_id(document)["req:alpha:broken"]["source_status"], "unknown"
            )
            self.assertEqual(json.loads(dumps(document))["schema_version"], SCHEMA_VERSION)


def _beacon_project(
    root: Path,
    feature: str,
    *,
    version: str = "v0.1.5",
    status: str = "frozen",
    frontmatter: str | None = None,
    evidence: bool = False,
    write_truth: bool = True,
) -> Path:
    """Write one beacon feature package under ``<root>/docs/beacon/<version>``."""
    package = root / "docs" / "beacon" / version / "features" / feature
    package.mkdir(parents=True, exist_ok=True)
    if write_truth:
        if frontmatter is None:
            frontmatter = (
                "---\n"
                f"slug: {feature}\n"
                f"version: {version}\n"
                f"status: {status}\n"
                "language: zh\n"
                "canonical_refs:\n"
                f"  truth: docs/beacon/{version}/features/{feature}/truth.md\n"
                "---\n\n"
                f"# Truth: {feature}\n"
            )
        (package / "truth.md").write_text(frontmatter, encoding="utf-8")
    if evidence:
        (package / "evidence.md").write_text("# Evidence\n", encoding="utf-8")
    return package


def _cli_env(**overrides: str) -> dict[str, str]:
    """Environment for the ``adb`` subprocess: src on path, no inherited clock."""
    env = dict(os.environ)
    existing = env.get("PYTHONPATH", "")
    env["PYTHONPATH"] = str(ROOT / "src") + (os.pathsep + existing if existing else "")
    env.pop("ADB_REQUIREMENT_INDEX_NOW", None)
    env.update(overrides)
    return env


def _fake_lifecycle(mapping: dict[str, dict[str, str]] | None = None):
    """Injected lifecycle provider; tests never spawn the real ``beacon-go``."""
    stages = {slug: dict(value) for slug, value in (mapping or {}).items()}

    def provider(root: Path) -> dict[str, dict[str, str]]:
        return {slug: dict(value) for slug, value in stages.items()}

    return provider


def _fake_kernel_bin(directory: Path, payload: dict[str, object]) -> Path:
    """Write a ``beacon-go`` stub printing ``payload`` (keeps PATH hermetic)."""
    directory.mkdir(parents=True, exist_ok=True)
    stub = directory / "beacon-go"
    stub.write_text(
        f"#!{sys.executable}\n"
        "import json\n"
        f"print(json.dumps({payload!r}))\n",
        encoding="utf-8",
    )
    stub.chmod(0o755)
    return directory


class RequirementIndexEnumeratorTests(unittest.TestCase):
    """AC-007: only frozen beacon packages are enumerated; the rest are recorded."""

    def test_enumerator_collects_only_frozen_packages(self):
        with tempfile.TemporaryDirectory() as tmp:
            workspace = Path(tmp)
            alpha = workspace / "alpha"
            alpha.mkdir()
            _beacon_project(alpha, "requirement-index", evidence=True)

            beta = workspace / "beta"
            beta.mkdir()
            _beacon_project(beta, "frozen-two")
            _beacon_project(beta, "still-draft", status="draft")
            _beacon_project(
                beta, "broken-frontmatter", frontmatter="---\n- not a key/value pair\n---\n"
            )
            _beacon_project(beta, "no-frontmatter", frontmatter="# truth without frontmatter\n")
            _beacon_project(beta, "empty-dir", write_truth=False)
            (beta / "docs" / "beacon" / "v0.1.5" / "features" / "README.txt").write_text(
                "not a package", encoding="utf-8"
            )

            # The third entry is the same root spelled differently: it must not
            # be enumerated twice.  The lifecycle store is injected (never the
            # real kernel) and knows no slug here, so every record is missing
            # its stage.
            sources, skipped = enumerate_sources(
                [alpha, beta, alpha / "."],
                lifecycle_provider=_fake_lifecycle({"unrelated": {"stage": "plan"}}),
            )

            self.assertEqual(
                [source.req_id for source in sources],
                ["req:alpha:requirement-index", "req:beta:frozen-two"],
            )
            self.assertEqual(
                [source.project for source in sources], ["alpha", "beta"]
            )
            alpha_source, beta_source = sources
            self.assertEqual(
                alpha_source.truth_ref,
                "docs/beacon/v0.1.5/features/requirement-index/truth.md",
            )
            self.assertEqual(
                alpha_source.evidence_ref,
                "docs/beacon/v0.1.5/features/requirement-index/evidence.md",
            )
            self.assertEqual(beta_source.evidence_ref, "")
            # Frontmatter facts travel with the source; the lifecycle store had
            # nothing to say, so no stage is claimed.
            self.assertEqual(alpha_source.payload["frontmatter"]["status"], "frozen")
            self.assertEqual(alpha_source.payload["lifecycle"], {})

            self.assertEqual(
                [(entry["reason"], Path(entry["source"]).name) for entry in skipped],
                [
                    ("frontmatter_invalid", "truth.md"),
                    ("frontmatter_invalid", "truth.md"),
                    ("not_frozen", "truth.md"),
                    ("truth_missing", "empty-dir"),
                ],
            )
            self.assertEqual(
                {Path(entry["source"]).parent.name for entry in skipped if entry["reason"] == "frontmatter_invalid"},
                {"broken-frontmatter", "no-frontmatter"},
            )
            self.assertIn(
                "still-draft",
                [
                    Path(entry["source"]).parent.name
                    for entry in skipped
                    if entry["reason"] == "not_frozen"
                ],
            )

            # Enumerated packages feed the projection: all 11 fields, but the
            # lifecycle stage stays unknown (the provider answered with {}).
            document = build_index(sources, generated_at=STAMP).document
            self.assertEqual(len(document["requirements"]), 2)
            for record in document["requirements"]:
                self.assertEqual(set(record), set(RECORD_FIELDS))
                self.assertEqual(record["truth_stage"], "unknown")
                self.assertEqual(record["source_status"], "unknown")
            self.assertEqual(
                sorted(entry["reason"] for entry in document["missing"]),
                [LIFECYCLE_SOURCE_MISSING, LIFECYCLE_SOURCE_MISSING],
            )

    def test_enumerator_missing_root_does_not_raise(self):
        with tempfile.TemporaryDirectory() as tmp:
            workspace = Path(tmp)
            alpha = workspace / "alpha"
            alpha.mkdir()
            _beacon_project(alpha, "one")
            bare = workspace / "bare"
            bare.mkdir()

            sources, skipped = enumerate_sources(
                [workspace / "does-not-exist", "", bare, alpha],
                lifecycle_provider=_fake_lifecycle({"one": {"stage": "truth"}}),
            )

            self.assertEqual([source.req_id for source in sources], ["req:alpha:one"])
            self.assertEqual(
                [entry["reason"] for entry in skipped],
                ["features_missing", "root_missing", "root_missing"],
            )
            self.assertEqual(str(workspace / "does-not-exist"), skipped[2]["source"])
            self.assertEqual("", skipped[1]["source"])

    def test_enumerator_picks_highest_version_and_honours_explicit_version(self):
        with tempfile.TemporaryDirectory() as tmp:
            workspace = Path(tmp)
            gamma = workspace / "gamma"
            gamma.mkdir()
            _beacon_project(gamma, "nine", version="v0.1.9")
            _beacon_project(gamma, "ten", version="v0.1.10")

            self.assertEqual(resolve_version(gamma), "v0.1.10")  # numeric, not lexical
            self.assertEqual(resolve_version(gamma, "v0.1.9"), "v0.1.9")

            provider = _fake_lifecycle(
                {"nine": {"stage": "truth"}, "ten": {"stage": "implement"}}
            )
            sources, skipped = enumerate_sources([gamma], lifecycle_provider=provider)
            self.assertEqual([source.req_id for source in sources], ["req:gamma:ten"])
            self.assertEqual(skipped, [])

            sources, skipped = enumerate_sources(
                [gamma], version="v0.1.9", lifecycle_provider=provider
            )
            self.assertEqual([source.req_id for source in sources], ["req:gamma:nine"])

            sources, skipped = enumerate_sources(
                [gamma], version="v9.9.9", lifecycle_provider=provider
            )
            self.assertEqual(sources, [])
            self.assertEqual([entry["reason"] for entry in skipped], ["features_missing"])

    def test_enumerator_frontmatter_parser_is_strict(self):
        self.assertIsNone(parse_frontmatter("no frontmatter at all\n"))
        self.assertIsNone(parse_frontmatter("---\n---\n"))
        self.assertIsNone(parse_frontmatter("---\n- item\n---\n"))
        self.assertEqual(parse_frontmatter("---\nstatus: frozen\n---\n")["status"], "frozen")
        self.assertEqual(
            parse_frontmatter("---\r\nstatus: \"frozen\"\r\n---\r\n")["status"], "frozen"
        )


class RequirementIndexCliTests(unittest.TestCase):
    """AC-008: one command generates, one command reports, missing file exits 2."""

    def test_cli_generate_and_status(self):
        with tempfile.TemporaryDirectory() as tmp:
            workspace = Path(tmp)
            alpha = workspace / "alpha"
            alpha.mkdir()
            _beacon_project(alpha, "one")
            beta = workspace / "beta"
            beta.mkdir()
            _beacon_project(beta, "two", status="draft")

            out = workspace / "out" / "requirement-index.json"
            db = workspace / "adb.sqlite3"
            # Hermetic kernel: a stub on PATH answers the lifecycle query, so the
            # test never runs the real `beacon-go`.
            bin_dir = _fake_kernel_bin(
                workspace / "bin",
                {
                    "features": [
                        {"slug": "one", "lifecycle_stage": "truth", "lifecycle_status": "Completed"}
                    ]
                },
            )
            env = _cli_env(ADB_REQUIREMENT_INDEX_NOW=STAMP)
            env["PATH"] = str(bin_dir) + os.pathsep + env.get("PATH", "")
            generate = CLI + [
                "--db",
                str(db),
                "index",
                "generate",
                "--projects",
                f"{alpha},{beta}",
                "--out",
                str(out),
                "--json",
            ]

            first = subprocess.run(
                generate, capture_output=True, text=True, env=env, cwd=str(workspace)
            )
            self.assertEqual(first.returncode, 0, first.stderr + first.stdout)
            payload = json.loads(first.stdout)
            self.assertEqual(payload["status"], "pass")
            self.assertEqual(payload["data"]["count"], 1)
            self.assertEqual(payload["data"]["format"], "full")
            self.assertEqual(payload["data"]["requirements"], ["req:alpha:one"])
            self.assertEqual(
                [entry["reason"] for entry in payload["data"]["skipped"]], ["not_frozen"]
            )
            self.assertTrue(out.is_file())
            document = json.loads(out.read_text(encoding="utf-8"))
            self.assertEqual(document["schema_version"], SCHEMA_VERSION)
            self.assertEqual(
                [record["req_id"] for record in document["requirements"]],
                ["req:alpha:one"],
            )
            self.assertEqual(document["requirements"][0]["truth_stage"], "truth")
            self.assertEqual(document["generated_at"], STAMP)

            # ADB_REQUIREMENT_INDEX_NOW pins the clock: two runs are byte equal.
            first_bytes = out.read_bytes()
            second = subprocess.run(
                generate, capture_output=True, text=True, env=env, cwd=str(workspace)
            )
            self.assertEqual(second.returncode, 0, second.stderr + second.stdout)
            self.assertEqual(out.read_bytes(), first_bytes)

            status = subprocess.run(
                CLI + ["--db", str(db), "index", "status", "--out", str(out), "--json"],
                capture_output=True,
                text=True,
                env=env,
                cwd=str(workspace),
            )
            self.assertEqual(status.returncode, 0, status.stderr + status.stdout)
            info = json.loads(status.stdout)["data"]
            self.assertEqual(info["count"], 1)
            self.assertEqual(info["format"], "full")
            self.assertEqual(info["missing_count"], 0)  # the stub answered every slug
            self.assertEqual(info["schema_version"], SCHEMA_VERSION)

            text_mode = subprocess.run(
                CLI + ["--db", str(db), "index", "status", "--out", str(out)],
                capture_output=True,
                text=True,
                env=env,
                cwd=str(workspace),
            )
            self.assertEqual(text_mode.returncode, 0, text_mode.stderr + text_mode.stdout)
            self.assertIn("count=1", text_mode.stdout)
            self.assertIn("missing=0", text_mode.stdout)
            self.assertIn("format=full", text_mode.stdout)

    def test_cli_status_missing_file_exits_2(self):
        with tempfile.TemporaryDirectory() as tmp:
            workspace = Path(tmp)
            missing = workspace / "nowhere" / "requirement-index.json"
            result = subprocess.run(
                CLI + [
                    "--db",
                    str(workspace / "adb.sqlite3"),
                    "index",
                    "status",
                    "--out",
                    str(missing),
                    "--json",
                ],
                capture_output=True,
                text=True,
                env=_cli_env(),
                cwd=str(workspace),
            )
            self.assertEqual(result.returncode, 2, result.stderr + result.stdout)
            payload = json.loads(result.stdout)
            self.assertTrue(payload["blocked"])
            self.assertEqual(payload["reason_code"], "index_file_missing")
            self.assertFalse(missing.exists())

            with self.assertRaises(DeliveryBusError) as ctx:
                describe_index(missing)
            self.assertEqual(ctx.exception.reason_code, "index_file_missing")


class RequirementIndexLifecycleSourceTests(unittest.TestCase):
    """AC-009: ``truth_stage`` comes from the kernel lifecycle store only."""

    def test_lifecycle_source_provider_sets_truth_stage(self):
        """TC-RI-009: injected provider wins; an empty answer is recorded."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "delta"
            root.mkdir()
            _beacon_project(root, "feat-a", evidence=True)

            asked: list[Path] = []

            def provider(path: Path) -> dict[str, dict[str, str]]:
                asked.append(path)
                return {"feat-a": {"stage": "qa", "status": "InProgress"}}

            sources, skipped = enumerate_sources([root], lifecycle_provider=provider)
            self.assertEqual(len(asked), 1)  # one kernel call per root
            self.assertEqual(asked[0].name, "delta")
            self.assertEqual(skipped, [])
            self.assertEqual(
                sources[0].payload["lifecycle"],
                {"stage": "qa", "status": "InProgress"},
            )

            document = build_index(sources, generated_at=STAMP).document
            record = document["requirements"][0]
            self.assertEqual(record["req_id"], "req:delta:feat-a")
            self.assertEqual(record["truth_stage"], "qa")
            self.assertEqual(record["source_status"], "ok")
            self.assertEqual(document["missing"], [])

            # Same package, a store that knows nothing: unknown + recorded why.
            sources, skipped = enumerate_sources([root], lifecycle_provider=_fake_lifecycle())
            document = build_index(sources, generated_at=STAMP).document
            record = document["requirements"][0]
            self.assertEqual(record["truth_stage"], "unknown")
            self.assertEqual(record["source_status"], "unknown")
            self.assertEqual(
                [entry["reason"] for entry in document["missing"]],
                [LIFECYCLE_SOURCE_MISSING],
            )
            self.assertEqual(
                [entry["reason"] for entry in skipped], [SKIP_STORE_NOT_REGISTERED]
            )
            self.assertEqual(skipped[0]["source"], str(root.resolve()))

    def test_lifecycle_source_broken_provider_never_blocks(self):
        """TC-RI-009: a provider that raises or answers garbage degrades to {}."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "zeta"
            root.mkdir()
            _beacon_project(root, "feat-z")

            def boom(path: Path) -> dict[str, dict[str, str]]:
                raise RuntimeError("kernel exploded")

            sources, skipped = enumerate_sources([root], lifecycle_provider=boom)
            self.assertEqual([source.req_id for source in sources], ["req:zeta:feat-z"])
            self.assertEqual(
                [entry["reason"] for entry in skipped], [SKIP_STORE_NOT_REGISTERED]
            )

            def wrong_shape(path: Path):
                return {"feat-z": "qa"}

            sources, _ = enumerate_sources([root], lifecycle_provider=wrong_shape)
            document = build_index(sources, generated_at=STAMP).document
            self.assertEqual(document["requirements"][0]["truth_stage"], "unknown")
            self.assertEqual(
                [entry["reason"] for entry in document["missing"]],
                [LIFECYCLE_SOURCE_MISSING],
            )

    def test_lifecycle_source_default_provider_fails_closed(self):
        """TC-RI-009: the kernel call is parsed strictly and never raises."""
        root = Path("/nonexistent/project")
        healthy = mock.Mock(
            returncode=0,
            stdout=json.dumps(
                {
                    "features": [
                        {
                            "slug": "feat-a",
                            "lifecycle_stage": "qa",
                            "lifecycle_status": "InProgress",
                        },
                        {"slug": "", "lifecycle_stage": "plan"},
                        "not-a-feature",
                    ]
                }
            ),
        )
        with mock.patch("subprocess.run", return_value=healthy) as run:
            self.assertEqual(
                default_lifecycle_provider(root),
                {"feat-a": {"stage": "qa", "status": "InProgress"}},
            )
            args, kwargs = run.call_args
            self.assertEqual(args[0][:3], ["beacon-go", "truth", "list"])
            self.assertIn("--project", args[0])
            self.assertIn(str(root), args[0])
            self.assertIn("--json", args[0])
            self.assertTrue(kwargs["capture_output"])
            self.assertTrue(kwargs["text"])
            self.assertEqual(kwargs["timeout"], 30)

        for broken in (
            mock.Mock(returncode=2, stdout=""),
            mock.Mock(returncode=0, stdout="not json"),
            mock.Mock(returncode=0, stdout=json.dumps([1, 2])),
            mock.Mock(returncode=0, stdout=json.dumps({"features": "nope"})),
        ):
            with mock.patch("subprocess.run", return_value=broken):
                self.assertEqual(default_lifecycle_provider(root), {})

        for failure in (
            FileNotFoundError("beacon-go"),
            subprocess.TimeoutExpired("beacon-go", 30),
            subprocess.CalledProcessError(1, "beacon-go"),
        ):
            with mock.patch("subprocess.run", side_effect=failure):
                self.assertEqual(default_lifecycle_provider(root), {})

    def test_lifecycle_not_guessed_from_frontmatter(self):
        """TC-RI-ILL-004: flipping ``status`` never moves ``truth_stage``."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "eta"
            root.mkdir()
            package = _beacon_project(root, "flip", status="draft")

            registered = _fake_lifecycle({"other": {"stage": "plan"}})
            sources, skipped = enumerate_sources([root], lifecycle_provider=registered)
            self.assertEqual(sources, [])
            self.assertEqual([entry["reason"] for entry in skipped], ["not_frozen"])
            before = build_index(sources, generated_at=STAMP).document
            self.assertEqual(before["requirements"], [])

            # Flip only the frontmatter status; the kernel still knows no stage.
            truth = package / "truth.md"
            truth.write_text(
                truth.read_text(encoding="utf-8").replace("status: draft", "status: frozen"),
                encoding="utf-8",
            )
            sources, _ = enumerate_sources([root], lifecycle_provider=registered)
            self.assertEqual([source.req_id for source in sources], ["req:eta:flip"])
            after = build_index(sources, generated_at=STAMP).document
            self.assertEqual(after["requirements"][0]["truth_stage"], "unknown")
            self.assertEqual(after["requirements"][0]["source_status"], "unknown")
            self.assertEqual(
                [entry["reason"] for entry in after["missing"]],
                [LIFECYCLE_SOURCE_MISSING],
            )

            # A stage the kernel does report is used verbatim — never the status.
            sources, _ = enumerate_sources(
                [root],
                lifecycle_provider=_fake_lifecycle({"flip": {"stage": "implement"}}),
            )
            document = build_index(sources, generated_at=STAMP).document
            self.assertEqual(document["requirements"][0]["truth_stage"], "implement")


if __name__ == "__main__":
    unittest.main()
