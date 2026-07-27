from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from ..process import CommandRunner
from ..registry import Project


class BeaconAdapter:
    def __init__(self, runner: CommandRunner | None = None):
        self.runner = runner or CommandRunner()

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

    def closure(self, project: Project, *, stage: str, feature: str) -> dict[str, Any]:
        root = Path(project.repo)
        docs = Path(project.beacon_docs_root)
        version = project.current_docs_version
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
            present = [str(path) for path in evidence_dir.glob("*.json")] if evidence_dir.is_dir() else []
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
