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

    def closure(
        self,
        project: Project,
        *,
        stage: str,
        feature: str,
        dispatch_id: str = "",
        evidence_spec: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Return ``{"pass": bool, ...}`` for stage evidence completeness.

        v1.1 contract: when ``dispatch_id`` is provided, closure MUST reject
        evidence that is not owned by that dispatch (missing/stale manifest).
        ``evidence_spec`` carries the evidence_dir/glob contract emitted in the
        task body.
        """


@runtime_checkable
class ExecutorAdapter(Protocol):
    """Creates and inspects durable worker tasks.

    The executor owns claim/retry/worker lifecycle. Delivery Bus never reaches
    into the executor's private database.

    v1.2 contract — explicit capability declaration instead of exception
    sniffing:

    - ``capabilities`` (class attribute, `dict[str, bool]`) declares which
      optional call parameters the adapter understands:
        * ``task_skills`` — accepts the ``skills`` kwarg on ``create_task``
        * ``task_session`` — accepts the ``session_id`` kwarg on ``create_task``
    - DeliveryService consults ``capabilities`` before passing those kwargs.
      An adapter that needs them must declare them; an undeclared feature is
      treated as unsupported and never probed via ``TypeError``.
    """

    name: str
    capabilities: dict[str, bool]

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
        assignee: str = "coding",
        session_id: str = "",
    ) -> dict[str, Any]:
        """Create or reuse a task. Must return board and task_id.

        ``assignee`` selects the local runner profile (e.g. ``coding`` or
        ``codex``) when the executor backend supports named workers.
        ``session_id`` is the ADB session handle; backends that support
        persistent sessions (e.g. pi --session-id) should pass it through.
        """

    def show_task(self, board: str, task_id: str) -> dict[str, Any]:
        """Fetch current remote task state."""

    def find_by_idempotency(self, board: str, key: str) -> dict[str, Any] | None:
        """Locate a remote task by idempotency key during reconcile."""


@runtime_checkable
class MemoryAdapter(Protocol):
    """Thin memory SPI outside ADB core.

    Concrete backends (agentmemory REST, in-process test store, …) live under
    ``adapters/``. Core registry/storage/approvals must not import backend SDKs.
    """

    name: str

    def health(self) -> dict[str, Any]:
        """Return ``{"ok": bool, ...}`` for adapter readiness."""

    def recall(
        self,
        *,
        project_slug: str,
        query: str,
        limit: int = 8,
        agent_id: str = "",
    ) -> dict[str, Any]:
        """Scoped recall. Must fail closed on cross-project hits.

        Returns ``{"records": [...], "summary": str, "injection_ref": str}``.
        """

    def writeback(
        self,
        *,
        project_slug: str,
        stage: str,
        feature: str,
        dispatch_id: str,
        reason_code: str,
        payload: dict[str, Any] | None = None,
        agent_id: str = "",
    ) -> dict[str, Any]:
        """Persist evidence memory tagged with project_slug scope."""


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


def adapter_capabilities(adapter: Any) -> dict[str, bool]:
    """Return the adapter's declared capabilities (empty dict if undeclared)."""
    caps = getattr(adapter, "capabilities", None)
    return dict(caps) if isinstance(caps, dict) else {}
