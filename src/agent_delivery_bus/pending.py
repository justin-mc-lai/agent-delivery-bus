"""Pending approval / awaiting拍板 surfaces (CLI + Feishu channel payload)."""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

from .storage import Storage


def list_issued_approvals(storage: Storage, *, project_slug: str | None = None) -> list[dict[str, Any]]:
    if project_slug:
        rows = storage.conn.execute(
            """
            SELECT approval_id, actor, project_slug, stage, feature, expires_at, state, created_at
            FROM approvals
            WHERE project_slug=? AND state='issued'
            ORDER BY created_at DESC
            """,
            (project_slug,),
        ).fetchall()
    else:
        rows = storage.conn.execute(
            """
            SELECT approval_id, actor, project_slug, stage, feature, expires_at, state, created_at
            FROM approvals
            WHERE state='issued'
            ORDER BY created_at DESC
            """
        ).fetchall()
    return [dict(row) for row in rows]


def list_awaiting_dispatches(storage: Storage, *, project_slug: str | None = None) -> list[dict[str, Any]]:
    rows = storage.list_dispatches(project_slug=project_slug)
    return [row for row in rows if row.get("state") == "awaiting_approval"]


def list_pending_boundaries(storage: Storage) -> list[dict[str, Any]]:
    return storage.list_boundary_proposals(status="pending")


def pending_approval_views(
    storage: Storage,
    *,
    project_slug: str | None = None,
) -> list[dict[str, Any]]:
    """Unified 待拍板 list: awaiting dispatches + issued tokens + boundary pending."""
    views: list[dict[str, Any]] = []
    for row in list_awaiting_dispatches(storage, project_slug=project_slug):
        views.append(
            {
                "kind": "awaiting_dispatch",
                "project": row["project_slug"],
                "stage": row["stage"],
                "feature": row["feature"],
                "dispatch_id": row["dispatch_id"],
                "expires_at": "",
                "actor_hint": "human approver via `adb approve`",
                "state": row["state"],
            }
        )
    now = datetime.now(timezone.utc)
    for row in list_issued_approvals(storage, project_slug=project_slug):
        expires_at = str(row.get("expires_at") or "")
        expired = False
        if expires_at:
            try:
                expired = datetime.fromisoformat(expires_at) <= now
            except ValueError:
                expired = False
        views.append(
            {
                "kind": "issued_token",
                "project": row["project_slug"],
                "stage": row["stage"],
                "feature": row["feature"],
                "approval_id": row["approval_id"],
                "expires_at": expires_at,
                "actor_hint": row.get("actor") or "",
                "state": "expired" if expired else row["state"],
            }
        )
    # Boundary proposals are global (not project-scoped); include whenever no
    # project filter is set, or always as a cross-cutting review queue.
    if project_slug is None:
        for row in list_pending_boundaries(storage):
            views.append(
                {
                    "kind": "boundary_pending",
                    "project": "",
                    "stage": "boundary_review",
                    "feature": row.get("topic") or "",
                    "proposal_id": row["id"],
                    "expires_at": "",
                    "actor_hint": "human via `adb boundary decide`",
                    "state": row.get("status") or "pending",
                    "topic": row.get("topic") or "",
                }
            )
    return views


def render_pending_channel(views: list[dict[str, Any]], *, channel: str = "text") -> dict[str, Any]:
    """Render payload for Hermes 飞书通道 or plain text."""
    channel = (channel or "text").strip().lower()
    lines = ["待人工拍板事项 / Pending approvals"]
    if not views:
        lines.append("(none)")
    for item in views:
        if item.get("kind") == "boundary_pending":
            lines.append(
                f"- [boundary_pending] id={item.get('proposal_id')} "
                f"topic={item.get('topic') or item.get('feature') or '-'} "
                f"actor={item.get('actor_hint') or '-'}"
            )
            continue
        lines.append(
            f"- [{item.get('kind')}] project={item.get('project')} "
            f"stage={item.get('stage')} feature={item.get('feature')} "
            f"expires={item.get('expires_at') or '-'} actor={item.get('actor_hint') or '-'}"
        )
    text = "\n".join(lines)
    if channel in {"feishu", "lark"}:
        # Hermes Feishu channel consumes a compact card-like JSON; we do not
        # speak Feishu OpenAPI here — only produce a renderable payload.
        return {
            "channel": "feishu",
            "msg_type": "interactive",
            "title": "ADB 待拍板",
            "text": text,
            "items": views,
        }
    return {"channel": "text", "text": text, "items": views}
