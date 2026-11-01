"""AC-001..AC-006: requirement-index projection (beacon truth + ADB exec state)."""

from __future__ import annotations

import hashlib
import json
import os
import shutil
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.requirement_index import (
    BUDGET_BYTES,
    EXEC_STATES,
    RECORD_FIELDS,
    SCHEMA_VERSION,
    TRUTH_STAGES,
    RequirementSource,
    build_index,
    choose_format,
    dumps,
    generate_index,
    req_id_for,
    set_exec_state,
    shard_path_for,
    write_index,
)
from agent_delivery_bus.storage import Storage

STAMP = "2026-11-01T07:41:10+00:00"


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
            self.assertEqual(document["missing"][0]["reason"], "lifecycle_missing")


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


if __name__ == "__main__":
    unittest.main()
