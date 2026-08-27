"""d1_registry — Cloudflare D1 cloud registry backend (f8 AC-F8-002).

Multi-machine shared registry: machines + projects live in Cloudflare D1,
readable/writable from any machine with a Cloudflare login (wrangler).
Backs `adb machines list --remote` and is the shared source of truth for
multi-host scheduling.

Uses `wrangler d1 execute --remote` as transport (no extra API calls, no
secrets in code). Fail-closed: if wrangler login is absent, commands report
blocked with reason `d1_not_authenticated`.
"""
from __future__ import annotations

import json
import subprocess
import sys
from typing import Any

DB = "adb-registry"


def _d1(sql: str, timeout: int = 60) -> tuple[bool, list[dict[str, Any]], str]:
    """Run SQL on remote D1. Returns (ok, rows, err)."""
    try:
        r = subprocess.run(
            ["wrangler", "d1", "execute", DB, "--remote", "--command", sql],
            capture_output=True, text=True, timeout=timeout,
        )
    except FileNotFoundError:
        return False, [], "wrangler not installed"
    except subprocess.TimeoutExpired:
        return False, [], "d1 timeout"
    if r.returncode != 0:
        err = (r.stderr or r.stdout or "")[-300:]
        if "not authenticated" in err.lower() or "login" in err.lower():
            return False, [], "d1_not_authenticated"
        return False, [], f"d1 error: {err}"
    try:
        text = r.stdout or ""
        start = text.find("[")
        data = json.loads(text[start:]) if start >= 0 else json.loads(text)
        batches = data if isinstance(data, list) else data.get("results") or []
        rows = []
        for batch in batches:
            rows.extend(batch.get("results") or [])
        return True, rows, ""
    except Exception as exc:
        return False, [], f"d1 parse error: {exc}"


def machines_list(capability: str = "") -> tuple[bool, list[dict[str, Any]], str]:
    sql = "SELECT machine_id,name,tailscale_name,capabilities,permission_level,status,registered_at,updated_at FROM machines WHERE status='active'"
    if capability:
        sql += f" AND capabilities LIKE '%{capability}%'"
    sql += " ORDER BY name"
    return _d1(sql)


def projects_list() -> tuple[bool, list[dict[str, Any]], str]:
    sql = "SELECT slug,repo,dispatchable,status,executor_machine,executor_agent,permission_level,updated_at FROM projects WHERE status='active' ORDER BY slug"
    return _d1(sql)


def machines_register(name: str, capabilities: str, permission_level: str) -> tuple[bool, dict[str, Any] | None, str]:
    import time
    ts = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    sql = (
        f"INSERT OR REPLACE INTO machines(machine_id,name,tailscale_name,capabilities,permission_level,status,registered_at,updated_at) "
        f"VALUES('mch_{name}','{name}','{name}','{capabilities}','{permission_level}','active','{ts}','{ts}')"
    )
    ok, _, err = _d1(sql)
    if not ok:
        return False, None, err
    ok2, rows, err2 = _d1(
        f"SELECT machine_id,name,tailscale_name,capabilities,permission_level,status,registered_at,updated_at FROM machines WHERE name='{name}'"
    )
    if not ok2:
        return False, None, err2
    return True, (rows[0] if rows else None), ""
