"""Project registry: the only routing authority for Delivery Bus."""

from __future__ import annotations

import json
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any

from .errors import DeliveryBusError


ALLOWED_CLASSES = {"platform", "managed", "knowledge"}


@dataclass(frozen=True)
class Project:
    slug: str
    title: str
    project_class: str
    repo: str
    aliases: tuple[str, ...]
    dispatchable: bool
    docs_root: str = ""
    docs_version: str = ""
    metadata: dict[str, Any] = field(default_factory=dict)

    # Backward-compatible aliases used by older local configs and tests.
    @property
    def beacon_docs_root(self) -> str:
        return self.docs_root

    @property
    def current_docs_version(self) -> str:
        return self.docs_version

    def to_dict(self) -> dict[str, Any]:
        payload = {
            "slug": self.slug,
            "title": self.title,
            "class": self.project_class,
            "repo": self.repo,
            "aliases": list(self.aliases),
            "dispatchable": self.dispatchable,
            "docs_root": self.docs_root,
            "docs_version": self.docs_version,
        }
        # Keep legacy keys for older consumers and example Beacon adapters.
        if self.docs_root:
            payload["beacon_docs_root"] = self.docs_root
        if self.docs_version:
            payload["current_docs_version"] = self.docs_version
        if self.metadata:
            payload["metadata"] = dict(self.metadata)
        return payload


class ProjectRegistry:
    def __init__(self, projects: list[Project], *, source: Path, raw: dict[str, Any] | None = None):
        self.source = source
        self.raw = raw or {}
        self._projects = {project.slug: project for project in projects}
        self._aliases: dict[str, str] = {}
        self._paths: dict[str, str] = {}
        for project in projects:
            for alias in (project.slug, *project.aliases):
                key = alias.strip().casefold()
                existing = self._aliases.get(key)
                if existing and existing != project.slug:
                    raise DeliveryBusError(
                        "project_alias_ambiguous",
                        f"Alias {alias!r} maps to both {existing!r} and {project.slug!r}",
                        resume_action="remove or rename the conflicting alias in the project registry",
                    )
                self._aliases[key] = project.slug
            canonical = str(Path(project.repo).expanduser().resolve())
            existing_path = self._paths.get(canonical)
            if existing_path and existing_path != project.slug:
                raise DeliveryBusError(
                    "project_repo_duplicate",
                    f"Repository path {canonical} is registered more than once",
                    resume_action="keep one canonical project record for the repository",
                )
            self._paths[canonical] = project.slug

    @classmethod
    def load(cls, path: str | Path, *, validate_paths: bool = True) -> "ProjectRegistry":
        source = Path(path).expanduser().resolve()
        if not source.is_file():
            raise DeliveryBusError(
                "registry_missing",
                f"Project registry not found: {source}",
                resume_action="copy config/projects.example.json to config/projects.json or pass --config",
            )
        try:
            raw = json.loads(source.read_text(encoding="utf-8"))
        except json.JSONDecodeError as exc:
            raise DeliveryBusError("registry_invalid_json", f"Invalid registry JSON: {exc}") from exc
        rows = raw.get("projects") if isinstance(raw, dict) else None
        if not isinstance(rows, list) or not rows:
            raise DeliveryBusError("registry_empty", "Registry projects must be a non-empty array")

        projects: list[Project] = []
        seen: set[str] = set()
        for index, row in enumerate(rows):
            if not isinstance(row, dict):
                raise DeliveryBusError("registry_project_invalid", f"projects[{index}] must be an object")
            slug = str(row.get("slug") or "").strip()
            project_class = str(row.get("class") or "").strip()
            repo = str(row.get("repo") or "").strip()
            docs_root = str(
                row.get("docs_root")
                or row.get("beacon_docs_root")
                or ""
            ).strip()
            version = str(
                row.get("docs_version")
                or row.get("current_docs_version")
                or ""
            ).strip()
            if not slug or slug in seen:
                raise DeliveryBusError("project_slug_duplicate", f"Missing or duplicate project slug: {slug!r}")
            if project_class not in ALLOWED_CLASSES:
                raise DeliveryBusError(
                    "project_class_invalid",
                    f"Project {slug!r} has invalid class {project_class!r}",
                )
            if not repo:
                raise DeliveryBusError("project_repo_missing", f"Project {slug!r} has no repo")
            canonical_repo = Path(repo).expanduser().resolve()
            if validate_paths and not canonical_repo.is_dir():
                raise DeliveryBusError(
                    "repo_missing",
                    f"Project repository does not exist: {canonical_repo}",
                    resume_action=f"restore the repository or mark {slug} non-dispatchable",
                )
            aliases = row.get("aliases") or []
            if not isinstance(aliases, list) or not all(isinstance(item, str) for item in aliases):
                raise DeliveryBusError("project_aliases_invalid", f"Project {slug!r} aliases must be strings")
            metadata = row.get("metadata") if isinstance(row.get("metadata"), dict) else {}
            seen.add(slug)
            projects.append(
                Project(
                    slug=slug,
                    title=str(row.get("title") or slug),
                    project_class=project_class,
                    repo=str(canonical_repo),
                    aliases=tuple(item.strip() for item in aliases if item.strip()),
                    dispatchable=bool(row.get("dispatchable", True)),
                    docs_root=str(Path(docs_root).expanduser().resolve()) if docs_root else "",
                    docs_version=version,
                    metadata=dict(metadata),
                )
            )
        return cls(projects, source=source, raw=raw if isinstance(raw, dict) else {})

    def list(self, *, dispatchable_only: bool = False) -> list[Project]:
        projects = sorted(self._projects.values(), key=lambda item: item.slug)
        if dispatchable_only:
            projects = [item for item in projects if item.dispatchable]
        return projects

    def resolve(
        self,
        *,
        slug: str | None = None,
        alias: str | None = None,
        path: str | Path | None = None,
    ) -> Project:
        selectors = [slug is not None, alias is not None, path is not None]
        if sum(selectors) != 1:
            raise DeliveryBusError(
                "project_selector_invalid",
                "Exactly one of slug, alias, or path is required",
            )
        resolved_slug = ""
        if slug is not None:
            resolved_slug = slug.strip()
        elif alias is not None:
            resolved_slug = self._aliases.get(alias.strip().casefold(), "")
        else:
            canonical = str(Path(path or "").expanduser().resolve())
            resolved_slug = self._paths.get(canonical, "")
        project = self._projects.get(resolved_slug)
        if project is None:
            raise DeliveryBusError(
                "project_not_found",
                "No registered project matched the selector",
                resume_action="run `adb projects list` and use a registered slug or alias",
            )
        return project
