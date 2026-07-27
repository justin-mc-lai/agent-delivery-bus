from __future__ import annotations

import os
from pathlib import Path
from typing import Any

from .errors import DeliveryBusError


def install_skill(source: str | Path, *, dry_run: bool = False) -> dict[str, Any]:
    skill_source = Path(source).expanduser().resolve()
    if not (skill_source / "SKILL.md").is_file():
        raise DeliveryBusError("skill_source_invalid", f"Missing SKILL.md under {skill_source}")
    targets = [
        Path.home() / ".codex" / "skills" / "agent-delivery-bus",
        Path.home() / ".hermes" / "skills" / "productivity" / "agent-delivery-bus",
    ]
    actions: list[dict[str, Any]] = []
    for target in targets:
        if target.exists() or target.is_symlink():
            if target.is_symlink() and target.resolve() == skill_source:
                actions.append({"target": str(target), "status": "already_installed"})
                continue
            raise DeliveryBusError(
                "skill_target_exists",
                f"Refusing to overwrite existing skill target: {target}",
                resume_action="inspect or remove the existing target manually",
            )
        actions.append({"target": str(target), "status": "would_link" if dry_run else "linked"})
        if not dry_run:
            target.parent.mkdir(parents=True, exist_ok=True)
            os.symlink(skill_source, target, target_is_directory=True)
    return {"source": str(skill_source), "dry_run": dry_run, "actions": actions}
