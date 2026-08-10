"""Stage → worker binding profiles (Beacon is the built-in reference profile).

The dispatch envelope is truth-gate agnostic: every task body carries a binding
manifest (which skill/command the worker should run) plus an evidence spec
(where evidence must be written and how closure will verify ownership).
``beacon`` is the built-in reference profile; projects may declare their own
profile through the project registry.
"""

from __future__ import annotations

from typing import Any

from .errors import DeliveryBusError


# goal is intentionally absent until an explicit promote/change.
ENABLED_STAGES = frozenset({"plan", "truth", "implement", "qa", "freeze", "goal"})
DEFERRED_STAGES = frozenset()

# Local Hermes coding profile (or explicit Codex / equivalent). No cloud scheduler.
DEFAULT_RUNNER_PROFILE = {
    "runner_kind": "local_agent",
    "hermes_assignee": "coding",
    "allowed_profiles": ("coding", "codex"),
    "cloud_scheduler_forbidden": True,
}

DEFAULT_BINDING_PROFILE = "beacon"
BINDING_SCHEMA_VERSION = "1.1"

# Built-in reference profile: stage → Beacon skill / command.
STAGE_BEACON_BINDING: dict[str, dict[str, str]] = {
    "goal": {
        "beacon_skill": "beacon-goal",
        "public_harness": "goal",
        "beacon_command_template": (
            'beacon goal run "{feature}" --project . --version {docs_version} --auto-tick'
        ),
    },
    "plan": {
        "beacon_skill": "beacon-plan",
        "public_harness": "plan",
        "beacon_command_template": 'beacon workflow run plan "{feature}" --project . --version {docs_version}',
    },
    "truth": {
        "beacon_skill": "beacon-truth",
        "public_harness": "truth",
        "beacon_command_template": (
            'beacon truth gen "{feature}" --project . --version {docs_version} --intent-first'
        ),
    },
    "implement": {
        "beacon_skill": "beacon-implement",
        "public_harness": "implement",
        "beacon_command_template": (
            'beacon implement run "{feature}" --project . --version {docs_version} --mode single'
        ),
    },
    "qa": {
        "beacon_skill": "beacon-qa",
        "public_harness": "qa",
        "beacon_command_template": 'beacon qa "{feature}" --project . --version {docs_version}',
    },
    "freeze": {
        "beacon_skill": "beacon-truth",
        "public_harness": "truth",
        "beacon_command_template": 'beacon freeze "{feature}" --project . --version {docs_version}',
    },
}


def assert_stage_enabled(stage: str) -> str:
    """Normalize stage and fail-closed for deferred / unknown stages."""
    normalized = stage.strip().lower()
    if normalized in DEFERRED_STAGES:
        raise DeliveryBusError(
            "goal_stage_deferred",
            "goal stage binding is deferred until explicitly promoted",
            resume_action="dispatch plan/implement/qa/freeze, or promote goal via a governed change",
            data={"stage": normalized, "enabled_stages": sorted(ENABLED_STAGES)},
        )
    if normalized == "release":
        raise DeliveryBusError(
            "stage_not_enabled",
            "Automatic release dispatch is disabled",
            resume_action="run release manually after reviewing evidence",
            data={"stage": normalized},
        )
    if normalized not in ENABLED_STAGES:
        raise DeliveryBusError(
            "stage_invalid",
            f"Unsupported stage: {normalized}",
            data={"stage": normalized, "enabled_stages": sorted(ENABLED_STAGES)},
        )
    return normalized


def _normalize_profile(binding_profile: str = "") -> str:
    slug = (binding_profile or DEFAULT_BINDING_PROFILE).strip()
    return slug or DEFAULT_BINDING_PROFILE


def _resolve_profile_config(
    binding_profile: str,
    profile_config: dict[str, Any] | None,
) -> tuple[str, dict[str, Any] | None]:
    slug = _normalize_profile(binding_profile)
    if slug == "beacon":
        return slug, None
    cfg = profile_config if isinstance(profile_config, dict) else {}
    if not cfg:
        raise DeliveryBusError(
            "binding_profile_unknown",
            f"Unknown worker binding profile: {slug!r}",
            resume_action=(
                "declare project metadata['binding_profile'] (stages/runner/evidence_spec) "
                "or use the builtin 'beacon' profile"
            ),
            data={"binding_profile": slug},
        )
    stages = cfg.get("stages") if isinstance(cfg.get("stages"), dict) else {}
    if not stages:
        raise DeliveryBusError(
            "binding_profile_stages_missing",
            f"Binding profile {slug!r} declares no stages",
            resume_action="add metadata['binding_profile']['stages'] with plan/implement/qa/freeze entries",
            data={"binding_profile": slug},
        )
    return slug, cfg


def _evidence_spec_for(
    profile_slug: str,
    profile_config: dict[str, Any] | None,
    *,
    stage: str,
    feature: str,
    project_repo: str = "",
    dispatch_id: str = "",
) -> dict[str, Any]:
    repo = (project_repo or "").strip()
    if profile_slug == "beacon":
        base = f"{repo}/.beacon/evidence/{stage}/{feature}" if repo else f".beacon/evidence/{stage}/{feature}"
        return {
            "schema_version": BINDING_SCHEMA_VERSION,
            "evidence_dir": base,
            "glob": "*.json",
            "required_files": ["manifest.json"],
            "dispatch_id_binding": True,
            "dispatch_id": dispatch_id,
        }
    raw_spec = (profile_config or {}).get("evidence_spec")
    if not isinstance(raw_spec, dict) or not raw_spec:
        raise DeliveryBusError(
            "binding_profile_evidence_spec_required",
            f"Binding profile {profile_slug!r} declares no evidence_spec",
            resume_action=(
                "add metadata['binding_profile']['evidence_spec'] "
                "with evidence_dir/glob/dispatch_id_binding"
            ),
            data={"binding_profile": profile_slug},
        )
    spec = dict(raw_spec)
    spec.setdefault("schema_version", BINDING_SCHEMA_VERSION)
    declared_dir = str(
        spec.get("evidence_dir")
        or (f"{repo}/.adb/evidence/{stage}/{feature}" if repo else f".adb/evidence/{stage}/{feature}")
    )
    spec["evidence_dir"] = (
        declared_dir.replace("{feature}", feature)
        .replace("{stage}", stage)
        .replace("{dispatch_id}", dispatch_id)
    )
    spec.setdefault("glob", "*.json")
    spec.setdefault("required_files", ["manifest.json"])
    spec.setdefault("dispatch_id_binding", True)
    spec["dispatch_id"] = dispatch_id
    return spec


def resolve_worker_binding(
    *,
    stage: str,
    feature: str,
    docs_version: str = "",
    binding_profile: str = "",
    profile_config: dict[str, Any] | None = None,
    project_repo: str = "",
    dispatch_id: str = "",
) -> dict[str, Any]:
    """Return the stage binding for the project's binding profile.

    The builtin ``beacon`` profile keeps the legacy worker-beacon-binding
    contract fields (``beacon_skill`` / ``beacon_command``) and the
    ``### Beacon worker binding`` heading. Custom profiles emit profile-owned
    fields (``skill`` / ``command``) and never contain Beacon-only keys.
    """
    normalized = assert_stage_enabled(stage)
    slug, cfg = _resolve_profile_config(binding_profile, profile_config)
    version = (docs_version or "").strip() or "(none)"
    evidence_spec = _evidence_spec_for(
        slug,
        cfg,
        stage=normalized,
        feature=feature.strip(),
        project_repo=project_repo,
        dispatch_id=dispatch_id,
    )
    if slug == "beacon":
        meta = STAGE_BEACON_BINDING[normalized]
        command = meta["beacon_command_template"].format(feature=feature.strip(), docs_version=version)
        return {
            "schema_version": BINDING_SCHEMA_VERSION,
            "binding_profile": slug,
            "stage": normalized,
            "feature": feature.strip(),
            "beacon_skill": meta["beacon_skill"],
            "public_harness": meta["public_harness"],
            "beacon_command": command,
            "runner": dict(DEFAULT_RUNNER_PROFILE),
            "runner_profile": DEFAULT_RUNNER_PROFILE["hermes_assignee"],
            "evidence_spec": evidence_spec,
            "skills": [meta["beacon_skill"]],
        }
    stages = cfg["stages"]  # type: ignore[index]
    meta = stages.get(normalized)
    if not isinstance(meta, dict):
        raise DeliveryBusError(
            "binding_profile_stage_missing",
            f"Binding profile {slug!r} has no entry for stage {normalized!r}",
            resume_action=f"add metadata['binding_profile']['stages']['{normalized}']",
            data={"binding_profile": slug, "stage": normalized},
        )
    command = str(meta.get("command") or "").format(feature=feature.strip(), docs_version=version)
    runner = dict(cfg.get("runner") or DEFAULT_RUNNER_PROFILE)
    return {
        "schema_version": BINDING_SCHEMA_VERSION,
        "binding_profile": slug,
        "stage": normalized,
        "feature": feature.strip(),
        "skill": str(meta.get("skill") or ""),
        "public_harness": str(meta.get("public_harness") or ""),
        "command": command,
        "runner": runner,
        "runner_profile": str(
            runner.get("hermes_assignee")
            or runner.get("runner_profile")
            or DEFAULT_RUNNER_PROFILE["hermes_assignee"]
        ),
        "evidence_spec": evidence_spec,
        "skills": [str(meta.get("skill") or "")] if meta.get("skill") else [],
    }


def format_binding_section(binding: dict[str, Any]) -> str:
    """Render the machine-readable binding manifest embedded in task bodies."""
    runner = binding.get("runner") or {}
    is_beacon = binding.get("binding_profile") == "beacon"
    lines = [
        "### Beacon worker binding" if is_beacon else "### Worker binding",
        f"- stage: {binding['stage']}",
        f"- binding_profile: {binding.get('binding_profile', '')}",
        f"- schema_version: {binding.get('schema_version', '')}",
    ]
    if is_beacon:
        lines.extend(
            [
                f"- beacon_skill: {binding.get('beacon_skill', '')}",
                f"- public_harness: {binding.get('public_harness', '')}",
                f"- beacon_command: {binding.get('beacon_command', '')}",
            ]
        )
    else:
        lines.extend(
            [
                f"- skill: {binding.get('skill', '')}",
                f"- public_harness: {binding.get('public_harness', '')}",
                f"- command: {binding.get('command', '')}",
            ]
        )
    lines.extend(
        [
            f"- runner_kind: {runner.get('runner_kind', '')}",
            f"- runner_profile: {binding.get('runner_profile', '')}",
            f"- allowed_profiles: {', '.join(runner.get('allowed_profiles') or ())}",
            "- cloud_scheduler_forbidden: true",
            "",
            (
                "Invoke the bound Beacon skill/command for this stage inside the local runner profile."
                if is_beacon
                else "Invoke the bound skill/command for this stage inside the local runner profile."
            ),
            "Do not assume a cloud cluster scheduler; do not skip approval gates for restricted stages.",
        ]
    )
    return "\n".join(lines)


def format_evidence_spec_section(evidence_spec: dict[str, Any]) -> str:
    """Render the evidence contract so any worker knows where and how to report."""
    lines = [
        "### Evidence spec",
        f"- schema_version: {evidence_spec.get('schema_version', '')}",
        f"- evidence_dir: {evidence_spec.get('evidence_dir', '')}",
        f"- glob: {evidence_spec.get('glob', '*.json')}",
        f"- required_files: {', '.join(evidence_spec.get('required_files') or [])}",
        f"- dispatch_id_binding: {str(evidence_spec.get('dispatch_id_binding', True)).lower()}",
        f"- dispatch_id: {evidence_spec.get('dispatch_id', '')}",
        "",
        (
            "Write stage evidence into evidence_dir and create manifest.json with "
            '{"dispatch_id": "<dispatch_id>", "files": [...]}. '
            "Closure will reject evidence whose dispatch_id does not match this dispatch."
        ),
    ]
    return "\n".join(lines)
