from __future__ import annotations

import json
import re
from typing import Any

from ..errors import CommandFailed
from ..process import CommandRunner
from ..registry import Project


def board_slug(project_slug: str) -> str:
    slug = re.sub(r"[^a-z0-9-]+", "-", project_slug.lower()).strip("-")
    return f"adb-{slug}"[:64]


class HermesAdapter:
    def __init__(self, runner: CommandRunner | None = None):
        self.runner = runner or CommandRunner()

    def health(self, *, profile: str = "coding") -> dict[str, Any]:
        gateway = self.runner.run(["hermes", "gateway", "status"], timeout=30)
        assignees_result = self.runner.run(
            ["hermes", "kanban", "assignees", "--json"],
            timeout=30,
        )
        try:
            assignees = json.loads(assignees_result.stdout)
        except json.JSONDecodeError:
            assignees = []
        names = {str(item.get("name")) for item in assignees if isinstance(item, dict)}
        return {
            "gateway_pass": gateway.returncode == 0,
            "profile_pass": assignees_result.returncode == 0 and profile in names,
            "profiles": sorted(names),
            "gateway_output": gateway.stdout + gateway.stderr,
        }

    def list_boards(self) -> list[dict[str, Any]]:
        result = self.runner.run(["hermes", "kanban", "boards", "list", "--json"], timeout=30)
        if result.returncode != 0:
            raise CommandFailed(
                "hermes_boards_list_failed",
                "Failed to list Hermes boards",
                data={"stderr": result.stderr[-2000:]},
            )
        payload = result.json()
        return payload if isinstance(payload, list) else []

    def ensure_board(self, project: Project) -> dict[str, Any]:
        slug = board_slug(project.slug)
        for item in self.list_boards():
            if item.get("slug") == slug and not item.get("archived"):
                return item
        result = self.runner.run(
            [
                "hermes",
                "kanban",
                "boards",
                "create",
                slug,
                "--name",
                f"ADB · {project.title}",
                "--description",
                f"Agent Delivery Bus tasks for {project.slug}",
                "--default-workdir",
                project.repo,
            ],
            timeout=30,
        )
        if result.returncode != 0:
            raise CommandFailed(
                "hermes_board_create_failed",
                f"Failed to create Hermes board {slug}",
                data={"stderr": result.stderr[-2000:]},
            )
        return {"slug": slug, "default_workdir": project.repo, "created": True}

    def create_task(
        self,
        project: Project,
        *,
        stage: str,
        feature: str,
        body: str,
        idempotency_key: str,
    ) -> dict[str, Any]:
        slug = board_slug(project.slug)
        workspace = f"worktree:{project.repo}" if stage == "implement" else f"dir:{project.repo}"
        result = self.runner.run(
            [
                "hermes",
                "kanban",
                "--board",
                slug,
                "create",
                f"[{stage}] {project.slug}/{feature}",
                "--body",
                body,
                "--assignee",
                "coding",
                "--workspace",
                workspace,
                "--idempotency-key",
                idempotency_key,
                "--max-runtime",
                "2h",
                "--max-retries",
                "2",
                "--skill",
                "agent-delivery-bus",
                "--created-by",
                "agent-delivery-bus",
                "--json",
            ],
            timeout=60,
        )
        if result.returncode != 0:
            raise CommandFailed(
                "hermes_dispatch_failed",
                "Hermes task creation failed",
                resume_action="inspect Hermes gateway/board diagnostics, then retry the same request",
                data={"stderr": result.stderr[-2000:], "stdout": result.stdout[-2000:]},
            )
        payload = result.json()
        task_id = ""
        if isinstance(payload, dict):
            task_id = str(payload.get("id") or payload.get("task_id") or "")
            if not task_id and isinstance(payload.get("task"), dict):
                task_id = str(payload["task"].get("id") or "")
        if not task_id:
            raise CommandFailed(
                "hermes_receipt_invalid",
                "Hermes create JSON did not include a task id",
                resume_action="query the board by idempotency key before retrying",
                data={"payload": payload},
            )
        return {"board": slug, "task_id": task_id, "payload": payload}

    def list_tasks(self, board: str) -> list[dict[str, Any]]:
        result = self.runner.run(
            ["hermes", "kanban", "--board", board, "list", "--json"],
            timeout=30,
        )
        if result.returncode != 0:
            raise CommandFailed("hermes_task_list_failed", f"Failed to list tasks on {board}")
        payload = result.json()
        return payload if isinstance(payload, list) else []

    def show_task(self, board: str, task_id: str) -> dict[str, Any]:
        result = self.runner.run(
            ["hermes", "kanban", "--board", board, "show", task_id, "--json"],
            timeout=30,
        )
        if result.returncode != 0:
            raise CommandFailed("hermes_task_show_failed", f"Failed to show Hermes task {task_id}")
        payload = result.json()
        return payload if isinstance(payload, dict) else {"raw": payload}

    def find_by_idempotency(self, board: str, key: str) -> dict[str, Any] | None:
        for task in self.list_tasks(board):
            if str(task.get("idempotency_key") or "") == key:
                return task
        return None
