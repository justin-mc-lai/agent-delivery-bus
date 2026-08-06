"""Evidence ownership tests (AC-NS-004 / AC-NS-008)."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.beacon import BeaconAdapter
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import FakeHermes, PassingPreflight, make_project, write_registry


class ReconcileEvidenceOwnershipTests(unittest.TestCase):
    def _service(self, tmp: str):
        root = Path(tmp)
        project = make_project(root)
        registry = ProjectRegistry.load(write_registry(root / "projects.json", [project]))
        storage = Storage(":memory:")
        service = DeliveryService(
            registry,
            storage,
            preflight=PassingPreflight(),
            executor=FakeHermes(remote_status="done"),
            truth_gate=BeaconAdapter(),
        )
        return root, project, service, storage, service.executor

    def test_missing_evidence_keeps_reconciling(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, project, service, storage, hermes = self._service(tmp)
            issued = service.approvals.issue(
                actor="apple",
                project_slug="demo",
                stage="implement",
                feature="feature",
                ttl_seconds=300,
            )
            dispatched = service.dispatch(
                project_slug="demo",
                stage="implement",
                feature="feature",
                approval_token=issued["token"],
            )
            dispatch_id = dispatched["dispatch"]["dispatch_id"]
            self.assertIn("### Evidence spec", hermes.last_body)
            self.assertIn("dispatch_id:", hermes.last_body)
            first = service.reconcile(dispatch_id)
            self.assertEqual(first["status"], "reconciling")
            self.assertEqual(first["reason_code"], "truth_evidence_incomplete")
            storage.close()

    def test_stale_evidence_without_manifest_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, project, service, storage, hermes = self._service(tmp)
            issued = service.approvals.issue(
                actor="apple",
                project_slug="demo",
                stage="implement",
                feature="feature",
                ttl_seconds=300,
            )
            dispatched = service.dispatch(
                project_slug="demo",
                stage="implement",
                feature="feature",
                approval_token=issued["token"],
            )
            dispatch_id = dispatched["dispatch"]["dispatch_id"]
            evidence_dir = Path(project.repo) / ".beacon" / "evidence" / "implement" / "feature"
            evidence_dir.mkdir(parents=True)
            (evidence_dir / "result.json").write_text(json.dumps({"pass": True}), encoding="utf-8")
            first = service.reconcile(dispatch_id)
            self.assertEqual(first["status"], "reconciling")
            self.assertEqual(first["reason_code"], "truth_evidence_incomplete")
            self.assertEqual(first["closure"]["reason_code"], "evidence_ownership_mismatch")
            storage.close()

    def test_dispatch_id_mismatch_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, project, service, storage, hermes = self._service(tmp)
            issued = service.approvals.issue(
                actor="apple",
                project_slug="demo",
                stage="implement",
                feature="feature",
                ttl_seconds=300,
            )
            dispatched = service.dispatch(
                project_slug="demo",
                stage="implement",
                feature="feature",
                approval_token=issued["token"],
            )
            dispatch_id = dispatched["dispatch"]["dispatch_id"]
            evidence_dir = Path(project.repo) / ".beacon" / "evidence" / "implement" / "feature"
            evidence_dir.mkdir(parents=True)
            (evidence_dir / "result.json").write_text(json.dumps({"pass": True}), encoding="utf-8")
            (evidence_dir / "manifest.json").write_text(
                json.dumps({"dispatch_id": "apr_other", "files": ["result.json"]}),
                encoding="utf-8",
            )
            first = service.reconcile(dispatch_id)
            self.assertEqual(first["status"], "reconciling")
            self.assertEqual(first["reason_code"], "truth_evidence_incomplete")
            self.assertEqual(first["closure"]["reason_code"], "evidence_ownership_mismatch")
            storage.close()

    def test_matching_manifest_completes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, project, service, storage, hermes = self._service(tmp)
            issued = service.approvals.issue(
                actor="apple",
                project_slug="demo",
                stage="implement",
                feature="feature",
                ttl_seconds=300,
            )
            dispatched = service.dispatch(
                project_slug="demo",
                stage="implement",
                feature="feature",
                approval_token=issued["token"],
            )
            dispatch_id = dispatched["dispatch"]["dispatch_id"]
            evidence_dir = Path(project.repo) / ".beacon" / "evidence" / "implement" / "feature"
            evidence_dir.mkdir(parents=True)
            (evidence_dir / "result.json").write_text(json.dumps({"pass": True}), encoding="utf-8")
            (evidence_dir / "manifest.json").write_text(
                json.dumps({"dispatch_id": dispatch_id, "files": ["result.json"]}),
                encoding="utf-8",
            )
            first = service.reconcile(dispatch_id)
            self.assertEqual(first["status"], "completed")
            self.assertFalse(first["blocked"])
            storage.close()


if __name__ == "__main__":
    unittest.main()
