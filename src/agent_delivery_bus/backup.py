"""Local backup for the ADB control plane: SQLite ledger + registry configs.

Restore is the reverse copy: stop adb, replace ``data/agent-delivery-bus.sqlite3``
and ``config/projects*.json`` from a backup directory, restart. See
``docs/ops/backup-strategy.md``.
"""

from __future__ import annotations

import json
import shutil
import sqlite3
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Sequence

from .errors import DeliveryBusError


MANIFEST_NAME = "manifest.json"
DEFAULT_DB_NAME = "agent-delivery-bus.sqlite3"


def backup_control_plane(
    *,
    db_path: str | Path,
    dest: str | Path,
    config_paths: Sequence[str | Path],
) -> dict[str, Any]:
    """Copy the ledger (online-consistent) plus registry configs into ``dest``.

    ``config_paths`` lists the config files that must exist (the active registry
    at minimum). Missing required sources fail closed; nothing is written to a
    pre-existing destination directory.
    """
    dest = Path(dest)
    db_path = Path(db_path)
    if dest.exists() and any(dest.iterdir()):
        raise DeliveryBusError(
            "backup_dest_not_empty",
            f"backup destination is not empty: {dest}",
            resume_action="choose a fresh timestamped --dest and rerun `adb backup`",
        )
    if not db_path.is_file():
        raise DeliveryBusError(
            "backup_db_missing",
            f"ledger db not found: {db_path}",
            resume_action="point --db at the real ledger or run `adb doctor` first",
        )

    required = [Path(path) for path in config_paths]
    missing = [str(path) for path in required if not path.is_file()]
    if missing:
        raise DeliveryBusError(
            "backup_config_missing",
            "registry config missing: " + ", ".join(missing),
            resume_action="restore the config file or pass --config explicitly",
        )

    dest.mkdir(parents=True, exist_ok=True)
    copied: dict[str, str] = {}
    db_target = dest / DEFAULT_DB_NAME
    source = sqlite3.connect(str(db_path))
    target = sqlite3.connect(str(db_target))
    try:
        source.backup(target)
    finally:
        target.close()
        source.close()
    copied[DEFAULT_DB_NAME] = "copied"
    check = sqlite3.connect(str(db_target))
    try:
        integrity = str(check.execute("PRAGMA integrity_check").fetchone()[0])
    finally:
        check.close()

    for path in required:
        target = dest / path.name
        shutil.copy2(path, target)
        copied[path.name] = "copied"

    manifest = {
        "schema_version": "1.0",
        "created_at": datetime.now(timezone.utc).isoformat(),
        "source_db": str(db_path.expanduser().resolve()),
        "dest": str(dest.expanduser().resolve()),
        "files": copied,
        "integrity_check": integrity,
    }
    (dest / MANIFEST_NAME).write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    return manifest
