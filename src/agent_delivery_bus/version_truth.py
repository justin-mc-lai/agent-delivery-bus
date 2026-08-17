"""Version truth: ``docs/beacon/governance/version-truth-catalog.md`` is canonical.

All other version surfaces (pyproject, AGENTS.md, CLAUDE.md, onboarding state,
git tag) are read-only projections of the catalog. This module provides the
machine checks that keep the projections aligned. It intentionally has zero
runtime dependencies beyond the standard library.
"""

from __future__ import annotations

import json
import re
import subprocess
from pathlib import Path
from typing import Any


CATALOG_REL = Path("docs/beacon/governance/version-truth-catalog.md")
REQUIRED_KEYS = (
    "schema_version",
    "package_version",
    "active_docs_line",
    "runtime_version",
    "git_tag",
)


def parse_catalog(path: str | Path) -> dict[str, str]:
    """Parse the flat YAML-ish frontmatter of the catalog (stdlib only).

    Strict on purpose: an unparseable catalog must fail closed instead of
    letting projections drift silently.
    """
    text = Path(path).read_text(encoding="utf-8")
    match = re.match(r"^---\n(.*?)\n---", text, flags=re.DOTALL)
    if match is not None:
        block = match.group(1)
        if not block:
            raise ValueError(f"catalog frontmatter is empty: {path}")
        catalog: dict[str, str] = {}
        for line in block.splitlines():
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            if ":" not in line:
                raise ValueError(f"unparseable catalog line {line!r} in {path}")
            key, _, value = line.partition(":")
            key = key.strip()
            if not re.fullmatch(r"[a-z_][a-z0-9_]*", key):
                raise ValueError(f"invalid catalog key {key!r} in {path}")
            catalog[key] = value.strip().strip('"').strip("'")
    else:
        raise ValueError(f"catalog frontmatter missing in {path}")
    missing = [key for key in REQUIRED_KEYS if key not in catalog]
    if missing:
        raise ValueError(f"catalog missing required keys: {', '.join(missing)} in {path}")
    return catalog


def _pyproject_version(root: Path) -> str:
    text = (root / "pyproject.toml").read_text(encoding="utf-8")
    match = re.search(r'^version\s*=\s*"([^"]+)"', text, flags=re.MULTILINE)
    if match is None:
        raise ValueError("pyproject.toml does not declare [project].version")
    return match.group(1)


def _beacon_block_targets(path: Path) -> dict[str, str]:
    text = path.read_text(encoding="utf-8")
    docs = re.search(r"Docs target version: `([^`]+)`", text)
    runtime = re.search(r"Runtime target version: `([^`]+)`", text)
    if docs is None or runtime is None:
        raise ValueError(f"{path} is missing the Beacon target version lines")
    return {"docs": docs.group(1), "runtime": runtime.group(1)}


def _onboarding_state(root: Path) -> dict[str, str] | None:
    path = root / ".beacon" / "state" / "project-onboarding.json"
    if not path.is_file():
        return None
    payload = json.loads(path.read_text(encoding="utf-8"))
    return {
        "docs_version": str(payload.get("docs_version") or ""),
        "runtime_version": str(payload.get("runtime_version") or ""),
    }


def git_tag_exists(root: Path, tag: str) -> bool:
    result = subprocess.run(
        ["git", "-C", str(root), "tag", "--list", tag],
        capture_output=True,
        text=True,
        check=False,
    )
    return result.returncode == 0 and result.stdout.strip() == tag


def check_alignment(root: str | Path, *, check_tag: bool = False) -> list[str]:
    """Return a list of alignment problems (empty list means aligned).

    ``check_tag`` verifies the git tag projection; it is intended for local
    release verification, not for CI on fresh checkouts that may lack tags.
    ``project-onboarding.json`` is local state: it is checked when present.
    """
    root = Path(root).resolve()
    problems: list[str] = []
    try:
        catalog = parse_catalog(root / CATALOG_REL)
    except (OSError, ValueError) as exc:
        return [f"catalog unreadable/invalid: {exc}"]

    try:
        if _pyproject_version(root) != catalog["package_version"]:
            problems.append(
                f"pyproject.version != catalog.package_version "
                f"({_pyproject_version(root)} != {catalog['package_version']})"
            )
    except (OSError, ValueError) as exc:
        problems.append(f"pyproject unreadable: {exc}")

    for name in ("AGENTS.md", "CLAUDE.md"):
        path = root / name
        try:
            targets = _beacon_block_targets(path)
        except (OSError, ValueError) as exc:
            problems.append(f"{name} unreadable: {exc}")
            continue
        if targets["docs"] != catalog["active_docs_line"]:
            problems.append(
                f"{name} docs target {targets['docs']} != catalog.active_docs_line "
                f"{catalog['active_docs_line']}"
            )
        if targets["runtime"] != catalog["runtime_version"]:
            problems.append(
                f"{name} runtime target {targets['runtime']} != catalog.runtime_version "
                f"{catalog['runtime_version']}"
            )

    try:
        onboarding = _onboarding_state(root)
    except (OSError, ValueError) as exc:
        problems.append(f"onboarding state unreadable: {exc}")
        onboarding = None
    if onboarding is not None:
        if onboarding["docs_version"] != catalog["active_docs_line"]:
            problems.append(
                f"onboarding docs_version {onboarding['docs_version']} != "
                f"catalog.active_docs_line {catalog['active_docs_line']}"
            )
        if onboarding["runtime_version"] != catalog["runtime_version"]:
            problems.append(
                f"onboarding runtime_version {onboarding['runtime_version']} != "
                f"catalog.runtime_version {catalog['runtime_version']}"
            )

    if check_tag:
        expected = catalog["git_tag"]
        if not git_tag_exists(root, expected):
            problems.append(
                f"git tag {expected} missing (create after the human release gate)"
            )
    return problems


def main(argv: list[str] | None = None) -> int:
    import argparse

    parser = argparse.ArgumentParser(
        description="Verify version-truth-catalog projections (pyproject/AGENTS/CLAUDE/onboarding/tag)."
    )
    parser.add_argument("--check-tag", action="store_true", help="also verify the git tag projection")
    parser.add_argument("--root", default=".", help="repo root (default: current directory)")
    args = parser.parse_args(argv)
    problems = check_alignment(args.root, check_tag=args.check_tag)
    if problems:
        print("version alignment FAILED:")
        for problem in problems:
            print(f"- {problem}")
        return 1
    print("version alignment ok: catalog -> pyproject / AGENTS.md / CLAUDE.md / onboarding")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
