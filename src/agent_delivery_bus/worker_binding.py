"""Stage → Beacon skill / local runner binding for Hermes task bodies."""

from __future__ import annotations

from typing import Any

from .errors import DeliveryBusError


# goal is intentionally absent until an explicit promote/change.
ENABLED_STAGES = frozenset({"plan", "implement", "qa", "freeze"})
DEFERRED_STAGES = frozenset({"goal"})

# Local Hermes coding profile (or explicit Codex / equivalent). No cloud scheduler.
DEFAULT_RUNNER_PROFILE = {
    "runner_kind": "local_agent",
    "hermes_assignee": "coding",
    "allowed_profiles": ("coding", "codex"),
    "cloud_scheduler_forbidden": True,
}

STAGE_BEACON_BINDING: dict[str, dict[str, str]] = {
    "plan": {
        "beacon_skill": "beacon-plan",
        "public_harness": "plan",
        "beacon_command_template": 'beacon workflow run plan "{feature}" --project . --version {docs_version}',
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


def resolve_worker_binding(
    *,
    stage: str,
    feature: str,
    docs_version: str = "",
) -> dict[str, Any]:
    """Return the stage→Beacon skill binding used in Hermes task bodies."""
    normalized = assert_stage_enabled(stage)
    meta = STAGE_BEACON_BINDING[normalized]
    version = (docs_version or "").strip() or "(none)"
    command = meta["beacon_command_template"].format(feature=feature.strip(), docs_version=version)
    return {
        "schema_version": "1.0",
        "stage": normalized,
        "feature": feature.strip(),
        "beacon_skill": meta["beacon_skill"],
        "public_harness": meta["public_harness"],
        "beacon_command": command,
        "runner": dict(DEFAULT_RUNNER_PROFILE),
        "runner_profile": DEFAULT_RUNNER_PROFILE["hermes_assignee"],
    }


def format_binding_section(binding: dict[str, Any]) -> str:
    runner = binding.get("runner") or {}
    lines = [
        "### Beacon worker binding",
        f"- stage: {binding['stage']}",
        f"- beacon_skill: {binding['beacon_skill']}",
        f"- public_harness: {binding['public_harness']}",
        f"- beacon_command: {binding['beacon_command']}",
        f"- runner_kind: {runner.get('runner_kind', '')}",
        f"- runner_profile: {binding.get('runner_profile', '')}",
        f"- allowed_profiles: {', '.join(runner.get('allowed_profiles') or ())}",
        "- cloud_scheduler_forbidden: true",
        "",
        "Invoke the bound Beacon skill/command for this stage inside the local runner profile.",
        "Do not assume a cloud cluster scheduler; do not skip approval gates for restricted stages.",
    ]
    return "\n".join(lines)
