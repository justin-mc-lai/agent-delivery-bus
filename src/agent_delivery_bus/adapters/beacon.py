"""Example TruthGateAdapter backed by the Beacon CLI and local evidence files.

This is an optional reference integration, not part of the scheduling core.
"""

from __future__ import annotations

import json
from pathlib import Path
from shutil import which
from typing import Any

from ..process import CommandRunner
from ..registry import Project
from .spi import as_check


class BeaconAdapter:
    name = "beacon"

    def __init__(self, runner: CommandRunner | None = None, which_command=None):
        self.runner = runner or CommandRunner()
        self.which_command = which_command or which

    def preflight_checks(self, project: Project, *, stage: str) -> list[dict[str, Any]]:
        del stage  # Beacon readiness is project-scoped in the example adapter.
        checks: list[dict[str, Any]] = []
        docs_root = Path(project.docs_root) if project.docs_root else None
        docs_ok = bool(docs_root and docs_root.is_dir())
        checks.append(
            as_check(
                "truth_docs_root",
                docs_ok,
                reason_code="truth_docs_missing",
                resume_action="restore truth docs or re-register the project docs_root",
            )
        )
        version_ok = bool(docs_root and project.docs_version and (docs_root / project.docs_version).is_dir())
        checks.append(
            as_check(
                "truth_docs_version",
                version_ok,
                reason_code="truth_version_mismatch",
                resume_action=f"confirm docs_version={project.docs_version} in the project registry",
            )
        )
        cli_ok = bool(self.which_command("beacon"))
        checks.append(
            as_check(
                "beacon_cli",
                cli_ok,
                reason_code="beacon_cli_unavailable",
                resume_action="install or repair the Beacon CLI, then rerun preflight",
            )
        )
        context = (
            self.verify_context(project)
            if Path(project.repo).is_dir() and cli_ok
            else {"pass": False, "payload": {}}
        )
        actual_docs_version = str(((context.get("payload") or {}).get("docs_version") or ""))
        declared_ok = not actual_docs_version or actual_docs_version == project.docs_version
        checks.append(
            as_check(
                "truth_declared_version",
                declared_ok,
                reason_code="truth_version_mismatch",
                resume_action=(
                    f"update the registry from {project.docs_version} "
                    f"to the project-reported {actual_docs_version}"
                ),
                detail={
                    "registered": project.docs_version,
                    "project_reported": actual_docs_version,
                },
            )
        )
        checks.append(
            as_check(
                "beacon_context_strict",
                bool(context.get("pass")),
                reason_code="beacon_context_invalid",
                resume_action=(
                    f"run `beacon doctor setup-context --project-root {project.repo}` "
                    "and verify manually; Delivery Bus will not auto-repair"
                ),
                detail=context if isinstance(context, dict) else {"raw": context},
            )
        )
        return checks

    def verify_context(self, project: Project) -> dict[str, Any]:
        result = self.runner.run(
            [
                "beacon",
                "doctor",
                "verify-context",
                "--project-root",
                project.repo,
                "--strict",
                "--json",
            ],
            timeout=60,
        )
        try:
            payload = json.loads(result.stdout)
        except json.JSONDecodeError:
            payload = {}
        status = str(payload.get("status") or "").lower()
        passed = result.returncode == 0 and status in {"pass", "ok", "healthy"}
        return {
            "pass": passed,
            "returncode": result.returncode,
            "payload": payload,
            "stderr": result.stderr[-2000:],
        }

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
        docs = Path(project.docs_root) if project.docs_root else root / "docs" / "beacon"
        version = project.docs_version
        feature_root = docs / version / "features" / feature
        if stage == "plan":
            candidates = [
                feature_root / "truth.md",
                docs / version / "research" / f"{feature}.md",
                docs / version / "programs" / feature / "program-manifest.md",
            ]
            present = [str(path) for path in candidates if path.is_file()]
            return {"pass": bool(present), "evidence": present}
        if stage == "implement":
            evidence_dir = root / ".beacon" / "evidence" / "implement" / feature
            declared = (evidence_spec or {}).get("evidence_dir") or ""
            if declared:
                candidate = Path(declared)
                evidence_dir = candidate if candidate.is_absolute() else root / candidate
            present = [str(path) for path in evidence_dir.glob("*.json")] if evidence_dir.is_dir() else []
            if dispatch_id:
                manifest = evidence_dir / "manifest.json"
                manifest_ok = False
                if manifest.is_file():
                    try:
                        payload = json.loads(manifest.read_text(encoding="utf-8"))
                        manifest_ok = str(payload.get("dispatch_id") or "") == str(dispatch_id)
                    except (json.JSONDecodeError, OSError):
                        manifest_ok = False
                if not manifest_ok:
                    return {
                        "pass": False,
                        "reason_code": "evidence_ownership_mismatch",
                        "evidence": present,
                        "dispatch_id": dispatch_id,
                        "resume_action": (
                            "write evidence/manifest.json with the matching dispatch_id, then reconcile again"
                        ),
                    }
            return {"pass": bool(present), "evidence": present}
        if stage == "qa":
            qa_candidates = list((docs / version / ".machine" / "qa").glob(f"{feature}*.json"))
            for path in qa_candidates:
                try:
                    payload = json.loads(path.read_text(encoding="utf-8"))
                except (json.JSONDecodeError, OSError):
                    continue
                if payload.get("pass") is True or str(payload.get("status") or "").lower() == "pass":
                    return {"pass": True, "evidence": [str(path)], "payload": payload}
            return {"pass": False, "evidence": [str(path) for path in qa_candidates]}
        if stage == "freeze":
            truth = feature_root / "truth.md"
            revision = docs / version / ".machine" / "execution" / f"{feature}.revision.json"
            frozen = truth.is_file() and "status: frozen" in truth.read_text(encoding="utf-8")
            return {
                "pass": frozen and revision.is_file(),
                "evidence": [str(path) for path in (truth, revision) if path.is_file()],
            }
        return {"pass": False, "reason_code": "stage_not_enabled", "evidence": []}
