"""Third-party enforced workflow presets + user-configured workflow registry.

ADB is a generic dispatch kernel: a "workflow" is a binding-profile-shaped
declaration (stages → skill/command, runner, evidence spec). Presets ship two
popular open-source workflows (Aider, OpenHands); private/company workflows
(e.g. beacon-goal) live only in the local project registry, never in presets.
"""

from __future__ import annotations

from typing import Any

from .errors import DeliveryBusError
from .worker_binding import DEFAULT_RUNNER_PROFILE


PRESET_SOURCE = {
    "aider": "https://github.com/Aider-AI/aider",
    "openhands": "https://github.com/All-Hands-AI/OpenHands",
}


def _base_workflow(slug: str, name: str, description: str, source: str) -> dict[str, Any]:
    return {
        "name": name,
        "description": description,
        "source": source,
        "skills": [slug],
        "runner": dict(DEFAULT_RUNNER_PROFILE),
        "stages": {},
        "evidence_spec": {
            "evidence_dir": f".adb/workflows/{name}/{{stage}}/{{feature}}",
            "glob": "*.json",
            "required_files": ["manifest.json"],
            "dispatch_id_binding": True,
        },
    }


def build_preset(slug: str) -> dict[str, Any]:
    """Return a ready-to-install binding-profile-shaped workflow preset."""
    slug = (slug or "").strip().lower()
    if slug == "aider":
        wf = _base_workflow(
            "aider",
            "Aider",
            "Pair-programming AI with an enforced test command (open-source).",
            PRESET_SOURCE["aider"],
        )
        wf["stages"] = {
            "plan": {
                "skill": "aider",
                "public_harness": "plan",
                "command": 'aider --message "plan: {feature}" --yes-always',
            },
            "implement": {
                "skill": "aider",
                "public_harness": "implement",
                "command": 'aider --message "implement: {feature}" --test-cmd "pytest -q" --yes-always',
            },
            "qa": {
                "skill": "aider",
                "public_harness": "qa",
                "command": 'aider --message "verify: {feature}" --test-cmd "pytest -q" --yes-always',
            },
            "freeze": {
                "skill": "aider",
                "public_harness": "truth",
                "command": 'aider --message "freeze summary: {feature}" --yes-always',
            },
        }
        return wf
    if slug == "openhands":
        wf = _base_workflow(
            "openhands",
            "OpenHands",
            "Autonomous software agent workflow for long-horizon tasks (open-source).",
            PRESET_SOURCE["openhands"],
        )
        wf["stages"] = {
            "plan": {
                "skill": "openhands",
                "public_harness": "plan",
                "command": 'openhands run --task "plan: {feature}" --cwd .',
            },
            "implement": {
                "skill": "openhands",
                "public_harness": "implement",
                "command": 'openhands run --task "implement: {feature}" --cwd .',
            },
            "qa": {
                "skill": "openhands",
                "public_harness": "qa",
                "command": 'openhands run --task "verify: {feature} (run tests)" --cwd .',
            },
            "freeze": {
                "skill": "openhands",
                "public_harness": "truth",
                "command": 'openhands run --task "freeze summary: {feature}" --cwd .',
            },
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
    """Resolve a workflow by name: configured registry first, then preset template."""
    configured = raw.get("workflows") if isinstance(raw, dict) and isinstance(raw.get("workflows"), dict) else {}
    if name in configured and isinstance(configured[name], dict):
        return dict(configured[name])
    if name in PRESET_SOURCE:
        return build_preset(name)
    raise DeliveryBusError(
        "workflow_not_found",
        f"Workflow {name!r} is not configured and not a preset",
        resume_action="run `adb workflow install --preset <name>` or add config['workflows']",
        data={"workflow": name},
    )


def install_workflow(
    raw: dict[str, Any],
    *,
    name: str,
    preset: str,
    overrides: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """Install a preset as a named user workflow (overrides merged on top)."""
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
    removed = dict(workflows.pop(name))
    return removed
