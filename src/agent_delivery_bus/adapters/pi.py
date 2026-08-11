"""ExecutorAdapter for the pi agent (``@earendil-works/pi-coding-agent`` CLI).

Driver_pi contract (v0.1.0):
- preflight requires the ``pi`` binary (https://github.com/earendil-works/pi);
  ``pi --version`` is the health probe. Missing/unhealthy CLI ->
  ``pi_cli_unavailable`` / ``pi_version_failed``, never a silent fallback to
  hermes.
- create_task launches ``pi -p --mode json`` in the project workspace with the
  ADB task body as the prompt. Idempotency is kept in a local run ledger
  (``~/.adb/pi/<board>/<idempotency-key>.json``), so the same key reuses the
  same receipt without launching a second session.
- The adapter never passes ``--auto-approve`` / ``--yolo``; auto approval is a
  hard illegal transition (AC-PI-006).
"""

from __future__ import annotations

import json
import re
import uuid
from pathlib import Path
from shutil import which
from typing import Any

from ..errors import CommandFailed, DeliveryBusError
from ..process import CommandRunner
from ..registry import Project
from .spi import as_check


PI_CLI_DEFAULT = "pi"


def board_slug(project_slug: str) -> str:
    slug = re.sub(r"[^a-z0-9-]+", "-", project_slug.lower()).strip("-")
    return f"adb-pi-{slug}"[:64]


class PiRunLedger:
    """Local append-only-ish run receipts keyed by idempotency key.

    pi/omp does not expose an idempotency-key flag, so driver_pi owns the
    idempotency contract on the adapter side.
    """

    def __init__(self, root: Path | None = None):
        self.root = Path(root) if root is not None else Path.home() / ".adb" / "pi"

    def _path(self, board: str, key: str) -> Path:
        safe_key = re.sub(r"[^a-zA-Z0-9_-]+", "_", key)[:120] or "run"
        return self.root / board / f"{safe_key}.json"

    def get(self, board: str, key: str) -> dict[str, Any] | None:
        path = self._path(board, key)
        if not path.is_file():
            return None
        try:
            payload = json.loads(path.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, OSError):
            return None
        return payload if isinstance(payload, dict) else None

    def put(self, board: str, key: str, receipt: dict[str, Any]) -> dict[str, Any]:
        path = self._path(board, key)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(receipt, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        return receipt


class PiExecutorAdapter:
    name = "pi"

    def __init__(
        self,
        runner: CommandRunner | None = None,
        which_command=None,
        ledger: PiRunLedger | None = None,
    ):
        self.runner = runner or CommandRunner()
        self.which_command = which_command or which
        self.ledger = ledger or PiRunLedger()

    def _cli(self) -> str:
        return PI_CLI_DEFAULT

    def _cli_path(self) -> str | None:
        resolved = self.which_command(self._cli())
        return resolved if isinstance(resolved, str) and resolved.strip() else None

    def preflight_checks(self, project: Project, *, stage: str) -> list[dict[str, Any]]:
        del project, stage
        cli = self._cli_path()
        checks = [
            as_check(
                "pi_cli",
                bool(cli),
                reason_code="pi_cli_unavailable",
                resume_action="install oh-my-pi (`omp`) or keep the project on the hermes executor",
            )
        ]
        if not cli:
            return checks
        version = self.runner.run([cli, "--version"], timeout=30)
        checks.append(
            as_check(
                "pi_version",
                version.returncode == 0,
                reason_code="pi_version_failed",
                resume_action="run `omp --version` manually and repair the installation",
                detail={"stderr": version.stderr[-1000:]},
            )
        )
        return checks

    def health(self, *, profile: str = "coding") -> dict[str, Any]:
        del profile
        checks = self.preflight_checks(Project(slug="__health__", title="health", project_class="platform", repo=str(Path.cwd()), aliases=(), dispatchable=False), stage="")
        passed = all(bool(item.get("passed")) for item in checks)
        return {
            "gateway_pass": passed,
            "profile_pass": passed,
            "profiles": [PI_CLI_DEFAULT] if passed else [],
            "checks": checks,
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

    def list_boards(self) -> list[str]:
        if not self.ledger.root.is_dir():
            return []
        return sorted(
            path.name
            for path in self.ledger.root.iterdir()
            if path.is_dir() and not path.name.startswith(".")
        )

    def ensure_board(self, project: Project) -> dict[str, Any]:
        slug = self.board_for(project)
        return {"slug": slug, "default_workdir": project.repo, "created": False}

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
        board = self.board_for(project)
        existing = self.ledger.get(board, idempotency_key)
        if existing is not None:
            return dict(existing)
        if any(token in body for token in ("--auto-approve", "--yolo")):
            raise DeliveryBusError(
                "pi_auto_approve_forbidden",
                "driver_pi refuses task bodies that request auto-approve flags",
                resume_action="remove --auto-approve/--yolo from the bound command",
            )
        workspace = self.workspace_for(project, stage=stage).split(":", 1)[-1]
        task_id = f"pi_{uuid.uuid4().hex[:16]}"
        command = [self._cli(), "-p", "--mode", "json", body]
        result = self.runner.run(command, cwd=workspace, timeout=600)
        if result.returncode != 0:
            raise CommandFailed(
                "pi_dispatch_failed",
                "pi launch failed",
                resume_action="inspect `pi` output, then retry the same idempotency key",
                data={"stderr": result.stderr[-2000:], "stdout": result.stdout[-2000:]},
            )
        session_ref = ""
        if result.stdout.strip():
            try:
                payload = json.loads(result.stdout)
                if isinstance(payload, dict):
                    session_ref = str(
                        payload.get("sessionId")
                        or payload.get("session_id")
                        or payload.get("id")
                        or ""
                    )
            except json.JSONDecodeError:
                session_ref = result.stdout.strip()[-200:]
        failed = (
            result.returncode != 0
            or "Request timed out" in result.stdout
            or '"stopReason":"error"' in result.stdout
            or '"finalError"' in result.stdout
        )
        receipt = {
            "board": board,
            "task_id": task_id,
            "idempotency_key": idempotency_key,
            "project_slug": project.slug,
            "stage": stage,
            "feature": feature,
            "status": "failed" if failed else "done",
            "session_ref": session_ref,
            "assignee": assignee,
            "skills": list(skills or []),
        }
        self.ledger.put(board, idempotency_key, receipt)
        return dict(receipt)

    def show_task(self, board: str, task_id: str) -> dict[str, Any]:
        for receipt in self._receipts(board):
            if receipt.get("task_id") == task_id:
                return dict(receipt)
        raise DeliveryBusError(
            "pi_task_not_found",
            f"No pi run receipt for task {task_id!r} on board {board!r}",
            resume_action="inspect the pi run ledger under ~/.adb/pi",
        )

    def find_by_idempotency(self, board: str, key: str) -> dict[str, Any] | None:
        receipt = self.ledger.get(board, key)
        return dict(receipt) if receipt else None

    def _receipts(self, board: str) -> list[dict[str, Any]]:
        root = self.ledger.root / board
        if not root.is_dir():
            return []
        receipts: list[dict[str, Any]] = []
        for path in sorted(root.glob("*.json")):
            try:
                payload = json.loads(path.read_text(encoding="utf-8"))
            except (json.JSONDecodeError, OSError):
                continue
            if isinstance(payload, dict):
                receipts.append(payload)
        return receipts

    def skills_available(self, skills: list[str]) -> dict[str, list[str]]:
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
