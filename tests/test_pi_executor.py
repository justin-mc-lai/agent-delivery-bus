from __future__ import annotations

import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from typing import Any

from agent_delivery_bus.adapters.beacon import BeaconAdapter
from agent_delivery_bus.adapters.factory import AdapterResolver, create_executor
from agent_delivery_bus.adapters.pi import PiExecutorAdapter, PiRunLedger
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.schedule import ScheduleService
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import make_project, write_registry


class FakeResult:
    def __init__(self, returncode: int = 0, stdout: str = "", stderr: str = ""):
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr


class FakeRunner:
    def __init__(self, result: FakeResult | None = None):
        self.calls: list[list[str]] = []
        self.result = result or FakeResult()

    def run(self, command: list[str], timeout: int = 30, cwd=None):  # noqa: ARG002
        self.calls.append(list(command))
        self.last_cwd = cwd
        return self.result


class AutoDonePi(PiExecutorAdapter):
    """Test-only pi adapter that marks runs done for reconcile smoke."""

    def create_task(self, project, *, stage, feature, body, idempotency_key, assignee="coding", skills=None, session_id="", wait=True):
        receipt = super().create_task(
            project,
            stage=stage,
            feature=feature,
            body=body,
            idempotency_key=idempotency_key,
            assignee=assignee,
            skills=skills,
            session_id=session_id,
            wait=wait,
        )
        receipt["status"] = "done"
        self.ledger.put(receipt["board"], idempotency_key, receipt)
        return receipt


class GoalGate:
    """Minimal truth gate whose goal closure checks the dispatch-bound manifest."""

    name = "goal-gate"

    def __init__(self, repo: Path):
        self.repo = repo

    def preflight_checks(self, project, *, stage):
        del project, stage
        return []

    def closure(self, project, *, stage, feature, dispatch_id="", evidence_spec=None):
        del evidence_spec
        if stage != "goal":
            return {"pass": True, "evidence": []}
        manifest = Path(project.repo) / ".beacon" / "state" / "goal" / feature / "manifest.json"
        if not manifest.is_file():
            return {"pass": False, "reason_code": "goal_manifest_missing", "evidence": []}
        try:
            payload = json.loads(manifest.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, OSError):
            payload = {}
        if str(payload.get("dispatch_id") or "") != str(dispatch_id):
            return {"pass": False, "reason_code": "evidence_ownership_mismatch", "evidence": []}
        return {"pass": True, "evidence": [str(manifest)]}


class PiExecutorContractTests(unittest.TestCase):
    def test_spi_surface(self):
        adapter = PiExecutorAdapter()
        for method in (
            "preflight_checks",
            "board_for",
            "workspace_for",
            "ensure_board",
            "create_task",
            "show_task",
            "find_by_idempotency",
            "skills_available",
        ):
            self.assertTrue(callable(getattr(adapter, method, None)), method)
        self.assertEqual(adapter.name, "pi")

    def test_cli_missing_fail_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            adapter = PiExecutorAdapter(which_command=lambda _name: None)
            checks = adapter.preflight_checks(project, stage="goal")
            self.assertTrue(any(c["name"] == "pi_cli" and not c["passed"] for c in checks))
            self.assertTrue(any(c["reason_code"] == "pi_cli_unavailable" for c in checks))

    def test_silent_fallback_forbidden(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config = write_registry(root / "projects.json", [make_project(root, slug="demo")])
            raw = json.loads(config.read_text(encoding="utf-8"))
            raw["adapters"] = {"executor": "no-such-executor", "truth_gate": "null"}
            config.write_text(json.dumps(raw), encoding="utf-8")
            registry = ProjectRegistry.load(config)
            resolver = AdapterResolver(registry.raw)
            with self.assertRaises(DeliveryBusError) as ctx:
                resolver.for_project(registry.list()[0])
            self.assertEqual(ctx.exception.reason_code, "executor_adapter_unknown")

    def test_create_task_body_and_receipt(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            runner = FakeRunner(FakeResult(stdout='{"sessionId":"sess-1"}'))
            adapter = PiExecutorAdapter(
                runner=runner,
                which_command=lambda _name: "/usr/local/bin/omp",
                ledger=PiRunLedger(root / "ledger"),
            )
            receipt = adapter.create_task(
                project,
                stage="goal",
                feature="feat-x",
                body="### Worker binding\n- binding_profile: beacon\n### Evidence spec\n- dispatch_id_binding: true",
                idempotency_key="key-1",
                wait=True,
            )
            self.assertTrue(receipt["task_id"].startswith("pi_"))
            self.assertEqual(receipt["board"], "adb-pi-demo")
            self.assertEqual(receipt["status"], "done")
            body_cmd = next(cmd for cmd in runner.calls if "-p" in cmd)
            self.assertIn("### Evidence spec", body_cmd[-1])
            self.assertEqual(adapter.find_by_idempotency("adb-pi-demo", "key-1")["task_id"], receipt["task_id"])

    def test_create_task_marks_failed_on_llm_timeout(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            runner = FakeRunner(FakeResult(stdout='{"type":"agent_end","message":{},"stopReason":"error","errorMessage":"Request timed out."}\n{"type":"agent_settled"}'))
            adapter = PiExecutorAdapter(
                runner=runner,
                which_command=lambda _name: "/usr/local/bin/pi",
                ledger=PiRunLedger(root / "ledger"),
            )
            receipt = adapter.create_task(project, stage="goal", feature="f", body="b", idempotency_key="k", wait=True)
            self.assertEqual(receipt["status"], "failed")

    def test_create_task_marks_done_on_successful_agent_settled(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            runner = FakeRunner(FakeResult(stdout='{"type":"message_end","message":{"stopReason":"stop"}}\n{"type":"agent_settled"}'))
            adapter = PiExecutorAdapter(
                runner=runner,
                which_command=lambda _name: "/usr/local/bin/pi",
                ledger=PiRunLedger(root / "ledger"),
            )
            receipt = adapter.create_task(project, stage="goal", feature="f", body="b", idempotency_key="k", wait=True)
            self.assertEqual(receipt["status"], "done")

    def test_create_task_idempotent(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            runner = FakeRunner()
            adapter = PiExecutorAdapter(
                runner=runner,
                which_command=lambda _name: "/usr/local/bin/omp",
                ledger=PiRunLedger(root / "ledger"),
            )
            first = adapter.create_task(project, stage="plan", feature="f", body="b", idempotency_key="k", wait=True)
            second = adapter.create_task(project, stage="plan", feature="f", body="b", idempotency_key="k", wait=True)
            self.assertEqual(first["task_id"], second["task_id"])
            self.assertEqual(sum(1 for c in runner.calls if "-p" in c), 1)

    def test_auto_approve_forbidden(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            adapter = PiExecutorAdapter(
                runner=FakeRunner(),
                which_command=lambda _name: "/usr/local/bin/omp",
                ledger=PiRunLedger(root / "ledger"),
            )
            with self.assertRaises(DeliveryBusError) as ctx:
                adapter.create_task(project, stage="implement", feature="f", body="--auto-approve", idempotency_key="k")
            self.assertEqual(ctx.exception.reason_code, "pi_auto_approve_forbidden")

    def test_goal_closure_manifest(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            gate = BeaconAdapter()
            goal_root = Path(project.repo) / ".beacon" / "state" / "goal" / "feat-x"
            self.assertFalse(gate.closure(project, stage="goal", feature="feat-x", dispatch_id="adb_1")["pass"])
            goal_root.mkdir(parents=True)
            (goal_root / "manifest.json").write_text(
                json.dumps({"dispatch_id": "adb_1", "stage": "goal", "feature": "feat-x"}),
                encoding="utf-8",
            )
            ok = gate.closure(project, stage="goal", feature="feat-x", dispatch_id="adb_1")
            self.assertTrue(ok["pass"])
            bad = gate.closure(project, stage="goal", feature="feat-x", dispatch_id="adb_2")
            self.assertFalse(bad["pass"])
            self.assertEqual(bad["reason_code"], "evidence_ownership_mismatch")

    def test_stage_executor_policy_routing(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            raw = json.loads(
                write_registry(root / "projects.json", [project]).read_text(encoding="utf-8")
            )
            raw["projects"][0]["metadata"] = {
                "executor_policy": {"stages": {"goal": "pi", "plan": "hermes"}}
            }
            registry = ProjectRegistry.load(write_registry(root / "projects2.json", [project]), validate_paths=False)
            registry.raw = raw
            registry.save()
            registry = ProjectRegistry.load(root / "projects2.json", validate_paths=False)
            resolver = AdapterResolver(registry.raw)
            goal_adapters = resolver.for_project(registry.list()[0], stage="goal")
            self.assertEqual(goal_adapters["executor_name"], "pi")
            plan_adapters = resolver.for_project(registry.list()[0], stage="plan")
            self.assertEqual(plan_adapters["executor_name"], "hermes")

    def test_smoke_goal_reconcile(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root, slug="demo")
            repo = Path(project.repo)
            subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
            config = write_registry(root / "projects.json", [project])
            registry = ProjectRegistry.load(config, validate_paths=False)
            ledger = PiRunLedger(root / "ledger")
            pi = AutoDonePi(
                runner=FakeRunner(FakeResult(stdout='{"sessionId":"s-1"}')),
                which_command=lambda _name: "/usr/local/bin/omp",
                ledger=ledger,
            )
            gate = GoalGate(repo)
            storage = Storage(":memory:")
            service = DeliveryService(
                registry,
                storage,
                executor=pi,
                truth_gate=gate,
                adapter_resolver=lambda p, stage="": {
                    "executor": pi,
                    "truth_gate": gate,
                    "binding_profile": "beacon",
                    "executor_name": "pi",
                    "truth_gate_name": "goal-gate",
                },
            )
            result = service.dispatch(project_slug="demo", stage="goal", feature="long-run-x", dry_run=True)
            self.assertEqual(result["status"], "pass")
            dispatched = service.dispatch(project_slug="demo", stage="goal", feature="long-run-x")
            self.assertEqual(dispatched["status"], "dispatched")
            dispatch_id = dispatched["dispatch"]["dispatch_id"]
            manifest_dir = repo / ".beacon" / "state" / "goal" / "long-run-x"
            manifest_dir.mkdir(parents=True)
            (manifest_dir / "manifest.json").write_text(
                json.dumps({"dispatch_id": dispatch_id, "stage": "goal", "feature": "long-run-x"}),
                encoding="utf-8",
            )
            reconciled = service.reconcile(dispatch_id)
            self.assertEqual(reconciled["status"], "completed")

    def test_heartbeat_auto_dispatch_forbidden(self):
        storage = Storage(":memory:")
        svc = ScheduleService(storage)
        blocked = svc.reject_illegal(action="dispatch")
        self.assertEqual(blocked["reason_code"], "illegal_heartbeat_action")
        blocked2 = svc.reject_illegal(action="approve")
        self.assertEqual(blocked2["reason_code"], "illegal_heartbeat_action")


if __name__ == "__main__":
    unittest.main()
