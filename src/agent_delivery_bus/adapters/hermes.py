"""Example ExecutorAdapter backed by the Hermes Kanban public CLI.

This is an optional reference integration, not part of the scheduling core.
Delivery Bus never opens Hermes private databases.
"""

from __future__ import annotations

import json
import re
from pathlib import Path
from shutil import which
from typing import Any

from ..errors import CommandFailed
from ..process import CommandRunner
from ..registry import Project
from .spi import as_check


def board_slug(project_slug: str) -> str:
    slug = re.sub(r"[^a-z0-9-]+", "-", project_slug.lower()).strip("-")
    return f"adb-{slug}"[:64]


class HermesAdapter:
    name = "hermes"

    def __init__(self, runner: CommandRunner | None = None, which_command=None):
        self.runner = runner or CommandRunner()
        self.which_command = which_command or which

    def preflight_checks(self, project: Project, *, stage: str) -> list[dict[str, Any]]:
        del project, stage
        checks: list[dict[str, Any]] = []
        cli_ok = bool(self.which_command("hermes"))
        checks.append(
            as_check(
                "hermes_cli",
                cli_ok,
                reason_code="hermes_cli_unavailable",
                resume_action="install or repair the Hermes CLI, then rerun preflight",
            )
        )
        health = self.health(profile="coding") if cli_ok else {
            "gateway_pass": False,
            "profile_pass": False,
            "profiles": [],
        }
        checks.append(
            as_check(
                "hermes_gateway",
                bool(health.get("gateway_pass")),
                reason_code="hermes_gateway_unavailable",
                resume_action="run `hermes gateway status` and start/restart the gateway",
                detail=health,
            )
        )
        checks.append(
            as_check(
                "hermes_profile",
                bool(health.get("profile_pass")),
                reason_code="hermes_profile_missing",
                resume_action="create or restore the Hermes `coding` profile",
                detail={"profiles": health.get("profiles", [])},
            )
        )
        return checks

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

    def board_for(self, project: Project) -> str:
        return board_slug(project.slug)

    def workspace_for(self, project: Project, *, stage: str) -> str:
        declared = str((project.metadata or {}).get("workspace_kind") or "").strip().lower()
        if declared == "dir":
            return f"dir:{project.repo}"
        if stage == "implement":
            return f"worktree:{project.repo}"
        return f"dir:{project.repo}"

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
        slug = self.board_for(project)
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
        assignee: str = "coding",
        skills: list[str] | None = None,
    ) -> dict[str, Any]:
        slug = self.board_for(project)
        workspace = self.workspace_for(project, stage=stage)
        task_skills = ["agent-delivery-bus"]
        for skill in skills or []:
            if skill and skill not in task_skills:
                task_skills.append(skill)
        skill_args: list[str] = []
        for skill in task_skills:
            skill_args.extend(["--skill", skill])
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
                assignee,
                "--workspace",
                workspace,
                "--idempotency-key",
                idempotency_key,
                "--max-runtime",
                "2h",
                "--max-retries",
                "2",
                *skill_args,
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

    def skills_available(self, skills: list[str]) -> dict[str, list[str]]:
        """Return missing skill names (searches local Hermes/Codex skill trees)."""
        homes = [Path.home() / ".hermes" / "skills", Path.home() / ".codex" / "skills"]
        installed: set[str] = set()
        for root in homes:
            if not root.is_dir():
                continue
            for skill_dir in root.rglob("*"):
                if skill_dir.is_dir() and (skill_dir / "SKILL.md").is_file():
                    installed.add(skill_dir.name)
        missing = [s for s in skills if s and s not in installed]
        return {"missing": missing, "installed": sorted(installed)}

    def deliver(self, text: str, *, channel_thread: str = "", channel: str = "feishu") -> dict[str, Any]:
        """Post a message back to the originating channel thread via `hermes send`."""
        target = f"{channel}:{channel_thread}" if channel_thread else channel
        result = self.runner.run(["hermes", "send", "--to", target, str(text)], timeout=30)
        if result.returncode != 0:
            raise CommandFailed(
                "hermes_deliver_failed",
                "hermes send failed",
                resume_action="check gateway/platform credentials and rerun reconcile delivery",
                data={"stderr": result.stderr[-2000:]},
            )
        return {"delivered": True, "target": target}

    def stats(self, board: str) -> dict[str, Any]:
        result = self.runner.run(
            ["hermes", "kanban", "--board", board, "stats", "--json"],
            timeout=30,
        )
        if result.returncode != 0:
            raise CommandFailed(
                "hermes_board_stats_failed",
                f"Failed to read Hermes stats for board {board}",
                data={"stderr": result.stderr[-2000:]},
            )
        payload = result.json()
        return payload if isinstance(payload, dict) else {"raw": payload}

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
        if not isinstance(payload, dict):
            return {"raw": payload}
        task = payload.get("task") if isinstance(payload.get("task"), dict) else payload
        if isinstance(payload.get("latest_summary"), dict):
            task = dict(task)
            task["latest_summary"] = payload["latest_summary"]
        return task

    def find_by_idempotency(self, board: str, key: str) -> dict[str, Any] | None:
        for task in self.list_tasks(board):
            if str(task.get("idempotency_key") or "") == key:
                return task
        return None
