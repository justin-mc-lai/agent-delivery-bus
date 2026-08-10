"""Third-party enforced workflow lifecycle.

ADB is a generic dispatch kernel. A "workflow" is a binding-profile-shaped
declaration (stages -> skill/command, runner, evidence spec). Two open-source
peer skill workflows ship as presets (superpowers, openspec); beacon is the
first-party lifecycle and is NOT a preset. Arbitrary open-source repos are
adapted generically: adb inventories the repo (read-only) and produces an
analysis request; the HOST AGENT (the LLM running adb as a skill) fills the
response; adb validates, drafts, and installs only after human confirmation.
Every step is recorded as a JSONL trace for debugging/replay.
"""

from __future__ import annotations

import json
import shutil
import subprocess
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from .errors import DeliveryBusError
from .worker_binding import DEFAULT_RUNNER_PROFILE


PRESET_SOURCE = {
    "superpowers": "https://github.com/obra/superpowers",
    "openspec": "https://github.com/Fission-AI/OpenSpec",
}

REQUEST_SCHEMA = "workflow-analysis-request.v1"
RESPONSE_SCHEMA = "workflow-analysis-response.v1"
TRACE_SCHEMA = "workflow-trace.v1"

ALLOWED_PLACEHOLDERS = ("{feature}", "{stage}", "{docs_version}")
DANGEROUS_PATTERNS = (
    "rm -rf",
    "sudo ",
    "chmod -R 777",
    "curl ",
    "wget ",
    "| sh",
    "| bash",
    "base64 -d",
    "git push --force",
    "mkfs",
    "dd if=",
    "$(",
    "`",
)


def _now() -> str:
    return datetime.now(timezone.utc).isoformat()


def _base_workflow(slug: str, name: str, description: str, source: str) -> dict[str, Any]:
    return {
        "name": name,
        "description": description,
        "source": source,
        "skills": [slug],
        "runner": dict(DEFAULT_RUNNER_PROFILE),
        "stages": {},
        "evidence_spec": {
            "evidence_dir": f".adb/workflows/{slug}/{{stage}}/{{feature}}",
            "glob": "*.json",
            "required_files": ["manifest.json"],
            "dispatch_id_binding": True,
        },
    }


def build_preset(slug: str) -> dict[str, Any]:
    slug = (slug or "").strip().lower()
    if slug == "superpowers":
        wf = _base_workflow(
            "superpowers",
            "Superpowers",
            "Open-source Claude Code skill framework: brainstorm/plan/TDD/debug as skill workflows.",
            PRESET_SOURCE["superpowers"],
        )
        wf["stages"] = {
            "plan": {"skill": "superpowers", "public_harness": "plan", "command": 'superpowers plan "{feature}"'},
            "implement": {"skill": "superpowers", "public_harness": "implement", "command": 'superpowers implement "{feature}"'},
            "qa": {"skill": "superpowers", "public_harness": "qa", "command": 'superpowers tdd "{feature}"'},
            "freeze": {"skill": "superpowers", "public_harness": "truth", "command": 'superpowers finalize "{feature}"'},
        }
        return wf
    if slug == "openspec":
        wf = _base_workflow(
            "openspec",
            "OpenSpec",
            "Open-source spec-driven development workflow: spec -> tasks -> implementation -> verify.",
            PRESET_SOURCE["openspec"],
        )
        wf["stages"] = {
            "plan": {"skill": "openspec", "public_harness": "plan", "command": 'openspec plan "{feature}"'},
            "implement": {"skill": "openspec", "public_harness": "implement", "command": 'openspec implement "{feature}"'},
            "qa": {"skill": "openspec", "public_harness": "qa", "command": 'openspec verify "{feature}"'},
            "freeze": {"skill": "openspec", "public_harness": "truth", "command": 'openspec finalize "{feature}"'},
        }
        return wf
    raise DeliveryBusError(
        "workflow_preset_unknown",
        f"Unknown workflow preset: {slug!r}",
        resume_action=f"use one of: {', '.join(sorted(PRESET_SOURCE))}",
        data={"preset": slug},
    )


def workflow_names(raw: dict[str, Any] | None) -> list[str]:
    configured = raw.get("workflows") if isinstance(raw, dict) and isinstance(raw.get("workflows"), dict) else {}
    return sorted(set(PRESET_SOURCE) | set(str(k) for k in configured))


def get_workflow(raw: dict[str, Any] | None, name: str) -> dict[str, Any]:
    configured = raw.get("workflows") if isinstance(raw, dict) and isinstance(raw.get("workflows"), dict) else {}
    if name in configured and isinstance(configured[name], dict):
        return dict(configured[name])
    if name in PRESET_SOURCE:
        return build_preset(name)
    raise DeliveryBusError(
        "workflow_not_found",
        f"Workflow {name!r} is not configured and not a preset",
        resume_action="run `adb workflow list` or ingest a repo first",
        data={"workflow": name},
    )


def install_workflow(
    raw: dict[str, Any],
    *,
    name: str,
    preset: str,
    overrides: dict[str, Any] | None = None,
) -> dict[str, Any]:
    name = (name or "").strip()
    if not name:
        raise DeliveryBusError("workflow_name_required", "workflow install requires --name")
    template = build_preset(preset)
    if overrides:
        merged = dict(template)
        merged.update({k: v for k, v in (overrides or {}).items() if v is not None})
        if isinstance(overrides.get("stages"), dict):
            merged["stages"] = {**template.get("stages", {}), **overrides["stages"]}
        if isinstance(overrides.get("evidence_spec"), dict):
            merged["evidence_spec"] = {**template.get("evidence_spec", {}), **overrides["evidence_spec"]}
        template = merged
    workflows = raw.setdefault("workflows", {})
    workflows[name] = template
    return template


def remove_workflow(raw: dict[str, Any], name: str) -> dict[str, Any]:
    workflows = raw.get("workflows") if isinstance(raw.get("workflows"), dict) else {}
    if name not in workflows:
        raise DeliveryBusError(
            "workflow_not_configured",
            f"Workflow {name!r} is not in the local registry",
            resume_action="run `adb workflow list`",
            data={"workflow": name},
        )
    return dict(workflows.pop(name))


class TraceWriter:
    """Append-only JSONL trace for one workflow analysis/run chain."""

    def __init__(self, root: Path, name: str):
        self.dir = Path(root) / ".beacon" / "state" / "workflows" / name / "traces"
        self.dir.mkdir(parents=True, exist_ok=True)
        self.trace_id = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S%f")
        self.path = self.dir / f"{self.trace_id}.jsonl"

    def event(self, event: str, **fields: Any) -> str:
        line = {
            "schema": TRACE_SCHEMA,
            "trace_id": self.trace_id,
            "ts": _now(),
            "event": event,
            **fields,
        }
        with self.path.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps(line, ensure_ascii=False) + "\n")
        return self.trace_id

    @staticmethod
    def latest(root: Path, name: str) -> Path | None:
        traces = TraceWriter.all(root, name)
        return traces[-1] if traces else None

    @staticmethod
    def all(root: Path, name: str) -> list[Path]:
        return sorted((Path(root) / ".beacon" / "state" / "workflows" / name / "traces").glob("*.jsonl"))

    @staticmethod
    def read(path: Path) -> list[dict[str, Any]]:
        rows = []
        for line in path.read_text(encoding="utf-8").splitlines():
            if line.strip():
                try:
                    rows.append(json.loads(line))
                except json.JSONDecodeError:
                    continue
        return rows


def _git_commit(repo: Path) -> str:
    try:
        result = subprocess.run(
            ["git", "-C", str(repo), "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
            timeout=15,
        )
        if result.returncode == 0:
            return result.stdout.strip()[:12]
    except (OSError, subprocess.TimeoutExpired):
        pass
    return "local"


def _inventory(repo: Path) -> list[dict[str, str]]:
    anchors: list[dict[str, str]] = []
    skip_dirs = {".git", "node_modules", ".venv", "venv", "target", "__pycache__", ".pytest_cache", "dist"}
    walk = sorted(repo.rglob("*"))
    for path in walk:
        if any(part in skip_dirs for part in path.relative_to(repo).parts):
            continue
        if not path.is_file() or path.stat().st_size > 64 * 1024:
            continue
        rel = str(path.relative_to(repo))
        low = rel.casefold()
        kind = "other"
        if path.name == "SKILL.md" or "/skills/" in low:
            kind = "skill"
        elif low in {"pyproject.toml", "package.json", "makefile", "claude.md", "agents.md"}:
            kind = "cli"
        elif low.startswith(("readme", "docs/", "openspec/", "spec/", ".claude/skills/", ".cursor/skills/")):
            kind = "workflow"
        try:
            excerpt = path.read_text(encoding="utf-8", errors="replace")[:400]
        except OSError:
            continue
        anchors.append({"path": rel, "kind": kind, "excerpt": excerpt})
        if len(anchors) >= 80:
            break
    return anchors


def ingest_request(*, name: str, source: str, root: Path, workdir: Path) -> dict[str, Any]:
    """Fetch/copy source read-only, inventory anchors, and emit an analysis request."""
    name = (name or "").strip() or "ingested"
    state = Path(root) / ".beacon" / "state" / "workflows" / "_ingest"
    state.mkdir(parents=True, exist_ok=True)
    src = str(source).strip()
    if src.startswith(("http://", "https://", "git@", "git://")):
        target = state / name
        if target.exists():
            shutil.rmtree(target)
        subprocess.run(
            ["git", "clone", "--depth", "1", src, str(target)],
            check=True,
            capture_output=True,
            timeout=120,
        )
        repo = target
    else:
        repo = Path(src).expanduser().resolve()
        if not repo.is_dir():
            raise DeliveryBusError("workflow_source_missing", f"Source path is not a directory: {repo}")
    commit = _git_commit(repo)
    anchors = _inventory(repo)
    request = {
        "schema": REQUEST_SCHEMA,
        "schema_version": "1.0",
        "name": name,
        "source": src,
        "commit": commit,
        "anchors": anchors,
        "prompt": (
            f"Analyze repo {src} (commit {commit}) and fill a workflow-analysis-response.v1. "
            "Map its lifecycle into stages (plan/implement/qa/freeze/goal/truth are canonical; "
            "use only stages you found evidence for). Every field must cite an anchor path "
            "from this request. Never invent commands or skills without file evidence."
        ),
    }
    request_path = state / f"{name}.request.json"
    request_path.write_text(json.dumps(request, ensure_ascii=False, indent=2), encoding="utf-8")
    trace = TraceWriter(root, name)
    trace.event("inventory", anchors_count=len(anchors), source=src, commit=commit)
    trace.event("analysis_request", request_ref=str(request_path))
    return {
        "request_path": str(request_path),
        "trace_id": trace.trace_id,
        "anchors_count": len(anchors),
        "commit": commit,
    }


def validate_fill_response(request: dict[str, Any], response: dict[str, Any]) -> dict[str, Any]:
    problems: list[str] = []
    if response.get("schema") != RESPONSE_SCHEMA:
        problems.append("response schema must be workflow-analysis-response.v1")
    stages = response.get("stages") if isinstance(response.get("stages"), dict) else {}
    if not stages:
        problems.append("stages must be a non-empty map")
    evidence = response.get("fields_evidence") if isinstance(response.get("fields_evidence"), dict) else {}
    anchor_paths = {str(a.get("path")) for a in (request.get("anchors") or [])}
    for stage, meta in stages.items():
        if not isinstance(meta, dict) or not meta.get("skill") or not meta.get("command"):
            problems.append(f"stage {stage!r} needs skill and command")
            continue
        command = str(meta["command"])
        for bad in DANGEROUS_PATTERNS:
            if bad in command:
                problems.append(f"stage {stage!r} command contains dangerous token {bad!r}")
        for token in ("{", "}"):
            if token in command:
                pass
        import re

        for match in re.findall(r"\{[^}]+\}", command):
            if match not in ALLOWED_PLACEHOLDERS:
                problems.append(f"stage {stage!r} command uses unsupported placeholder {match!r}")
        refs = evidence.get(stage) or []
        if not isinstance(refs, list) or not any(str(r) in anchor_paths for r in refs):
            problems.append(f"stage {stage!r} has no anchor evidence")
    spec = response.get("evidence_spec") if isinstance(response.get("evidence_spec"), dict) else {}
    for key in ("evidence_dir", "glob", "dispatch_id_binding"):
        if key not in spec:
            problems.append(f"evidence_spec missing {key!r}")
    skills = response.get("skills") or []
    if not isinstance(skills, list) or not skills:
        problems.append("skills must be a non-empty list")
    return {"pass": not problems, "problems": problems, "problems_count": len(problems)}


def draft_apply(*, name: str, root: Path, request: dict[str, Any], response: dict[str, Any]) -> dict[str, Any]:
    validation = validate_fill_response(request, response)
    draft_dir = Path(root) / ".beacon" / "state" / "workflows" / name
    draft_dir.mkdir(parents=True, exist_ok=True)
    trace = TraceWriter(root, name)
    if not validation["pass"]:
        trace.event("validation", status="fail", problems=validation["problems"])
        raise DeliveryBusError(
            "workflow_validation_failed",
            "Host fill response failed validation",
            resume_action="fix the response fields and re-apply",
            data={"problems": validation["problems"]},
        )
    workflow = {
        "name": response.get("name") or name,
        "description": response.get("description") or "",
        "source": request.get("source") or "",
        "commit": request.get("commit") or "local",
        "skills": list(response.get("skills") or []),
        "runner": dict(response.get("runner") or DEFAULT_RUNNER_PROFILE),
        "stages": dict(response.get("stages") or {}),
        "evidence_spec": dict(response.get("evidence_spec") or {}),
    }
    draft = {"schema": "workflow-draft.v1", "workflow": workflow, "validation": validation}
    draft_path = draft_dir / "draft.json"
    draft_path.write_text(json.dumps(draft, ensure_ascii=False, indent=2), encoding="utf-8")
    trace.event("validation", status="pass", draft_ref=str(draft_path))
    trace.event("host_fill_response", agent="host", response_ref="inline")
    return {"draft_path": str(draft_path), "trace_id": trace.trace_id, "workflow": workflow}


def confirm_install(*, name: str, root: Path, raw: dict[str, Any]) -> dict[str, Any]:
    draft_path = Path(root) / ".beacon" / "state" / "workflows" / name / "draft.json"
    if not draft_path.is_file():
        raise DeliveryBusError(
            "workflow_draft_missing",
            f"No draft for workflow {name!r}",
            resume_action="run `adb workflow draft apply` after the host agent fills the response",
            data={"name": name},
        )
    draft = json.loads(draft_path.read_text(encoding="utf-8"))
    workflow = draft["workflow"]
    workflows = raw.setdefault("workflows", {})
    workflows[name] = workflow
    trace = TraceWriter(root, name)
    trace.event(
        "install",
        workflow=name,
        commit=workflow.get("commit"),
        trace_id=trace.trace_id,
        source=workflow.get("source"),
    )
    return {"installed": name, "workflow": workflow, "trace_id": trace.trace_id}


def verify_workflow(
    *,
    name: str,
    raw: dict[str, Any],
    root: Path,
    executor: Any,
    project: Any = None,
    service: Any = None,
) -> dict[str, Any]:
    workflow = get_workflow(raw, name)
    checks: list[dict[str, Any]] = []
    missing_skills = executor.skills_available(workflow.get("skills") or [])["missing"] if hasattr(
        executor, "skills_available"
    ) else []
    checks.append({"name": "skills", "pass": not missing_skills, "detail": {"missing": missing_skills}})
    stages = workflow.get("stages") or {}
    checks.append({"name": "stages", "pass": bool(stages), "detail": {"stages": sorted(stages)}})
    spec = workflow.get("evidence_spec") or {}
    checks.append(
        {
            "name": "evidence_spec",
            "pass": all(k in spec for k in ("evidence_dir", "glob", "dispatch_id_binding")),
            "detail": {"keys": sorted(spec)},
        }
    )
    if service is not None and project is not None:
        for stage in sorted(stages):
            result = service.dispatch(
                project_slug=project.slug,
                stage=stage,
                feature="__verify__",
                dry_run=True,
            )
            checks.append(
                {
                    "name": f"dry_run:{stage}",
                    "pass": not result.get("blocked"),
                    "detail": {"reason_code": result.get("reason_code", "")},
                }
            )
    report = {"workflow": name, "pass": all(c["pass"] for c in checks), "checks": checks}
    trace = TraceWriter(root, name)
    trace.event("verify", pass_=report["pass"], checks=checks)
    return report
