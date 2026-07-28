"""Adapter service-provider interfaces for Agent Delivery Bus.

Core depends only on these contracts. Concrete backends such as Hermes and
Beacon live as example adapters and can be replaced without changing the
dispatch ledger, approval FSM, or idempotency rules.
"""

from __future__ import annotations

from typing import Any, Protocol, runtime_checkable

from ..registry import Project


@runtime_checkable
class TruthGateAdapter(Protocol):
    """Decides whether a project is ready and whether stage evidence is complete.

    Knowledge bases, wikis, and inspiration inboxes are intentionally outside
    this contract. A truth gate only answers readiness and closure questions.
    """

    name: str

    def preflight_checks(self, project: Project, *, stage: str) -> list[dict[str, Any]]:
        """Return ordered readiness checks. First failure blocks dispatch."""

    def closure(self, project: Project, *, stage: str, feature: str) -> dict[str, Any]:
        """Return ``{"pass": bool, ...}`` for stage evidence completeness."""


@runtime_checkable
class ExecutorAdapter(Protocol):
    """Creates and inspects durable worker tasks.

    The executor owns claim/retry/worker lifecycle. Delivery Bus never reaches
    into the executor's private database.
    """

    name: str

    def preflight_checks(self, project: Project, *, stage: str) -> list[dict[str, Any]]:
        """Return ordered executor readiness checks."""

    def board_for(self, project: Project) -> str:
        """Stable board/queue identifier for a project."""

    def workspace_for(self, project: Project, *, stage: str) -> str:
        """Workspace hint for the worker (for example ``dir:`` or ``worktree:``)."""

    def ensure_board(self, project: Project) -> dict[str, Any]:
        """Create or reuse the project board/queue."""

    def create_task(
        self,
        project: Project,
        *,
        stage: str,
        feature: str,
        body: str,
        idempotency_key: str,
    ) -> dict[str, Any]:
        """Create or reuse a task. Must return board and task_id."""

    def show_task(self, board: str, task_id: str) -> dict[str, Any]:
        """Fetch current remote task state."""

    def find_by_idempotency(self, board: str, key: str) -> dict[str, Any] | None:
        """Locate a remote task by idempotency key during reconcile."""


def as_check(
    name: str,
    passed: bool,
    *,
    reason_code: str = "",
    resume_action: str = "",
    detail: dict[str, Any] | None = None,
) -> dict[str, Any]:
    return {
        "name": name,
        "passed": bool(passed),
        "reason_code": "" if passed else reason_code,
        "resume_action": "" if passed else resume_action,
        "detail": detail or {},
    }
