"""``adb projects import --scan``: register every repo that carries beacon truth.

Frozen by ``docs/beacon/v0.1.5/features/projects-import/truth.md``:

* AC-001 scanning collects only *direct subdirectories* of ``--scan`` that hold
  ``docs/beacon/<version>/features/``; ``slug`` is the directory name, ``repo``
  the absolute path and ``docs_version`` the highest version directory
  (numeric-aware, so ``v0.1.10`` beats ``v0.1.9``).  The version helper is
  reused from :mod:`agent_delivery_bus.requirement_index` instead of being
  reimplemented, so importing and requirement-index never disagree.
* AC-002 the CLI is dry-run unless ``--apply`` is passed; nothing here writes
  on its own.
* AC-003 ``--apply`` writes the registry file, touching **only** ``projects``:
  ``schema_version`` / ``adapters`` / ``notes`` / ``workflows`` / any unknown
  key survive verbatim because the whole document is loaded, mutated in place
  and written back atomically.
* AC-004 a slug that is already registered is skipped and reported as
  ``existing``; ``--overwrite`` refreshes its ``repo`` + ``docs_version``
  instead (its ``index``, ``title``, ``class``, ``aliases``, ``status`` stay).
* AC-005 ``--register-kernel`` asks the kernel to learn each written entry via
  ``beacon-go projects register --project <repo> --version <v>``.  One failure
  never blocks the others and never blocks the registry write.
* AC-006 a missing scan directory, an empty scan result and an unreadable
  registry are reported as ``scan_dir_missing`` / ``no_projects_found`` /
  ``registry_invalid`` -- reason codes, not exceptions that abort the CLI.

Invariant relied on by the count line: ``new + existing + updated == found``.
Every scanned repo lands in exactly one of the three buckets, so
``found=<n> new=<m> existing=<k> updated=<u>`` always adds up.

Nothing here deletes a registration, clones a repository, rewrites a scanned
project or touches beacon truth documents: the scan is read-only and the
registry is the only file written.
"""

from __future__ import annotations

import json
import os
import subprocess
import tempfile
from pathlib import Path
from typing import Any, Callable, Mapping, Sequence

from .errors import DeliveryBusError
from .requirement_index import resolve_version

#: ``reason_code`` values fixed by AC-006.
SCAN_DIR_MISSING = "scan_dir_missing"
NO_PROJECTS_FOUND = "no_projects_found"
REGISTRY_INVALID = "registry_invalid"

#: Shape of a registry file this module is willing to create from scratch.
DEFAULT_SCHEMA_VERSION = "1.0"
#: Imported repos are ordinary delivery targets; the registry accepts them as
#: ``managed`` (see :data:`agent_delivery_bus.registry.ALLOWED_CLASSES`).
DEFAULT_PROJECT_CLASS = "managed"
DEFAULT_DOCS_ROOT = "docs/beacon"

#: The kernel call from AC-005.  Injected in tests; never the real binary there.
KERNEL_COMMAND = ("beacon-go", "projects", "register")
KERNEL_TIMEOUT_SECONDS = 60

#: ``runner(argv) -> int | bool | CompletedProcess``; may raise to signal failure.
KernelRunner = Callable[[Sequence[str]], Any]


# --- scanning (AC-001) ------------------------------------------------------


def scan_projects(root: str | Path) -> list[dict[str, str]]:
    """Every direct subdirectory of ``root`` that carries a beacon features tree.

    Returns ``[{"slug", "repo", "docs_version"}]`` sorted by slug.  Each entry
    is a candidate registration only: this function never writes anything.

    A root that is not a directory raises ``scan_dir_missing`` so the CLI can
    turn it into a reason code (AC-006) instead of a traceback.
    """
    raw = str(root or "").strip()
    if not raw:
        raise DeliveryBusError(
            SCAN_DIR_MISSING,
            "scan directory is required (--scan <dir>)",
            resume_action="pass --scan <dir> pointing at a directory of repositories",
        )
    base = Path(raw).expanduser()
    if not base.is_dir():
        raise DeliveryBusError(
            SCAN_DIR_MISSING,
            f"scan directory does not exist: {base}",
            resume_action="pass --scan <dir> pointing at a directory of repositories",
        )
    try:
        children = sorted(base.iterdir(), key=lambda item: item.name)
    except OSError as exc:  # an unreadable directory is a missing one, not a crash
        raise DeliveryBusError(
            SCAN_DIR_MISSING,
            f"scan directory unreadable: {base} ({exc})",
            resume_action="check permissions on the --scan directory and rerun",
        ) from None

    entries: list[dict[str, str]] = []
    for child in children:
        if not child.is_dir():
            continue
        # Same rule the requirement index uses: the highest
        # ``docs/beacon/<v>/features/`` wins; no features tree means "not a
        # beacon project" and the directory is ignored.
        version = resolve_version(child)
        if not version:
            continue
        entries.append(
            {
                "slug": child.name,
                "repo": str(child.resolve()),
                "docs_version": version,
            }
        )
    return entries


# --- registry file (AC-003 / AC-006) ----------------------------------------


def _empty_registry() -> dict[str, Any]:
    """What a first-ever registry looks like (consumable by ``ProjectRegistry``)."""
    return {
        "schema_version": DEFAULT_SCHEMA_VERSION,
        "adapters": {},
        "notes": {},
        "projects": [],
    }


def load_registry(path: str | Path) -> dict[str, Any]:
    """Read the registry as a plain document, ready to be mutated and written back.

    A missing file is not an error -- importing into a fresh install must work
    -- so it yields the empty skeleton from :func:`_empty_registry`.  Anything
    that cannot be safely extended (undecodable, non-JSON, not an object,
    ``projects`` not a list, a row that is not an object, duplicate slugs)
    raises ``registry_invalid`` rather than silently dropping a registration.
    """
    target = Path(str(path)).expanduser()
    if not target.is_file():
        return _empty_registry()
    try:
        text = target.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError) as exc:
        raise DeliveryBusError(
            REGISTRY_INVALID,
            f"project registry unreadable: {target} ({exc})",
            resume_action="fix the file permissions or restore it from config/projects.example.json",
        ) from None
    try:
        document = json.loads(text)
    except json.JSONDecodeError as exc:
        raise DeliveryBusError(
            REGISTRY_INVALID,
            f"project registry is not JSON: {target} ({exc})",
            resume_action="repair config/projects.json or start again from config/projects.example.json",
        ) from None
    if not isinstance(document, dict):
        raise DeliveryBusError(
            REGISTRY_INVALID,
            f"project registry is not a JSON object: {target}",
            resume_action="repair config/projects.json or start again from config/projects.example.json",
        )
    rows = document.get("projects")
    if rows is None:
        document["projects"] = []
        rows = document["projects"]
    if not isinstance(rows, list):
        raise DeliveryBusError(
            REGISTRY_INVALID,
            f"project registry 'projects' must be an array: {target}",
            resume_action="repair config/projects.json so that 'projects' is a JSON array",
        )
    seen: set[str] = set()
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            raise DeliveryBusError(
                REGISTRY_INVALID,
                f"project registry projects[{index}] is not an object: {target}",
                resume_action="repair config/projects.json so every entry is a JSON object",
            )
        slug = str(row.get("slug") or "").strip()
        if not slug:
            raise DeliveryBusError(
                REGISTRY_INVALID,
                f"project registry projects[{index}] has no slug: {target}",
                resume_action="repair config/projects.json so every entry has a unique slug",
            )
        if slug in seen:
            raise DeliveryBusError(
                REGISTRY_INVALID,
                f"project registry has duplicate slug {slug!r}: {target}",
                resume_action="keep one canonical entry per slug in config/projects.json",
            )
        seen.add(slug)
    return document


def _dumps(document: Mapping[str, Any]) -> str:
    return json.dumps(dict(document), ensure_ascii=False, indent=2) + "\n"


def save_registry(path: str | Path, data: Mapping[str, Any]) -> Path:
    """Write the registry atomically (temp file in the same dir, then replace)."""
    target = Path(str(path)).expanduser()
    target.parent.mkdir(parents=True, exist_ok=True)
    handle_fd, tmp_name = tempfile.mkstemp(
        dir=str(target.parent), prefix=".projects-import-", suffix=".tmp"
    )
    try:
        with os.fdopen(handle_fd, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(_dumps(data))
            handle.flush()
            os.fsync(handle.fileno())
        os.chmod(tmp_name, 0o644)
        os.replace(tmp_name, str(target))
    except BaseException:
        try:
            os.unlink(tmp_name)
        except OSError:
            pass
        raise
    return target


# --- merge (AC-004) ---------------------------------------------------------


def _next_index(rows: Sequence[Mapping[str, Any]]) -> int:
    """``max(existing index) + 1`` so an import never reuses a number."""
    highest = 0
    for row in rows:
        try:
            value = int(row.get("index") or 0)
        except (TypeError, ValueError):
            value = 0
        highest = max(highest, value)
    return highest + 1


def merge(
    registry: dict[str, Any],
    scanned: Sequence[Mapping[str, str]],
    overwrite: bool = False,
) -> tuple[list[dict[str, str]], list[dict[str, str]], list[dict[str, str]]]:
    """Fold scanned repos into ``registry["projects"]`` in place.

    Returns ``(new, existing, updated)`` where every scanned entry appears
    exactly once:

    * ``new`` -- appended as a fresh row (index auto-assigned, class
      ``managed``, ``docs_root`` ``docs/beacon``);
    * ``existing`` -- the slug is already registered and ``overwrite`` is off,
      so the row is left untouched (AC-004);
    * ``updated`` -- the slug is already registered and ``overwrite`` is on, so
      only ``repo`` and ``docs_version`` are refreshed; index, title, class,
      aliases and status keep their values.
    """
    rows = registry.get("projects")
    if not isinstance(rows, list):
        raise DeliveryBusError(
            REGISTRY_INVALID,
            "project registry 'projects' must be an array",
            resume_action="repair config/projects.json so that 'projects' is a JSON array",
        )
    by_slug: dict[str, dict[str, Any]] = {
        str(row.get("slug")).strip(): row for row in rows if isinstance(row, dict)
    }
    next_index = _next_index(rows)

    new: list[dict[str, str]] = []
    existing: list[dict[str, str]] = []
    updated: list[dict[str, str]] = []
    for raw_entry in scanned:
        entry = {
            "slug": str(raw_entry.get("slug") or "").strip(),
            "repo": str(raw_entry.get("repo") or "").strip(),
            "docs_version": str(raw_entry.get("docs_version") or "").strip(),
        }
        if not entry["slug"]:
            continue
        row = by_slug.get(entry["slug"])
        if row is not None:
            if not overwrite:
                existing.append(entry)
                continue
            row["repo"] = entry["repo"]
            row["docs_version"] = entry["docs_version"]
            updated.append(entry)
            continue
        fresh: dict[str, Any] = {
            "index": next_index,
            "slug": entry["slug"],
            "title": entry["slug"],
            "class": DEFAULT_PROJECT_CLASS,
            "repo": entry["repo"],
            "aliases": [],
            "dispatchable": True,
            "status": "active",
            "docs_root": DEFAULT_DOCS_ROOT,
            "docs_version": entry["docs_version"],
        }
        next_index += 1
        rows.append(fresh)
        by_slug[entry["slug"]] = fresh
        new.append(entry)
    return new, existing, updated


# --- kernel registration (AC-005) -------------------------------------------


def kernel_command_for(entry: Mapping[str, Any]) -> list[str]:
    """``beacon-go projects register --project <repo> --version <v>``."""
    return [
        *KERNEL_COMMAND,
        "--project",
        str(entry.get("repo") or ""),
        "--version",
        str(entry.get("docs_version") or ""),
    ]


def _default_kernel_runner(argv: Sequence[str]) -> Any:
    """Run the real kernel; the caller treats a non-zero exit as a failure."""
    return subprocess.run(
        list(argv),
        capture_output=True,
        text=True,
        timeout=KERNEL_TIMEOUT_SECONDS,
    )


def _succeeded(result: Any) -> bool:
    """Accept ``True``/``0``/``CompletedProcess(returncode=0)`` as success."""
    if isinstance(result, bool):
        return result
    if isinstance(result, int):
        return result == 0
    returncode = getattr(result, "returncode", None)
    if isinstance(returncode, int):
        return returncode == 0
    return bool(result)


def register_kernel(
    entries: Sequence[Mapping[str, Any]],
    runner: KernelRunner | None = None,
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    """Ask the kernel to learn each entry; never raises, never stops early.

    Returns ``(ok, failed)``.  A missing binary, a timeout, a non-zero exit or
    a runner that raises all land in ``failed`` with a ``reason`` -- AC-005's
    "one failure does not block the rest, and does not block the JSON write".
    """
    run = runner if runner is not None else _default_kernel_runner
    ok: list[dict[str, Any]] = []
    failed: list[dict[str, Any]] = []
    for raw_entry in entries:
        slug = str(raw_entry.get("slug") or "").strip()
        command = kernel_command_for(raw_entry)
        detail: dict[str, Any] = {
            "slug": slug,
            "repo": command[4],
            "docs_version": command[6],
            "command": command,
        }
        if not detail["repo"]:
            failed.append({**detail, "reason": "repo_missing"})
            continue
        try:
            result = run(command)
        except Exception as exc:  # missing binary, timeout, broken runner
            failed.append(
                {**detail, "reason": "kernel_call_failed", "error": str(exc)[:400]}
            )
            continue
        if _succeeded(result):
            ok.append(detail)
            continue
        stderr = str(getattr(result, "stderr", "") or "").strip()
        failed.append(
            {**detail, "reason": "kernel_exit_nonzero", "error": stderr[:400]}
        )
    return ok, failed


def summary_line(counts: Mapping[str, int]) -> str:
    """The count line AC-001/AC-004/AC-005 report in one grep-able string."""
    return (
        f"found={int(counts.get('found', 0))} "
        f"new={int(counts.get('new', 0))} "
        f"existing={int(counts.get('existing', 0))} "
        f"updated={int(counts.get('updated', 0))} "
        f"kernel_failed={int(counts.get('kernel_failed', 0))}"
    )
