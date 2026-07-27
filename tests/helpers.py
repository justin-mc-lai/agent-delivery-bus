from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from agent_delivery_bus.process import CommandResult
from agent_delivery_bus.registry import Project


def make_project(root: Path, *, slug: str = "demo") -> Project:
    repo = root / slug
    repo.mkdir(parents=True)
    (repo / ".git").mkdir()
    docs = repo / "docs" / "beacon"
    (docs / "v1.0.0").mkdir(parents=True)
    return Project(
        slug=slug,
        title=slug.title(),
        project_class="managed",
        repo=str(repo.resolve()),
        beacon_docs_root=str(docs.resolve()),
        current_docs_version="v1.0.0",
        aliases=(f"{slug}-alias",),
        dispatchable=True,
    )


def write_registry(path: Path, projects: list[Project]) -> Path:
    payload = {
        "schema_version": "1.0",
        "projects": [project.to_dict() for project in projects],
    }
    path.write_text(json.dumps(payload), encoding="utf-8")
    return path


class RecordingRunner:
    def __init__(self, responses: list[CommandResult] | None = None):
        self.responses = list(responses or [])
        self.calls: list[tuple[str, ...]] = []

    def run(self, argv, *, cwd=None, timeout=30.0, check=False):
        args = tuple(str(item) for item in argv)
        self.calls.append(args)
        if self.responses:
            return self.responses.pop(0)
        return CommandResult(args, 0, "{}", "")


class PassingPreflight:
    def run(self, project: Project, *, stage: str) -> dict[str, Any]:
        return {
            "status": "pass",
            "blocked": False,
            "reason_code": "",
            "resume_action": "",
            "project": project.slug,
            "stage": stage,
            "checks": [],
        }


class FakeHermes:
    def __init__(self, *, remote_status: str = "running"):
        self.create_count = 0
        self.remote_status = remote_status
        self.tasks: dict[str, dict[str, Any]] = {}

    def ensure_board(self, project: Project) -> dict[str, Any]:
        return {"slug": f"adb-{project.slug}"}

    def create_task(self, project: Project, *, stage: str, feature: str, body: str, idempotency_key: str):
        self.create_count += 1
        task_id = f"task-{self.create_count}"
        task = {
            "id": task_id,
            "status": self.remote_status,
            "idempotency_key": idempotency_key,
        }
        self.tasks[task_id] = task
        return {"board": f"adb-{project.slug}", "task_id": task_id, "payload": task}

    def show_task(self, board: str, task_id: str):
        task = dict(self.tasks[task_id])
        task["status"] = self.remote_status
        return task

    def find_by_idempotency(self, board: str, key: str):
        return next((task for task in self.tasks.values() if task["idempotency_key"] == key), None)


class FakeBeacon:
    def __init__(self, closure_pass: bool = False):
        self.closure_pass = closure_pass

    def closure(self, project: Project, *, stage: str, feature: str):
        return {"pass": self.closure_pass, "evidence": ["fixture"] if self.closure_pass else []}
