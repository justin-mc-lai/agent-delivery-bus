import json
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.adapters.content import ContentTruthGate
from agent_delivery_bus.adapters.factory import create_truth_gate
from agent_delivery_bus.registry import Project


def make_project(root: Path) -> Project:
    return Project(
        slug="content-creator",
        title="content-creator",
        project_class="managed",
        repo=str(root),
        aliases=("creator",),
        dispatchable=True,
        docs_root=str(root / "docs" / "beacon"),
        docs_version="v0.0.5",
        truth_gate="content",
        executor="hermes",
        binding_profile="selfmedia-codex",
    )


def write_manifest(evidence_dir: Path, dispatch_id: str) -> None:
    evidence_dir.mkdir(parents=True, exist_ok=True)
    (evidence_dir / "manifest.json").write_text(
        json.dumps({"dispatch_id": dispatch_id, "files": []}, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )


class ContentTruthGateTests(unittest.TestCase):
    def test_plan_closure_passes_with_master_package(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            master = root / "content" / "masters" / "oss-picks" / "anydoc"
            master.mkdir(parents=True)
            for name in ("treatment.md", "MASTER.md", "meta.yaml"):
                (master / name).write_text("x\n", encoding="utf-8")
            evidence_dir = root / ".beacon" / "evidence" / "plan" / "anydoc"
            write_manifest(evidence_dir, "d-1")
            spec = {"evidence_dir": str(evidence_dir)}
            result = ContentTruthGate().closure(
                make_project(root), stage="plan", feature="anydoc", dispatch_id="d-1", evidence_spec=spec
            )
            self.assertTrue(result["pass"])

    def test_plan_closure_rejects_missing_master(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            evidence_dir = root / ".beacon" / "evidence" / "plan" / "anydoc"
            write_manifest(evidence_dir, "d-1")
            spec = {"evidence_dir": str(evidence_dir)}
            result = ContentTruthGate().closure(
                make_project(root), stage="plan", feature="anydoc", dispatch_id="d-1", evidence_spec=spec
            )
            self.assertFalse(result["pass"])
            self.assertEqual(result["reason_code"], "content_evidence_incomplete")

    def test_implement_closure_requires_presentation_render_and_qa(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pres = root / "content" / "presentations" / "oss-picks" / "anydoc" / "wechat-gzh-image-post-v1"
            for rel in (
                "presentation.yaml",
                "caption-short.md",
                "shot-list.md",
                "assets/manifest.yaml",
                "qa/director-qc.md",
                "qa/anti-slop.md",
                "qa/value-gate.md",
                "qa/visual-qa.md",
            ):
                path = pres / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("x\n", encoding="utf-8")
            renders = root / "content" / "renders" / "demo-gzh" / "wechat-gzh" / "assets" / "anydoc-v1"
            renders.mkdir(parents=True)
            (renders / "01-cover.jpg").write_bytes(b"jpeg")
            (renders / "02-article.png").write_bytes(b"png")
            evidence_dir = root / ".beacon" / "evidence" / "implement" / "anydoc"
            write_manifest(evidence_dir, "d-2")
            spec = {"evidence_dir": str(evidence_dir)}
            result = ContentTruthGate().closure(
                make_project(root), stage="implement", feature="anydoc", dispatch_id="d-2", evidence_spec=spec
            )
            self.assertTrue(result["pass"])

    def test_qa_closure_requires_uploadable_conclusion(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            qa = root / "content" / "presentations" / "oss-picks" / "anydoc" / "wechat-gzh-image-post-v1" / "qa"
            qa.mkdir(parents=True)
            for name in ("visual-qa.md", "anti-slop.md", "value-gate.md", "release-approval.md"):
                (qa / name).write_text("x\n", encoding="utf-8")
            (qa / "supervisor-review.md").write_text("结论：可上传草稿\n", encoding="utf-8")
            evidence_dir = root / ".beacon" / "evidence" / "qa" / "anydoc"
            write_manifest(evidence_dir, "d-3")
            spec = {"evidence_dir": str(evidence_dir)}
            result = ContentTruthGate().closure(
                make_project(root), stage="qa", feature="anydoc", dispatch_id="d-3", evidence_spec=spec
            )
            self.assertTrue(result["pass"])

            (qa / "supervisor-review.md").write_text("结论：要求返工\n", encoding="utf-8")
            result = ContentTruthGate().closure(
                make_project(root), stage="qa", feature="anydoc", dispatch_id="d-3", evidence_spec=spec
            )
            self.assertFalse(result["pass"])

    def test_manifest_ownership_mismatch_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            master = root / "content" / "masters" / "oss-picks" / "anydoc"
            master.mkdir(parents=True)
            for name in ("treatment.md", "MASTER.md", "meta.yaml"):
                (master / name).write_text("x\n", encoding="utf-8")
            evidence_dir = root / ".beacon" / "evidence" / "plan" / "anydoc"
            write_manifest(evidence_dir, "other-dispatch")
            spec = {"evidence_dir": str(evidence_dir)}
            result = ContentTruthGate().closure(
                make_project(root), stage="plan", feature="anydoc", dispatch_id="d-1", evidence_spec=spec
            )
            self.assertFalse(result["pass"])
            self.assertIn("dispatch_id mismatch", " ".join(result.get("problems", [])))

    def test_factory_registers_content_gate(self):
        gate = create_truth_gate("content")
        self.assertIsInstance(gate, ContentTruthGate)


if __name__ == "__main__":
    unittest.main()
