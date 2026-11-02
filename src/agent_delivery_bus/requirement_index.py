"""requirement-index.v1: read-only projection of frozen requirement truth.

Authority split frozen by
``docs/beacon/v0.1.5/features/requirement-index/truth.md``:

* ``truth_stage`` is **read-only**.  It is derived from the frozen beacon
  package (its lifecycle store) and this module offers no way to write it.
  Any attempt carrying ``truth_stage`` fails with ``truth_stage_readonly``.
* ``exec_state`` / ``thread_id`` / ``worker`` / ``blockers`` are ADB-owned and
  live in the ADB SQLite ledger (``Storage``), never in beacon documents.

The generated ``requirement-index.json`` is a **derived view**: it is written
for readers (Router, operators) and is never a source of truth for anything.

Determinism rules (AC-002): requirements are sorted by ``req_id``, ``missing``
is sorted by ``(reason, source, req_id)``, every mapping is serialised with
sorted keys, and the clock is injectable (``generated_at=``), so the same input
produces byte-identical output.  ``content_hash`` deliberately ignores the
volatile timestamps so two runs at different wall-clock times are still
recognisably identical.

``enumerate_sources`` (AC-007) does the discovery half: given project roots it
finds ``docs/beacon/<version>/features/<slug>/truth.md`` packages whose
frontmatter says ``status: frozen`` and turns them into ``RequirementSource``
rows.  Anything it cannot accept (missing root, missing/unreadable truth.md,
unparsable frontmatter, not frozen) lands in ``skipped[]`` instead of aborting
the run.  Discovery reads the *frontmatter* only to decide membership; the
lifecycle stage still comes from the beacon lifecycle store (AC-009):
``enumerate_sources`` asks the kernel (``beacon-go truth list --json``, via an
injectable ``LifecycleProvider``) for each root, so a package that the store
does not know keeps ``truth_stage=unknown`` and shows up in ``missing[]`` as
``lifecycle_source_missing`` rather than being guessed at.  A root the kernel
does not know at all is recorded once in ``skipped[]`` as
``store_not_registered``; its packages are still enumerated.
"""

from __future__ import annotations

import hashlib
import json
import os
import re
import subprocess
import tempfile
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Callable, Iterable, Mapping, Sequence

from .errors import DeliveryBusError

SCHEMA_VERSION = "requirement-index.v1"

#: Byte budget above which the index is split into a summary plus shards.
BUDGET_BYTES = 10240

#: Directory (relative to the index file) holding per-project shards.
SHARD_DIRNAME = "requirements"

UNKNOWN = "unknown"

#: ``truth_stage`` values (frozen enum, beacon-owned).
TRUTH_STAGES = ("plan", "truth", "implement", "qa", "release", UNKNOWN)
#: Stages a beacon lifecycle record is allowed to report.
LIFECYCLE_STAGES = ("plan", "truth", "implement", "qa", "release")

#: ``exec_state`` values (frozen enum, ADB-owned).
EXEC_STATES = ("queued", "running", "reported", "done", "failed", UNKNOWN)

#: The 11 mandatory record fields from the truth data contract.
RECORD_FIELDS = (
    "req_id",
    "project",
    "truth_stage",
    "exec_state",
    "thread_id",
    "worker",
    "blockers",
    "evidence_ref",
    "truth_ref",
    "updated_at",
    "source_status",
)

#: Fields the ADB write interface accepts.
WRITABLE_FIELDS = ("exec_state", "thread_id", "worker", "blockers")

#: Beacon-owned fields; writing them is refused with ``truth_stage_readonly``.
READONLY_FIELDS = (
    "req_id",
    "project",
    "truth_stage",
    "evidence_ref",
    "truth_ref",
    "updated_at",
    "source_status",
)

#: Fields a layered summary keeps for every requirement (AC-006).
SUMMARY_FIELDS = ("req_id", "project", "truth_stage", "exec_state", "truth_ref", "source_status")

# --- reason codes -----------------------------------------------------------
SOURCE_MISSING = "source_missing"
SOURCE_UNREADABLE = "source_unreadable"
SOURCE_INVALID_JSON = "source_invalid_json"
LIFECYCLE_MISSING = "lifecycle_missing"
#: The lifecycle source answered but knows no stage for this slug (AC-009).
LIFECYCLE_SOURCE_MISSING = "lifecycle_source_missing"
DUPLICATE_SOURCE = "duplicate_source"
TRUTH_STAGE_READONLY = "truth_stage_readonly"
EXEC_STATE_INVALID = "exec_state_invalid"
WRITE_NOT_ALLOWED = "write_not_allowed"
INDEX_FILE_MISSING = "index_file_missing"
INDEX_FILE_UNREADABLE = "index_file_unreadable"
INDEX_FILE_INVALID = "index_file_invalid"
#: The adbd exec_state projection (feature worker-report) could not be read.
EXEC_STATE_SOURCE_UNREADABLE = "exec_state_source_unreadable"
EXEC_STATE_SOURCE_INVALID = "exec_state_source_invalid"

#: Where the adbd daemon projects what it observed about running workers
#: (``adbd reconcile --once`` writes this file).
EXEC_STATE_DIRNAME = ".beacon"
EXEC_STATE_FILENAME = "adbd-exec-state.json"

#: Environment override for the injected clock (reproducible CLI runs).
CLOCK_ENV = "ADB_REQUIREMENT_INDEX_NOW"


def req_id_for(project: str, feature: str) -> str:
    """Canonical primary key: ``req:<project_slug>:<feature_slug>``.

    The version is deliberately *not* part of the id so exec_state survives a
    docs version bump (shared naming with the later intake fastpath).
    """
    project_slug = str(project).strip()
    feature_slug = str(feature).strip()
    if not project_slug or not feature_slug:
        raise DeliveryBusError(
            "req_id_invalid",
            "req_id requires both a project slug and a feature slug",
        )
    return f"req:{project_slug}:{feature_slug}"


@dataclass(frozen=True)
class RequirementSource:
    """One frozen beacon package as handed over by the enumerator.

    ``payload_path`` points at the beacon machine projection (JSON) that
    carries the lifecycle record.  ``truth_ref`` / ``evidence_ref`` are the
    pointers declared by the enumerator; they stay usable even when the payload
    itself cannot be read.
    """

    req_id: str
    project: str
    payload_path: str
    truth_ref: str = ""
    evidence_ref: str = ""
    #: Inline projection already read by the caller (the enumerator).  When set
    #: it wins over ``payload_path``; ``payload_path`` then only records where
    #: the projection came from.
    payload: Mapping[str, Any] | None = None


@dataclass(frozen=True)
class IndexBundle:
    """The document for ``requirement-index.json`` plus optional shards."""

    document: dict[str, Any]
    shards: dict[str, dict[str, Any]] = field(default_factory=dict)
    format: str = "full"
    size_bytes: int = 0

    def to_json(self) -> str:
        return dumps(self.document)


# --- helpers ----------------------------------------------------------------


def _utc_now() -> str:
    override = str(os.environ.get(CLOCK_ENV, "")).strip()
    if override:
        return override
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat()


def dumps(document: Mapping[str, Any]) -> str:
    """Serialise deterministically: sorted keys, UTF-8, trailing newline."""
    return json.dumps(dict(document), ensure_ascii=False, sort_keys=True, indent=2) + "\n"


def content_hash(document: Mapping[str, Any]) -> str:
    """Hash the semantic content, ignoring the volatile timestamps."""
    body: dict[str, Any] = {}
    for key in sorted(document):
        if key in ("content_hash", "generated_at"):
            continue
        value = document[key]
        if key == "requirements" and isinstance(value, list):
            value = [
                {item_key: item_value for item_key, item_value in requirement.items() if item_key != "updated_at"}
                if isinstance(requirement, dict)
                else requirement
                for requirement in value
            ]
        body[key] = value
    blob = json.dumps(body, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return "sha256:" + hashlib.sha256(blob.encode("utf-8")).hexdigest()


def choose_format(payload_bytes: int, budget: int = BUDGET_BYTES) -> str:
    """Pure function: serialised bytes -> ``full`` or ``summary``.

    ``<= budget`` stays a single file; anything strictly larger is layered so
    the same input always reaches the same conclusion.
    """
    if budget <= 0:
        raise DeliveryBusError("budget_invalid", f"budget must be positive, got {budget!r}")
    return "summary" if int(payload_bytes) > int(budget) else "full"


def shard_path_for(project: str) -> str:
    """Relative shard path for one project: ``requirements/<project>.json``."""
    slug = re.sub(r"[^0-9a-z._-]+", "-", str(project).strip().casefold()).strip("-._")
    return f"{SHARD_DIRNAME}/{slug or 'unknown'}.json"


def _load_payload(source: RequirementSource) -> tuple[dict[str, Any] | None, str]:
    """Read one beacon projection. Returns ``(payload, reason_code)``."""
    if source.payload is not None:
        return dict(source.payload), ""
    path = Path(source.payload_path).expanduser()
    if not path.exists():
        return None, SOURCE_MISSING
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return None, SOURCE_UNREADABLE
    try:
        payload = json.loads(text)
    except json.JSONDecodeError:
        return None, SOURCE_INVALID_JSON
    if not isinstance(payload, dict):
        return None, SOURCE_INVALID_JSON
    return payload, ""


#: The kernel's lifecycle vocabulary -> this index's five stages
#: (AC-001). Not a guess: the kernel is the authority for its own stage
#: names, and AC-001 fixes the vocabulary this view speaks. Freeze closes
#: the truth stage; Prd is still planning.
_KERNEL_STAGE_MAP = {
    "prd": "plan",
    "think": "truth",
    "freeze": "truth",
    "implement": "implement",
    "qa": "qa",
    "release": "release",
}


def _stage_from_payload(payload: Mapping[str, Any]) -> str:
    """Read the lifecycle stage from the frozen package; never guess."""
    stage = ""
    lifecycle = payload.get("lifecycle")
    if isinstance(lifecycle, Mapping):
        stage = str(lifecycle.get("stage") or "").strip()
    if not stage:
        stage = str(payload.get("truth_stage") or "").strip()
    if not stage:
        return ""
    if stage in LIFECYCLE_STAGES:
        return stage
    return _KERNEL_STAGE_MAP.get(stage.casefold(), "")


# --- enumeration (AC-007) ---------------------------------------------------

#: Where a project root keeps its beacon docs tree.
BEACON_DOCS_ROOT = Path("docs") / "beacon"
FEATURES_DIRNAME = "features"
TRUTH_FILENAME = "truth.md"
EVIDENCE_FILENAME = "evidence.md"
FROZEN_STATUS = "frozen"

#: ``skipped[]`` reasons: every rejected package says why it was rejected.
SKIP_ROOT_MISSING = "root_missing"
SKIP_FEATURES_MISSING = "features_missing"
SKIP_TRUTH_MISSING = "truth_missing"
SKIP_TRUTH_UNREADABLE = "truth_unreadable"
SKIP_FRONTMATTER_INVALID = "frontmatter_invalid"
SKIP_NOT_FROZEN = "not_frozen"
#: Root-level notice: the beacon lifecycle store does not know this project yet
#: (``beacon-go projects register``).  Its packages are still enumerated (AC-009).
SKIP_STORE_NOT_REGISTERED = "store_not_registered"

_FRONTMATTER_BLOCK = re.compile(r"\A---\n(.*?)\n---", re.DOTALL)
_FRONTMATTER_KEY = re.compile(r"[a-z_][a-z0-9_]*\Z")

# --- lifecycle source (AC-009) ----------------------------------------------

#: Kernel command whose answer is the *only* legitimate ``truth_stage`` source.
LIFECYCLE_COMMAND = ("beacon-go", "truth", "list")
LIFECYCLE_TIMEOUT_SECONDS = 30

#: ``root -> {slug: {"stage": ..., "status": ...}}``.  Injected so tests never
#: need the real kernel binary.
LifecycleProvider = Callable[[Path], Mapping[str, Mapping[str, str]]]


def default_lifecycle_provider(root: Path) -> dict[str, dict[str, str]]:
    """Ask the beacon kernel for every slug's real lifecycle stage.

    Runs ``beacon-go truth list --project <root> --json`` and maps
    ``features[].slug`` to ``{"stage": lifecycle_stage, "status":
    lifecycle_status}``.  Fails closed: a missing binary, a non-zero exit, a
    timeout or unparsable JSON all yield ``{}`` (the caller then records
    ``store_not_registered`` / ``lifecycle_source_missing``).  Never raises.
    """
    try:
        completed = subprocess.run(
            [*LIFECYCLE_COMMAND, "--project", str(root), "--json"],
            capture_output=True,
            text=True,
            timeout=LIFECYCLE_TIMEOUT_SECONDS,
        )
    except (OSError, subprocess.SubprocessError):  # missing binary, timeout
        return {}
    if completed.returncode != 0:
        return {}
    try:
        document = json.loads(completed.stdout or "")
    except (TypeError, ValueError):
        return {}
    if not isinstance(document, Mapping):
        return {}
    features = document.get("features")
    if isinstance(features, (str, bytes)) or not isinstance(features, Sequence):
        return {}
    stages: dict[str, dict[str, str]] = {}
    for feature in features:
        if not isinstance(feature, Mapping):
            continue
        slug = str(feature.get("slug") or "").strip()
        if not slug:
            continue
        stages[slug] = {
            "stage": str(feature.get("lifecycle_stage") or "").strip(),
            "status": str(feature.get("lifecycle_status") or "").strip(),
        }
    return stages


def parse_frontmatter(text: str) -> dict[str, str] | None:
    """Parse the flat YAML-ish frontmatter beacon writes; ``None`` if unparsable.

    Stdlib only and strict on purpose: a package whose frontmatter cannot be
    read must be skipped *and recorded*, never guessed at.  Indented keys (a
    nested ``canonical_refs:`` block) are kept flat; the enumerator only needs
    scalar facts such as ``status``.
    """
    normalised = text.replace("\r\n", "\n").replace("\r", "\n")
    match = _FRONTMATTER_BLOCK.match(normalised)
    if match is None:
        return None
    block = match.group(1)
    if not block.strip():
        return None
    fields: dict[str, str] = {}
    for raw in block.splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        key, separator, value = line.partition(":")
        key = key.strip()
        if not separator:
            # YAML list item or continuation line (e.g. a `provenance:`
            # block). It carries no scalar fact this enumerator needs.
            continue
        if _FRONTMATTER_KEY.match(key) is None:
            # Nested/indented key: kept flat, and not a scalar fact.
            continue
        fields[key] = value.strip().strip('"').strip("'")
    return fields or None


def _version_sort_key(name: str) -> tuple[tuple[int, ...], str]:
    """Numeric-aware ordering so ``v0.1.10`` beats ``v0.1.9``."""
    return (tuple(int(part) for part in re.findall(r"\d+", name)), name)


def resolve_version(root: str | Path, version: str | None = None) -> str:
    """Explicit version wins; otherwise the highest ``docs/beacon/<v>`` with ``features/``.

    Returns ``""`` when the root has no beacon docs tree at all.
    """
    explicit = str(version or "").strip()
    if explicit:
        return explicit
    base = Path(root).expanduser() / BEACON_DOCS_ROOT
    if not base.is_dir():
        return ""
    try:
        children = list(base.iterdir())
    except OSError:  # an unreadable docs tree is handled by the caller as a skip
        return ""
    candidates = [
        child.name
        for child in children
        if child.is_dir() and (child / FEATURES_DIRNAME).is_dir()
    ]
    if not candidates:
        return ""
    return max(candidates, key=_version_sort_key)


def _relative_path(path: Path, root: Path) -> str:
    """POSIX-relative pointer when possible, absolute otherwise."""
    try:
        return path.relative_to(root).as_posix()
    except ValueError:
        return path.as_posix()


def _source_for(
    root: Path,
    package: Path,
    truth_path: Path,
    frontmatter: Mapping[str, str],
    lifecycle: Mapping[str, str] | None = None,
) -> RequirementSource:
    """One frozen package as a ``RequirementSource``.

    ``lifecycle`` is the kernel's answer for this slug (or ``{}``).  It is
    carried verbatim in the payload; the stage is never inferred from the
    frontmatter or from the directory name (AC-009).
    """
    evidence_path = package / EVIDENCE_FILENAME
    project = root.name
    payload: dict[str, Any] = {
        "lifecycle": dict(lifecycle or {}),
        "truth_ref": _relative_path(truth_path, root),
        "evidence_ref": (
            _relative_path(evidence_path, root) if evidence_path.is_file() else ""
        ),
        # Frontmatter facts are safe to carry; the lifecycle stage is not —
        # ``status: frozen`` proves the package is frozen, not how far it ran.
        "frontmatter": dict(frontmatter),
    }
    return RequirementSource(
        req_id=req_id_for(project, package.name),
        project=project,
        payload_path=str(truth_path),
        truth_ref=str(payload["truth_ref"]),
        evidence_ref=str(payload["evidence_ref"]),
        payload=payload,
    )


def _stages_for_root(
    provider: LifecycleProvider,
    root: Path,
) -> dict[str, dict[str, str]]:
    """One root's lifecycle projection; a broken provider degrades to ``{}``."""
    try:
        raw = provider(root)
    except Exception:  # a provider must never break the whole run
        return {}
    if not isinstance(raw, Mapping):
        return {}
    stages: dict[str, dict[str, str]] = {}
    for slug, value in raw.items():
        key = str(slug).strip()
        if not key or not isinstance(value, Mapping):
            continue
        stages[key] = {str(name): str(item) for name, item in value.items()}
    return stages


def _feature_packages(features_dir: Path) -> list[Path]:
    """Directories under ``features/``; an unreadable tree yields none."""
    try:
        children = list(features_dir.iterdir())
    except OSError:  # pragma: no cover - permissions on the docs tree
        return []
    return sorted(child for child in children if child.is_dir())


def enumerate_sources(
    project_roots: Iterable[str | Path],
    version: str | None = None,
    lifecycle_provider: LifecycleProvider | None = None,
) -> tuple[list[RequirementSource], list[dict[str, Any]]]:
    """Discover frozen beacon packages under the given project roots.

    Returns ``(sources, skipped)``.  ``skipped`` entries are
    ``{"source": <path>, "reason": <code>}`` and each names one of
    ``root_missing`` / ``features_missing`` / ``truth_missing`` /
    ``truth_unreadable`` / ``frontmatter_invalid`` / ``not_frozen`` /
    ``store_not_registered``.

    ``lifecycle_provider`` supplies each root's ``{slug: {stage, status}}``
    (AC-009); it defaults to :func:`default_lifecycle_provider` (the kernel).  A
    root the provider does not know is recorded once as
    ``store_not_registered`` and its packages are still enumerated, so the gap
    is visible in ``missing[]`` instead of silently guessing a stage.

    A bad root is a recorded skip, never an exception, and the same root passed
    twice is enumerated once.
    """
    provider = (
        lifecycle_provider if lifecycle_provider is not None else default_lifecycle_provider
    )
    sources: list[RequirementSource] = []
    skipped: list[dict[str, Any]] = []
    seen_roots: set[str] = set()

    for raw_root in project_roots:
        if not str(raw_root).strip():
            skipped.append({"source": str(raw_root), "reason": SKIP_ROOT_MISSING})
            continue
        root = Path(str(raw_root)).expanduser()
        try:
            resolved = root.resolve()
        except OSError:  # pragma: no cover - unreadable path components
            resolved = root
        marker = str(resolved)
        if marker in seen_roots:  # the same root twice counts once
            continue
        seen_roots.add(marker)
        if not resolved.is_dir() or not resolved.name:
            skipped.append({"source": str(root), "reason": SKIP_ROOT_MISSING})
            continue

        chosen = resolve_version(resolved, version)
        features_dir = resolved / BEACON_DOCS_ROOT / chosen / FEATURES_DIRNAME if chosen else None
        if features_dir is None or not features_dir.is_dir():
            skipped.append(
                {
                    "source": str(resolved / BEACON_DOCS_ROOT / chosen) if chosen else str(resolved),
                    "reason": SKIP_FEATURES_MISSING,
                }
            )
            continue

        # AC-009: the stage comes from the kernel store, once per root.  When
        # the store knows nothing this is a root-level notice, not a reason to
        # drop the packages (they still land in ``missing[]`` per record).
        stages = _stages_for_root(provider, resolved)
        if not stages:
            skipped.append({"source": str(resolved), "reason": SKIP_STORE_NOT_REGISTERED})

        for package in _feature_packages(features_dir):
            truth_path = package / TRUTH_FILENAME
            if not truth_path.is_file():
                skipped.append({"source": str(package), "reason": SKIP_TRUTH_MISSING})
                continue
            try:
                text = truth_path.read_text(encoding="utf-8")
            except (OSError, UnicodeDecodeError):
                skipped.append({"source": str(truth_path), "reason": SKIP_TRUTH_UNREADABLE})
                continue
            frontmatter = parse_frontmatter(text)
            if frontmatter is None:
                skipped.append({"source": str(truth_path), "reason": SKIP_FRONTMATTER_INVALID})
                continue
            if str(frontmatter.get("status") or "").strip().casefold() != FROZEN_STATUS:
                skipped.append({"source": str(truth_path), "reason": SKIP_NOT_FROZEN})
                continue
            sources.append(
                _source_for(
                    resolved,
                    package,
                    truth_path,
                    frontmatter,
                    stages.get(package.name, {}),
                )
            )

    sources.sort(key=lambda item: (item.req_id, item.payload_path))
    skipped.sort(key=lambda item: (item["reason"], item["source"]))
    return sources, skipped


# --- record + document building --------------------------------------------


def _build_record(
    source: RequirementSource,
    payload: Mapping[str, Any] | None,
    exec_row: Mapping[str, Any],
    stamp: str,
    reason: str,
) -> dict[str, Any]:
    record: dict[str, Any] = {
        "req_id": source.req_id,
        "project": source.project,
        "truth_stage": UNKNOWN,
        "exec_state": str(exec_row.get("exec_state") or UNKNOWN),
        "thread_id": str(exec_row.get("thread_id") or ""),
        "worker": str(exec_row.get("worker") or ""),
        "blockers": [str(item) for item in (exec_row.get("blockers") or [])],
        "evidence_ref": source.evidence_ref,
        "truth_ref": source.truth_ref,
        "updated_at": stamp,
        "source_status": UNKNOWN if reason else "ok",
    }
    if payload is not None:
        record["truth_ref"] = str(payload.get("truth_ref") or source.truth_ref)
        record["evidence_ref"] = str(payload.get("evidence_ref") or source.evidence_ref)
        stage = _stage_from_payload(payload)
        if stage:
            record["truth_stage"] = stage
    return record


def build_index(
    sources: Iterable[RequirementSource],
    *,
    exec_rows: Mapping[str, Mapping[str, Any]] | None = None,
    generated_at: str | None = None,
    budget: int = BUDGET_BYTES,
) -> IndexBundle:
    """Project frozen truth + ADB exec state into one deterministic document."""
    stamp = str(generated_at or _utc_now())
    rows = dict(exec_rows or {})

    ordered = sorted(sources, key=lambda item: (item.req_id, str(item.payload_path)))
    seen: dict[str, str] = {}
    records: list[dict[str, Any]] = []
    missing: list[dict[str, Any]] = []

    for source in ordered:
        if not str(source.req_id).strip():
            raise DeliveryBusError("req_id_invalid", "every source needs a req_id")
        if source.req_id in seen:
            # Never emit duplicate rows; say where the duplicate came from.
            missing.append(
                {
                    "source": str(source.payload_path),
                    "reason": DUPLICATE_SOURCE,
                    "req_id": source.req_id,
                }
            )
            continue
        seen[source.req_id] = str(source.payload_path)

        payload, reason = _load_payload(source)
        if not reason and not _stage_from_payload(payload or {}):
            # A payload we could read but that yields no stage means the
            # lifecycle source itself had nothing to say (AC-009); only a
            # payload that could not be loaded keeps ``lifecycle_missing``.
            reason = LIFECYCLE_SOURCE_MISSING if payload is not None else LIFECYCLE_MISSING
        exec_row = rows.get(source.req_id) or {}
        records.append(_build_record(source, payload, exec_row, stamp, reason))
        if reason:
            missing.append(
                {
                    "source": str(source.payload_path),
                    "reason": reason,
                    "req_id": source.req_id,
                }
            )

    records.sort(key=lambda item: item["req_id"])
    missing.sort(key=lambda item: (item["reason"], item["source"], item["req_id"]))

    document: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "format": "full",
        "budget_bytes": int(budget),
        "generated_at": stamp,
        "generated_from": sorted({str(source.payload_path) for source in ordered}),
        "missing": missing,
        "requirements": records,
    }
    document["content_hash"] = content_hash(document)
    size_bytes = len(dumps(document).encode("utf-8"))

    chosen = choose_format(size_bytes, budget)
    if chosen == "full":
        return IndexBundle(document=document, format="full", size_bytes=size_bytes)
    return _layer(document, records, stamp, size_bytes)


def _layer(
    full_document: Mapping[str, Any],
    records: Sequence[Mapping[str, Any]],
    stamp: str,
    size_bytes: int,
) -> IndexBundle:
    """Split into summary + ``requirements/<project>.json`` shards (AC-006)."""
    shards: dict[str, dict[str, Any]] = {}
    shard_by_project: dict[str, str] = {}
    for record in records:
        project = str(record["project"])
        relative = shard_path_for(project)
        previous = shard_by_project.get(project)
        if previous is not None and previous != relative:
            raise DeliveryBusError(
                "shard_name_collision",
                f"project {project!r} maps to more than one shard path",
            )
        shard_by_project[project] = relative
        bucket = shards.setdefault(
            relative,
            {
                "schema_version": SCHEMA_VERSION,
                "format": "shard",
                "project": project,
                "generated_at": stamp,
                "requirements": [],
            },
        )
        bucket["requirements"].append(dict(record))

    owned: dict[str, str] = {}
    for project, relative in sorted(shard_by_project.items()):
        owner = owned.get(relative)
        if owner is not None and owner != project:
            raise DeliveryBusError(
                "shard_name_collision",
                f"projects {owner!r} and {project!r} both map to {relative!r}",
            )
        owned[relative] = project

    summary_records: list[dict[str, Any]] = []
    for record in records:
        entry = {key: record[key] for key in SUMMARY_FIELDS if key in record}
        # ``shard`` is how a summary row finds its full record again (AC-004).
        entry["shard"] = shard_by_project[str(record["project"])]
        summary_records.append(entry)

    summary: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "format": "summary",
        "budget_bytes": int(full_document["budget_bytes"]),
        "generated_at": stamp,
        "generated_from": list(full_document["generated_from"]),
        "missing": list(full_document["missing"]),
        "requirements": summary_records,
    }
    summary["content_hash"] = content_hash(summary)

    stamped_shards: dict[str, dict[str, Any]] = {}
    for relative, shard in sorted(shards.items()):
        document = dict(shard)
        document["content_hash"] = content_hash(document)
        stamped_shards[relative] = document

    return IndexBundle(
        document=summary,
        shards=stamped_shards,
        format="summary",
        size_bytes=size_bytes,
    )


# --- writing ----------------------------------------------------------------


def _write_atomic(path: Path, text: str) -> bool:
    """Write ``text`` to ``path`` atomically; skip when already identical."""
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.is_file():
        try:
            if path.read_text(encoding="utf-8") == text:
                return False
        except OSError:
            pass
    handle_fd, tmp_name = tempfile.mkstemp(
        dir=str(path.parent), prefix=".requirement-index-", suffix=".tmp"
    )
    try:
        with os.fdopen(handle_fd, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(text)
            handle.flush()
            os.fsync(handle.fileno())
        os.chmod(tmp_name, 0o644)
        os.replace(tmp_name, str(path))
    except BaseException:
        try:
            os.unlink(tmp_name)
        except OSError:
            pass
        raise
    return True


def write_index(bundle: IndexBundle, out_path: str | Path) -> dict[str, Any]:
    """Write the summary/full document plus shards; shards land before it."""
    out = Path(out_path).expanduser()
    written: list[str] = []
    skipped: list[str] = []
    for relative, document in sorted(bundle.shards.items()):
        target = out.parent / relative
        if _write_atomic(target, dumps(document)):
            written.append(str(target))
        else:
            skipped.append(str(target))
    if _write_atomic(out, bundle.to_json()):
        written.append(str(out))
    else:
        skipped.append(str(out))
    return {
        "path": str(out),
        "format": bundle.format,
        "shards": sorted(bundle.shards),
        "size_bytes": bundle.size_bytes,
        "written": written,
        "skipped": skipped,
    }


def describe_index(out_path: str | Path) -> dict[str, Any]:
    """Read one generated index file and summarise it (``adb index status``).

    Fails closed: a missing file is ``index_file_missing``, an unreadable one
    ``index_file_unreadable`` and a non-JSON one ``index_file_invalid``.
    """
    path = Path(out_path).expanduser()
    if not path.is_file():
        raise DeliveryBusError(
            INDEX_FILE_MISSING,
            f"requirement index not found: {path}",
            resume_action="run `adb index generate --projects <roots> --out <path>` first",
        )
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError) as exc:
        raise DeliveryBusError(
            INDEX_FILE_UNREADABLE,
            f"requirement index unreadable: {path} ({exc})",
            resume_action="check file permissions, then regenerate the index",
        ) from None
    try:
        document = json.loads(text)
    except json.JSONDecodeError as exc:
        raise DeliveryBusError(
            INDEX_FILE_INVALID,
            f"requirement index is not JSON: {path} ({exc})",
            resume_action="regenerate the file with `adb index generate`",
        ) from None
    if not isinstance(document, dict):
        raise DeliveryBusError(
            INDEX_FILE_INVALID,
            f"requirement index is not a JSON object: {path}",
            resume_action="regenerate the file with `adb index generate`",
        )
    requirements = document.get("requirements")
    missing = document.get("missing")
    return {
        "path": str(path),
        "schema_version": str(document.get("schema_version") or ""),
        "format": str(document.get("format") or "full"),
        "count": len(requirements) if isinstance(requirements, list) else 0,
        "missing_count": len(missing) if isinstance(missing, list) else 0,
        "generated_at": str(document.get("generated_at") or ""),
        "content_hash": str(document.get("content_hash") or ""),
    }


def generate_index(
    sources: Iterable[RequirementSource],
    *,
    storage: Any = None,
    exec_rows: Mapping[str, Mapping[str, Any]] | None = None,
    exec_state_paths: Iterable[str | Path] | None = None,
    generated_at: str | None = None,
    budget: int = BUDGET_BYTES,
) -> IndexBundle:
    """Convenience wrapper: collect exec state from every source, then build.

    Precedence per ``req_id`` (worker-report AC-004): the ADB SQLite ledger is
    the baseline, then explicit ``exec_rows``, then each
    ``.beacon/adbd-exec-state.json`` -- so a receipt the daemon just observed
    wins over whatever the ledger last recorded, and a requirement with no row
    anywhere stays ``unknown``.
    """
    rows: dict[str, Mapping[str, Any]] = {}
    if storage is not None:
        rows = {
            str(row["req_id"]): row for row in storage.list_requirement_exec_states()
        }
    for req_id, row in (exec_rows or {}).items():
        rows[str(req_id)] = row
    # Sorted and de-duplicated so two runs over the same roots merge in the same
    # order: the generated index has to stay byte-identical (AC-002).
    for path in sorted({str(Path(item).expanduser()) for item in (exec_state_paths or ())}):
        rows.update(load_exec_state_file(path))
    return build_index(
        sources,
        exec_rows=rows,
        generated_at=generated_at,
        budget=budget,
    )


def exec_state_path_for(root: str | Path) -> Path:
    """The daemon's exec_state projection inside one project root."""
    return Path(root).expanduser() / EXEC_STATE_DIRNAME / EXEC_STATE_FILENAME


def load_exec_state_file(path: str | Path) -> dict[str, dict[str, Any]]:
    """Read ``.beacon/adbd-exec-state.json`` into ADB-shaped exec rows.

    Returns ``{req_id: {req_id, exec_state, thread_id, worker, blockers}}``.
    A missing (or empty) file is an empty mapping -- AC-004's "文件不存在时行为
    不变" -- because a project that never ran ``adbd reconcile`` owes the index
    nothing. An unreadable or malformed file is a loud ``DeliveryBusError``
    instead: silently dropping those rows would make the index claim
    ``unknown`` for work that demonstrably finished.
    """
    source = Path(path).expanduser()
    if not source.is_file():
        return {}
    try:
        text = source.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError) as exc:
        raise DeliveryBusError(
            EXEC_STATE_SOURCE_UNREADABLE,
            f"exec_state source unreadable: {source} ({exc})",
            resume_action="check permissions on .beacon/adbd-exec-state.json, then rerun adbd reconcile --once",
        ) from None
    if not text.strip():
        return {}
    try:
        document = json.loads(text)
    except json.JSONDecodeError as exc:
        raise DeliveryBusError(
            EXEC_STATE_SOURCE_INVALID,
            f"exec_state source is not JSON: {source} ({exc})",
            resume_action="regenerate it with `adbd reconcile --once`",
        ) from None
    if not isinstance(document, Mapping):
        raise DeliveryBusError(
            EXEC_STATE_SOURCE_INVALID,
            f"exec_state source is not a JSON object: {source}",
            resume_action="regenerate it with `adbd reconcile --once`",
        )

    rows: dict[str, dict[str, Any]] = {}
    for raw_req_id, entry in document.items():
        req_id = str(raw_req_id).strip()
        if not req_id:
            continue
        if not isinstance(entry, Mapping):
            raise DeliveryBusError(
                EXEC_STATE_SOURCE_INVALID,
                f"exec_state entry for {req_id} is not an object: {source}",
                resume_action="regenerate it with `adbd reconcile --once`",
            )
        state = str(entry.get("exec_state") or UNKNOWN)
        if state not in EXEC_STATES:
            raise DeliveryBusError(
                EXEC_STATE_INVALID,
                f"Unsupported exec_state {state!r} for {req_id} in {source}",
                resume_action="use one of " + ", ".join(EXEC_STATES),
            )
        rows[req_id] = {
            "req_id": req_id,
            "exec_state": state,
            # The daemon records the intake thread; the index calls it thread_id.
            "thread_id": str(entry.get("thread") or entry.get("thread_id") or ""),
            "worker": str(entry.get("worker") or ""),
            "blockers": [str(item) for item in (entry.get("blockers") or [])],
        }
    return rows


# --- writable half (ADB-owned) ---------------------------------------------


def set_exec_state(
    storage: Any,
    req_id: str,
    changes: Mapping[str, Any],
    *,
    actor: str = "",
    reason_code: str = "",
) -> dict[str, Any]:
    """Write ADB-owned fields for one requirement.

    ``truth_stage`` (and every other beacon-owned field) is refused with
    ``truth_stage_readonly``; unknown fields are refused with
    ``write_not_allowed``.  Nothing here can touch the generated index.
    """
    payload = dict(changes or {})
    for key in sorted(payload):
        if key in READONLY_FIELDS:
            raise DeliveryBusError(
                TRUTH_STAGE_READONLY,
                f"{key} is read-only: truth_stage only changes via beacon truth change + refreeze",
                resume_action="edit the beacon package and refreeze; never write truth from ADB",
            )
        if key not in WRITABLE_FIELDS:
            raise DeliveryBusError(
                WRITE_NOT_ALLOWED,
                f"{key} is not an ADB-writable requirement field",
                resume_action="write one of: " + ", ".join(WRITABLE_FIELDS),
            )

    exec_state = payload.get("exec_state")
    if exec_state is not None and str(exec_state) not in EXEC_STATES:
        raise DeliveryBusError(
            EXEC_STATE_INVALID,
            f"Unsupported exec_state {exec_state!r}",
            resume_action="use one of " + ", ".join(EXEC_STATES),
        )

    blockers = payload.get("blockers")
    if blockers is not None and (
        isinstance(blockers, (str, bytes)) or not isinstance(blockers, Sequence)
    ):
        raise DeliveryBusError(
            EXEC_STATE_INVALID,
            "blockers must be a list of strings",
            resume_action="pass blockers as a JSON array of strings",
        )

    return storage.set_requirement_exec_state(
        req_id,
        exec_state=None if exec_state is None else str(exec_state),
        thread_id=None if "thread_id" not in payload else str(payload.get("thread_id") or ""),
        worker=None if "worker" not in payload else str(payload.get("worker") or ""),
        blockers=None if blockers is None else [str(item) for item in blockers],
        actor=actor,
        reason_code=reason_code,
    )
