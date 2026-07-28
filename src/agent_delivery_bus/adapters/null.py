"""Null adapters for local demos without Hermes or Beacon.

These backends keep the full Delivery Bus control plane working:

- registry resolution
- preflight
- approval
- idempotent dispatch ledger
- reconcile

They intentionally do not talk to external CLIs or databases.
"""

from __future__ import annotations

import json
import uuid
from pathlib import Path
from typing import Any

from ..registry import Project
from .spi import as_check


class NullExecutor:
    """In-memory executor used for Hermes-free demos and unit wiring."""

    name = "null"

    def __init__(self, *, auto_complete: bool = True):
        self.auto_complete = auto_complete
        self.boards: dict[str, dict[str, Any]] = {}
        self.tasks: dict[str, dict[str, Any]] = {}
        self.create_count = 0

    def preflight_checks(self, project: Project, *, stage: str) -> list[dict[str, Any]]:
        del project, stage
        return [
            as_check(
                "null_executor",
                True,
                detail={"mode": "in-memory", "auto_complete": self.auto_complete},
            )
        ]

    def board_for(self, project: Project) -> str:
        return f"adb-{project.slug}"[:64]

    def workspace_for(self, project: Project, *, stage: str) -> str:
        prefix = "worktree" if stage == "implement" else "dir"
        return f"{prefix}:{project.repo}"

    def ensure_board(self, project: Project) -> dict[str, Any]:
        slug = self.board_for(project)
        board = self.boards.get(slug)
        if board is None:
            board = {
                "slug": slug,
                "name": f"ADB · {project.title}",
                "default_workdir": project.repo,
                "created": True,
            }
            self.boards[slug] = board
        return dict(board)

    def create_task(
        self,
        project: Project,
        *,
        stage: str,
        feature: str,
        body: str,
        idempotency_key: str,
    ) -> dict[str, Any]:
        existing = self.find_by_idempotency(self.board_for(project), idempotency_key)
        if existing is not None:
            return {
                "board": self.board_for(project),
                "task_id": str(existing["id"]),
                "payload": dict(existing),
                "duplicate": True,
            }

        self.create_count += 1
        task_id = f"null-{uuid.uuid4().hex[:12]}"
        status = "done" if self.auto_complete else "running"
        task = {
            "id": task_id,
            "board": self.board_for(project),
            "title": f"[{stage}] {project.slug}/{feature}",
            "status": status,
            "state": status,
            "idempotency_key": idempotency_key,
            "workspace": self.workspace_for(project, stage=stage),
            "body": body,
            "project_slug": project.slug,
            "stage": stage,
            "feature": feature,
        }
        self.tasks[task_id] = task

        if self.auto_complete:
            evidence_dir = Path(project.repo) / ".adb" / "evidence" / stage
            evidence_dir.mkdir(parents=True, exist_ok=True)
            evidence_path = evidence_dir / f"{feature}.json"
            evidence_path.write_text(
                json.dumps(
                    {
                        "pass": True,
                        "adapter": "null",
                        "stage": stage,
                        "feature": feature,
                        "task_id": task_id,
                    },
                    ensure_ascii=False,
                    indent=2,
                )
                + "\n",
                encoding="utf-8",
            )
            task["evidence"] = str(evidence_path)

        return {"board": task["board"], "task_id": task_id, "payload": dict(task)}

    def list_boards(self) -> list[dict[str, Any]]:
        return [dict(board) for board in self.boards.values()]

    def stats(self, board: str) -> dict[str, Any]:
        tasks = self.list_tasks(board)
        by_status: dict[str, int] = {}
        for task in tasks:
            status = str(task.get("status") or task.get("state") or "unknown").lower() or "unknown"
            by_status[status] = by_status.get(status, 0) + 1
        return {"by_status": by_status, "by_assignee": {}, "total": len(tasks)}

    def list_tasks(self, board: str) -> list[dict[str, Any]]:

        return [dict(task) for task in self.tasks.values() if task.get("board") == board]

    def show_task(self, board: str, task_id: str) -> dict[str, Any]:

        task = self.tasks.get(task_id)
        if task is None or task.get("board") != board:
            return {"id": task_id, "board": board, "status": "missing"}
        return dict(task)

    def find_by_idempotency(self, board: str, key: str) -> dict[str, Any] | None:
        for task in self.tasks.values():
            if task.get("board") == board and str(task.get("idempotency_key") or "") == key:
                return dict(task)
        return None

    def complete(self, task_id: str) -> dict[str, Any]:
        task = self.tasks.get(task_id)
        if task is None:
            raise KeyError(task_id)
        task["status"] = "done"
        task["state"] = "done"
        return dict(task)


class NullTruthGate:
    """Filesystem-light truth gate for demos without Beacon."""

    name = "null"

    def __init__(self, *, auto_pass: bool = False):
        self.auto_pass = auto_pass

    def preflight_checks(self, project: Project, *, stage: str) -> list[dict[str, Any]]:
        del stage
        checks: list[dict[str, Any]] = []
        if project.docs_root:
            docs = Path(project.docs_root)
            checks.append(
                as_check(
                    "truth_docs_root",
                    docs.is_dir(),
                    reason_code="truth_docs_missing",
                    resume_action="create docs_root or clear it for null-adapter demos",
                )
            )
            if project.docs_version:
                version_ok = (docs / project.docs_version).is_dir()
                checks.append(
                    as_check(
                        "truth_docs_version",
                        version_ok,
                        reason_code="truth_version_mismatch",
                        resume_action=f"create {docs / project.docs_version} or fix docs_version",
                    )
                )
        else:
            checks.append(
                as_check(
                    "null_truth_gate",
                    True,
                    detail={"mode": "null", "docs_root": ""},
                )
            )
        return checks

    def closure(self, project: Project, *, stage: str, feature: str) -> dict[str, Any]:
        if self.auto_pass:
            return {"pass": True, "evidence": ["null-truth-gate:auto-pass"]}
        evidence = Path(project.repo) / ".adb" / "evidence" / stage / f"{feature}.json"
        if evidence.is_file():
            try:
                payload = json.loads(evidence.read_text(encoding="utf-8"))
            except json.JSONDecodeError:
                payload = {}
            passed = payload.get("pass") is True or str(payload.get("status") or "").lower() == "pass"
            return {"pass": passed, "evidence": [str(evidence)], "payload": payload}
        return {
            "pass": False,
            "evidence": [],
            "reason_code": "truth_evidence_incomplete",
            "resume_action": (
                f"write {evidence} with {{\"pass\": true}} or rerun null executor with auto_complete"
            ),
        }
