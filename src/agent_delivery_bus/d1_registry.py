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
D1_DATABASE_ID = "f5920a55-f34d-4d7f-9cfa-3e477cb49a0f"
D1_ACCOUNT_ID = "a6ac9a09ce24078a41f93bfe0cbf5c39"


def _load_env() -> None:
    """Load Cloudflare credentials from ~/.config/adb-d1/token.env if present."""
    import os
    if os.environ.get("CLOUDFLARE_API_TOKEN"):
        return
    import pathlib
    envfile = pathlib.Path.home() / ".config" / "adb-d1" / "token.env"
    if envfile.exists():
        for line in envfile.read_text().splitlines():
            line = line.strip()
            if "=" in line and not line.startswith("#"):
                k, v = line.split("=", 1)
                os.environ.setdefault(k.strip(), v.strip())


def _d1(sql: str, timeout: int = 30) -> tuple[bool, list[dict[str, Any]], str]:
    """Run SQL on remote D1 via Cloudflare REST API (no wrangler subprocess).

    Faster and lighter than shelling out to `wrangler d1 execute`. Uses the
    CLOUDFLARE_API_TOKEN from ~/.config/adb-d1/token.env (fail-closed if absent).
    """
    _load_env()
    import os as _os
    import urllib.request
    token = _os.environ.get("CLOUDFLARE_API_TOKEN")
    account = _os.environ.get("CLOUDFLARE_ACCOUNT_ID") or D1_ACCOUNT_ID
    if not token:
        return False, [], "d1_not_authenticated"
    url = (
        f"https://api.cloudflare.com/client/v4/accounts/{account}"
        f"/d1/database/{D1_DATABASE_ID}/query"
    )
    body = json.dumps({"sql": sql, "params": []}).encode()
    req = urllib.request.Request(url, data=body, headers={
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    })
    # route through local Clash proxy (7897 macbook / 7890 mini)
    _node = _os.uname().nodename.lower()
    _proxy_host = "127.0.0.1:7890" if "mac-mini" in _node else "127.0.0.1:7897"
    proxy = urllib.request.ProxyHandler({"http": f"http://{_proxy_host}", "https": f"http://{_proxy_host}"})
    opener = urllib.request.build_opener(proxy)
    try:
        with opener.open(req, timeout=timeout) as resp:
            payload = json.loads(resp.read().decode())
    except urllib.error.HTTPError as exc:
        detail = ""
        try:
            detail = json.loads(exc.read().decode()).get("errors", [{}])[0].get("message", "")
        except Exception:
            pass
        if exc.code in (401, 403):
            return False, [], "d1_not_authenticated"
        return False, [], f"d1 api {exc.code}: {detail[:120]}"
    except Exception as exc:
        return False, [], f"d1 network error: {exc}"
    if not payload.get("success"):
        errs = payload.get("errors") or []
        return False, [], f"d1 error: {errs[0].get('message', str(errs)) if errs else 'unknown'}"
    results = payload.get("result") or []
    rows = []
    for batch in results:
        rows.extend(batch.get("results") or [])
    return True, rows, ""


def machines_list(capability: str = "") -> tuple[bool, list[dict[str, Any]], str]:
    sql = "SELECT machine_id,name,tailscale_name,capabilities,permission_level,status,registered_at,updated_at FROM machines WHERE status='active'"
    if capability:
        sql += f" AND capabilities LIKE '%{capability}%'"
    sql += " ORDER BY name"
    return _d1(sql)


def projects_list() -> tuple[bool, list[dict[str, Any]], str]:
    sql = "SELECT slug,repo,dispatchable,status,executor_machine,executor_agent,permission_level,updated_at FROM projects WHERE status='active' ORDER BY slug"
    return _d1(sql)


def _resolve_repo(repo: str, base_dir: str) -> str:
    """Resolve a cloud repo path to this machine's local path.

    Cloud stores relative paths like 'products/<slug>' (machine-agnostic).
    Absolute /Users/apple paths from old data are rewritten to this host.
    """
    import os as _os
    home = _os.path.expanduser("~")
    if repo.startswith("/Users/"):
        # legacy absolute path from another machine -> assume products/<basename>
        return _os.path.join(home, "Developer", "Personal", "products", _os.path.basename(repo))
    if repo.startswith(("http://", "https://", "git@")):
        return repo  # URL, unresolved
    # relative like products/<slug>
    return _os.path.join(home, "Developer", "Personal", repo)


def _to_git_url(slug: str) -> str:
    """Guess a GitHub URL for a slug (used for auto-clone). Empty = no auto-clone possible."""
    known = {
        "beacon": "justin-mc-lai/beacon",
        "agent-delivery-bus": "justin-mc-lai/agent-delivery-bus",
        "adb": "justin-mc-lai/agent-delivery-bus",
        "milemon": "justin-mc-lai/milemon-wordpress",
        "demo-app": "justin-mc-lai/demo-app",
        # shopxo is gitee+large+private; rsync instead of clone
    }
    slug_key = "adb" if slug in ("adb", "agent-delivery-bus") else slug
    if slug_key in known:
        return f"https://github.com/{known[slug_key]}.git"
    return ""


def sync_projects_from_cloud(
    local_projects: list[dict[str, Any]],
    base_dir: str,
    *,
    clone: bool = False,
) -> dict[str, Any]:
    """f10 AC-F10-001/002: align local registry to cloud D1; optionally clone missing repos.

    local_projects: current local registry rows (list of dicts with slug/repo).
    base_dir: directory to clone missing repos into (e.g. ~/Developer/Personal/products).
    Returns {synced, added, missing_repos, cloned, errors}.
    """
    import os
    import subprocess

    ok, cloud_rows, err = projects_list()
    if not ok:
        return {"status": "blocked", "reason": err, "synced": False}
    local_slugs = {p.get("slug") for p in local_projects if p.get("slug")}
    cloud_slugs = {r["slug"] for r in cloud_rows}
    added = sorted(cloud_slugs - local_slugs)
    missing_repos = []
    cloned = []
    errors = []

    for r in cloud_rows:
        slug = r["slug"]
        repo = r.get("repo") or ""
        local_repo = _resolve_repo(repo, base_dir)
        if repo and not os.path.isdir(local_repo):
            missing_repos.append({"slug": slug, "repo": local_repo})
            if clone:
                dest = os.path.join(base_dir, slug)
                if os.path.isdir(dest):
                    errors.append(f"{slug}: dest exists {dest} (skip)")
                    continue
                # cloud repo is relative (products/<slug>) or a URL; clone into base_dir/<slug>
                url = _to_git_url(slug)
                if url:
                    try:
                        subprocess.run(
                            ["git", "clone", url, dest],
                            capture_output=True, text=True, timeout=300,
                        )
                        cloned.append(slug)
                    except Exception as exc:
                        errors.append(f"{slug}: clone failed {exc}")
                else:
                    # no public repo URL -> cannot auto-clone; needs rsync from source machine
                    errors.append(f"{slug}: requires_rsync (no public clone URL)")
    return {
        "status": "pass",
        "synced": True,
        "cloud_count": len(cloud_rows),
        "local_count": len(local_slugs),
        "added": added,
        "missing_repos": missing_repos,
        "cloned": cloned,
        "errors": errors,
    }


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
