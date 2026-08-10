"""Content-pipeline truth gate for article/selfmedia style projects.

Beacon's closure semantics model software-delivery governance (truth.md /
revision packages). Content projects deliver article packages instead, so the
gate validates the real stage artifacts:

- plan      -> treatment.md + MASTER.md + meta.yaml
- implement -> presentation package + renders + core QA files
- qa        -> supervisor conclusion 可上传草稿 + release-approval
- freeze    -> evidence manifest with matching dispatch_id

Every stage also requires the evidence manifest declared by the binding
profile, so ownership stays tied to the dispatch id.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from ..registry import Project


class ContentTruthGate:
    """Truth gate adapter for article/content delivery packages."""

    name = "content"

    def preflight_checks(self, project: Project, *, stage: str) -> list[dict[str, Any]]:
        del project, stage
        # Repo/git are covered by the core preflight; Hermes availability is
        # covered by the executor adapter. Content truth has no extra
        # environment requirements.
        return []

    def closure(
        self,
        project: Project,
        *,
        stage: str,
        feature: str,
        dispatch_id: str = "",
        evidence_spec: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        root = Path(project.repo)
        feature = (feature or "").strip().lstrip("/")
        stage = (stage or "").strip().lower()
        layout = project.metadata.get("content_layout")
        layout = layout if isinstance(layout, dict) else {}
        vertical = str(layout.get("vertical") or "default")
        account = str(layout.get("account") or "default-account")
        presentation_template = str(layout.get("presentation_template") or "default-image-post-v1")
        account_template = str(layout.get("account_template") or "default")
        master_dir = root / "content" / "masters" / vertical / feature
        pres_dir = root / "content" / "presentations" / vertical / feature / presentation_template
        qa_dir = pres_dir / "qa"
        render_dir = (
            root / "content" / "renders" / account / account_template / "assets" / f"{feature}-v1"
        )

        evidence_dir = self._evidence_dir(project, stage, feature, evidence_spec)
        manifest_ok, manifest_path = self._manifest_ok(evidence_dir, dispatch_id)

        evidence: list[str] = []
        problems: list[str] = []

        if stage == "plan":
            required = [
                master_dir / "treatment.md",
                master_dir / "MASTER.md",
                master_dir / "meta.yaml",
            ]
            for path in required:
                evidence.append(str(path))
                if not path.is_file():
                    problems.append(f"missing {path.relative_to(root)}")
        elif stage == "implement":
            required_files = [
                pres_dir / "presentation.yaml",
                pres_dir / "caption-short.md",
                pres_dir / "shot-list.md",
                pres_dir / "assets" / "manifest.yaml",
                qa_dir / "director-qc.md",
                qa_dir / "anti-slop.md",
                qa_dir / "value-gate.md",
                qa_dir / "visual-qa.md",
            ]
            for path in required_files:
                evidence.append(str(path))
                if not path.is_file():
                    problems.append(f"missing {path.relative_to(root)}")
            cover = render_dir / "01-cover.jpg"
            evidence.append(str(cover))
            if not cover.is_file():
                problems.append(f"missing render cover {cover.relative_to(root)}")
            pages = sorted(render_dir.glob("*-article.png"))
            evidence.extend(str(path) for path in pages)
            if not pages:
                problems.append(f"missing rendered article pages under {render_dir.relative_to(root)}")
        elif stage == "qa":
            required_files = [
                qa_dir / "supervisor-review.md",
                qa_dir / "release-approval.md",
                qa_dir / "visual-qa.md",
                qa_dir / "anti-slop.md",
                qa_dir / "value-gate.md",
            ]
            for path in required_files:
                evidence.append(str(path))
                if not path.is_file():
                    problems.append(f"missing {path.relative_to(root)}")
            supervisor = qa_dir / "supervisor-review.md"
            if supervisor.is_file():
                text = supervisor.read_text(encoding="utf-8")
                if "可上传草稿" not in text:
                    problems.append("supervisor conclusion is not 可上传草稿")
        elif stage == "freeze":
            meta = master_dir / "meta.yaml"
            evidence.append(str(meta))
            if not meta.is_file():
                problems.append(f"missing {meta.relative_to(root)}")
            elif "current_snapshot" not in meta.read_text(encoding="utf-8"):
                problems.append("meta.yaml has no current_snapshot")
        else:
            return {
                "pass": False,
                "reason_code": "stage_not_enabled",
                "evidence": [],
                "resume_action": "use plan/implement/qa/freeze stages",
            }

        evidence.append(str(manifest_path) if manifest_path else str(evidence_dir))
        if not manifest_ok:
            problems.append(
                f"evidence manifest missing or dispatch_id mismatch in {evidence_dir}"
            )

        if problems:
            return {
                "pass": False,
                "reason_code": "content_evidence_incomplete",
                "evidence": evidence,
                "problems": problems,
                "resume_action": (
                    "run the stage worker to produce the missing artifacts and write "
                    f"manifest.json with dispatch_id={dispatch_id} into {evidence_dir}"
                ),
            }
        return {"pass": True, "evidence": evidence}

    @staticmethod
    def _evidence_dir(
        project: Project,
        stage: str,
        feature: str,
        evidence_spec: dict[str, Any] | None,
    ) -> Path:
        root = Path(project.repo)
        declared = (evidence_spec or {}).get("evidence_dir") or ""
        if declared:
            candidate = Path(declared).expanduser()
            return candidate if candidate.is_absolute() else root / candidate
        return root / ".beacon" / "evidence" / stage / feature

    @staticmethod
    def _manifest_ok(evidence_dir: Path, dispatch_id: str) -> tuple[bool, Path | None]:
        manifest = evidence_dir / "manifest.json"
        if not manifest.is_file():
            return False, manifest
        try:
            payload = json.loads(manifest.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, OSError):
            return False, manifest
        return str(payload.get("dispatch_id") or "") == str(dispatch_id), manifest
