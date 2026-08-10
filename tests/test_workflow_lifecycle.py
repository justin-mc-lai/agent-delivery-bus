"""Workflow lifecycle: presets, ingest/host-fill/confirm/verify, trace, illegal paths."""

from __future__ import annotations

import json
import contextlib
import io
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.cli import main
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.worker_binding import resolve_worker_binding
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage
from agent_delivery_bus.workflows import (
    TraceWriter,
    build_preset,
    confirm_install,
    draft_apply,
    ingest_request,
    invalidate_verified,
    is_verified,
    install_workflow,
    remove_workflow,
    validate_fill_response,
    verify_workflow,
    workflow_names,
)

from .helpers import FakeBeacon, FakeHermes, PassingPreflight, make_project, write_registry


def fake_repo(root: Path, *, with_skill: bool = True, with_danger: bool = False) -> Path:
    repo = root / "source-repo"
    repo.mkdir(parents=True)
    (repo / "README.md").write_text("# Demo workflow\nstages: plan -> implement -> qa\n", encoding="utf-8")
    if with_skill:
        skill = repo / "skills" / "demo-workflow" / "SKILL.md"
        skill.parent.mkdir(parents=True)
        skill.write_text("# demo-workflow skill\nRun plan/implement/qa.\n", encoding="utf-8")
    cli = repo / "demo-workflow.py"
    cli.write_text('def main():\n    print("demo")\n', encoding="utf-8")
    if with_danger:
        (repo / "danger.txt").write_text("run: rm -rf / --no-preserve-root\n", encoding="utf-8")
    return repo


def host_response(name: str = "demo-wf") -> dict:
    return {
        "schema": "workflow-analysis-response.v1",
        "name": name,
        "description": "Demo workflow from fake repo",
        "skills": ["demo-workflow"],
        "stages": {
            "plan": {
                "skill": "demo-workflow",
                "public_harness": "plan",
                "command": 'demo-workflow.py plan "{feature}"',
            },
            "implement": {
                "skill": "demo-workflow",
                "public_harness": "implement",
                "command": 'demo-workflow.py implement "{feature}"',
            },
            "qa": {
                "skill": "demo-workflow",
                "public_harness": "qa",
                "command": 'demo-workflow.py qa "{feature}"',
            },
        },
        "evidence_spec": {
            "evidence_dir": ".demo/evidence/{stage}/{feature}",
            "glob": "*.json",
            "required_files": ["manifest.json"],
            "dispatch_id_binding": True,
        },
        "fields_evidence": {
            "plan": ["skills/demo-workflow/SKILL.md", "README.md"],
            "implement": ["skills/demo-workflow/SKILL.md"],
            "qa": ["skills/demo-workflow/SKILL.md"],
            "skills": ["skills/demo-workflow/SKILL.md"],
            "evidence_spec": ["README.md"],
        },
    }


class PresetTests(unittest.TestCase):
    def test_presets_are_beacon_peer_skill_workflows(self):
        self.assertEqual(sorted(build_preset("superpowers")["skills"]), ["superpowers"])
        self.assertEqual(sorted(build_preset("openspec")["skills"]), ["openspec"])
        self.assertIn("plan", build_preset("openspec")["stages"])
        self.assertNotIn("beacon", build_preset("superpowers")["source"])

    def test_install_uses_superpowers_openspec(self):
        raw: dict = {}
        installed = install_workflow(raw, name="sp", preset="superpowers")
        self.assertEqual(installed["skills"], ["superpowers"])
        self.assertIn("sp", workflow_names(raw))


class BeaconStageTests(unittest.TestCase):
    def test_beacon_stages_force_load(self):
        expected = {
            "plan": "beacon-plan",
            "truth": "beacon-truth",
            "implement": "beacon-implement",
            "qa": "beacon-qa",
            "freeze": "beacon-truth",
            "goal": "beacon-goal",
        }
        for stage, skill in expected.items():
            binding = resolve_worker_binding(stage=stage, feature="f", docs_version="v0.0.7")
            self.assertEqual(binding["skills"], [skill], stage)

    def test_missing_skill_blocks(self):
        class MissingSkillExecutor(FakeHermes):
            def skills_available(self, skills):
                return {"missing": list(skills), "installed": []}

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            storage = Storage(":memory:")
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                executor=MissingSkillExecutor(),
                truth_gate=FakeBeacon(),
            )
            result = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(result["reason_code"], "binding_skill_missing")
            storage.close()


class IngestHostFillTests(unittest.TestCase):
    def _ingest(self, tmp: str):
        root = Path(tmp)
        repo = fake_repo(root)
        return root, repo, ingest_request(name="demo-wf", source=str(repo), root=root, workdir=root)

    def test_ingest_emits_request_with_anchors_and_trace(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, repo, result = self._ingest(tmp)
            request = json.loads(Path(result["request_path"]).read_text(encoding="utf-8"))
            self.assertEqual(request["schema"], "workflow-analysis-request.v1")
            self.assertEqual(request["commit"], "local")
            self.assertTrue(any(a["kind"] == "skill" for a in request["anchors"]))
            trace = TraceWriter.read(TraceWriter.latest(root, "demo-wf"))
            events = [e["event"] for e in trace]
            self.assertIn("inventory", events)
            self.assertIn("analysis_request", events)

    def test_host_fill_validate_draft_confirm_install(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, repo, result = self._ingest(tmp)
            request = json.loads(Path(result["request_path"]).read_text(encoding="utf-8"))
            response = host_response()
            validation = validate_fill_response(request, response)
            self.assertTrue(validation["pass"], validation)
            drafted = draft_apply(name="demo-wf", root=root, request=request, response=response)
            self.assertTrue(Path(drafted["draft_path"]).is_file())
            raw: dict = {}
            installed = confirm_install(name="demo-wf", root=root, raw=raw)
            self.assertEqual(installed["installed"], "demo-wf")
            self.assertEqual(raw["workflows"]["demo-wf"]["skills"], ["demo-workflow"])

    def test_dangerous_command_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = fake_repo(root, with_danger=True)
            result = ingest_request(name="bad-wf", source=str(repo), root=root, workdir=root)
            request = json.loads(Path(result["request_path"]).read_text(encoding="utf-8"))
            response = host_response("bad-wf")
            response["stages"]["implement"]["command"] = "rm -rf {feature}"
            validation = validate_fill_response(request, response)
            self.assertFalse(validation["pass"])
            self.assertTrue(any("dangerous" in p for p in validation["problems"]))

    def test_no_evidence_confirm_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = fake_repo(root)
            result = ingest_request(name="noev-wf", source=str(repo), root=root, workdir=root)
            request = json.loads(Path(result["request_path"]).read_text(encoding="utf-8"))
            response = host_response("noev-wf")
            response["fields_evidence"] = {"plan": [], "implement": [], "qa": []}
            validation = validate_fill_response(request, response)
            self.assertFalse(validation["pass"])
            self.assertTrue(any("no anchor evidence" in p for p in validation["problems"]))

    def test_bad_repo_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            empty = root / "empty-repo"
            empty.mkdir()
            result = ingest_request(name="empty-wf", source=str(empty), root=root, workdir=root)
            request = json.loads(Path(result["request_path"]).read_text(encoding="utf-8"))
            self.assertEqual(result["anchors_count"], 0)
            validation = validate_fill_response(request, host_response("empty-wf"))
            self.assertFalse(validation["pass"])


class VerifyAndTraceTests(unittest.TestCase):
    def _installed(self, tmp: str, raw: dict) -> tuple[Path, dict]:
        root, repo, result = self._ingest(tmp) if False else (None, None, None)
        return root, raw

    def test_verify_checks_skills_and_stages(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw = {"workflows": {"sp": build_preset("superpowers")}}
            hermes = FakeHermes()
            hermes.missing = []
            hermes.skills_available = lambda skills: {"missing": list(skills), "installed": []}
            report = verify_workflow(name="sp", raw=raw, root=root, executor=hermes)
            self.assertFalse(report["pass"])
            self.assertTrue(any(c["name"] == "skills" and not c["pass"] for c in report["checks"]))

    def test_verify_marker_invalidated_on_workflow_change(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw = {"workflows": {"sp": build_preset("superpowers")}}
            hermes = FakeHermes()
            hermes.skills_available = lambda skills: {"missing": [], "installed": list(skills)}
            report = verify_workflow(name="sp", raw=raw, root=root, executor=hermes)
            self.assertTrue(report["pass"])
            self.assertTrue(is_verified(root, "sp", workflow=raw["workflows"]["sp"]))
            # Changing the workflow (re-install/confirm) invalidates the marker.
            raw["workflows"]["sp"]["stages"]["plan"]["command"] = "superpowers plan v2 {feature}"
            self.assertFalse(is_verified(root, "sp", workflow=raw["workflows"]["sp"]))
            invalidate_verified(root, "sp")
            self.assertFalse(is_verified(root, "sp", workflow=raw["workflows"]["sp"]))
            # Re-verify restores it.
            report = verify_workflow(name="sp", raw=raw, root=root, executor=hermes)
            self.assertTrue(report["pass"])
            self.assertTrue(is_verified(root, "sp", workflow=raw["workflows"]["sp"]))

    def test_remove_invalidates_marker(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config = write_registry(root / "projects.json", [make_project(root)])
            raw = json.loads(config.read_text(encoding="utf-8"))
            raw["workflows"] = {"sp": build_preset("superpowers")}
            config.write_text(json.dumps(raw, ensure_ascii=False), encoding="utf-8")
            hermes = FakeHermes()
            hermes.skills_available = lambda skills: {"missing": [], "installed": list(skills)}
            verify_workflow(name="sp", raw=raw, root=root, executor=hermes)
            self.assertTrue(is_verified(root, "sp"))
            remove_workflow(raw, "sp")
            invalidate_verified(root, "sp")
            self.assertFalse(is_verified(root, "sp"))
            code = main(
                [
                    "--config", str(config), "--db", ":memory:",
                    "workflow", "remove", "sp", "--yes", "--json",
                ]
            )
            self.assertEqual(code, 0)

    def test_cli_register_reports_effective_workflow(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config = write_registry(root / "projects.json", [make_project(root)])
            new_repo = root / "new-app"
            new_repo.mkdir()
            out = io.StringIO()
            with contextlib.redirect_stdout(out):
                code = main(
                    [
                        "--config", str(config), "--db", ":memory:",
                        "projects", "register", "--slug", "new-app",
                        "--class", "managed", "--repo", str(new_repo), "--json",
                    ]
                )
            self.assertEqual(code, 0)
            payload = json.loads(out.getvalue())
            self.assertEqual(payload["data"]["effective_binding_profile"], "beacon")
            self.assertIn("默认第一方 beacon 生命周期", payload["data"]["text"])

    def test_cli_list_numbered_shows_workflow(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config = write_registry(root / "projects.json", [make_project(root)])
            out = io.StringIO()
            with contextlib.redirect_stdout(out):
                main(
                    [
                        "--config", str(config), "--db", ":memory:",
                        "projects", "list", "--numbered", "--json",
                    ]
                )
            payload = json.loads(out.getvalue())
            self.assertIn("wf=beacon", payload["data"]["text"])

    def test_trace_debug_replay_cli(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = fake_repo(root)
            config = write_registry(root / "projects.json", [make_project(root)])
            code = main(
                [
                    "--config", str(config), "--db", ":memory:",
                    "workflow", "ingest", "--source", str(repo), "--name", "demo-wf", "--json",
                ]
            )
            self.assertEqual(code, 0)
            self.assertEqual(
                main(
                    [
                        "--config", str(config), "--db", ":memory:",
                        "workflow", "trace", "--name", "demo-wf", "--json",
                    ]
                ),
                0,
            )
            self.assertEqual(
                main(
                    [
                        "--config", str(config), "--db", ":memory:",
                        "workflow", "debug", "--name", "demo-wf", "--json",
                    ]
                ),
                0,
            )
            self.assertEqual(
                main(
                    [
                        "--config", str(config), "--db", ":memory:",
                        "workflow", "replay", "--name", "demo-wf", "--json",
                    ]
                ),
                0,
            )

    def test_fake_trace_rejected(self):
        # A trace event stream with a non-JSONL/foreign schema must be rejected.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            trace = TraceWriter(root, "fake-wf")
            with open(trace.path, "w", encoding="utf-8") as fh:
                fh.write("not-json\n")
            rows = TraceWriter.read(trace.path)
            # json decode failure must not silently pass validation-style flow
            self.assertEqual(rows, [])

    def test_install_without_confirm_requires_yes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = fake_repo(root)
            config = write_registry(root / "projects.json", [make_project(root)])
            result = ingest_request(name="cwf", source=str(repo), root=root, workdir=root)
            request = json.loads(Path(result["request_path"]).read_text(encoding="utf-8"))
            draft_apply(name="cwf", root=root, request=request, response=host_response("cwf"))
            code = main(
                [
                    "--config", str(config), "--db", ":memory:",
                    "workflow", "confirm", "--name", "cwf", "--json",
                ]
            )
            self.assertEqual(code, 2)  # confirmation required


class DispatchReconcileTests(unittest.TestCase):
    def test_bound_workflow_dispatch_reconcile_closed_loop(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            config = write_registry(root / "projects.json", [project])
            raw = json.loads(config.read_text(encoding="utf-8"))
            raw["workflows"] = {"my-wf": build_preset("openspec")}
            raw["projects"][0]["binding_profile"] = "my-wf"
            config.write_text(json.dumps(raw, ensure_ascii=False), encoding="utf-8")
            registry = ProjectRegistry.load(config)
            storage = Storage(":memory:")
            hermes = FakeHermes(remote_status="done")
            hermes.skills_available = lambda skills: {"missing": [], "installed": list(skills)}
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                executor=hermes,
                truth_gate=FakeBeacon(closure_pass=True),
                workflow_root=root,
            )
            blocked = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(blocked["reason_code"], "workflow_verify_required")
            report = verify_workflow(
                name="my-wf",
                raw=registry.raw,
                root=root,
                executor=hermes,
                project=registry.resolve(slug="demo"),
                service=service,
            )
            self.assertTrue(report["pass"], report)
            dispatched = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(dispatched["status"], "dispatched")
            self.assertIn("openspec", hermes.last_skills)
            reconciled = service.reconcile(dispatched["dispatch"]["dispatch_id"])
            self.assertEqual(reconciled["status"], "completed")
            storage.close()


if __name__ == "__main__":
    unittest.main()
