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
    """Render payload for Hermes 飞书通道 or plain text.

    Feishu/Lark uses an ADHD-lean card: action first, ≤5 items, overflow noted.
    """
    channel = (channel or "text").strip().lower()
    if channel in {"feishu", "lark"}:
        return _render_feishu_lean(views)
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
    return {"channel": "text", "text": "\n".join(lines), "items": views}


def _render_feishu_lean(views: list[dict[str, Any]]) -> dict[str, Any]:
    """ADHD-friendly Feishu card: next action → ≤5 items → one closer."""
    if not views:
        return {
            "channel": "feishu",
            "msg_type": "interactive",
            "title": "ADB 待拍板",
            "text": "下一步：无待拍板事项\n\n状态：队列为空",
            "items": [],
        }

    show = views[:5]
    overflow = max(0, len(views) - len(show))
    first = show[0]
    first_id = (
        first.get("proposal_id")
        or first.get("approval_id")
        or first.get("dispatch_id")
        or "<id>"
    )
    if first.get("kind") == "boundary_pending":
        action = (
            f"下一步：对第 1 条拍板（约 1 分钟）\n"
            f"adb boundary decide {first_id} --actor you --decision approve --json"
        )
    elif first.get("kind") == "issued_token":
        action = (
            f"下一步：用 token 放行第 1 条（约 1 分钟）\n"
            f"adb approve --token <token>  # approval_id={first_id}"
        )
    else:
        action = f"下一步：处理第 1 条（约 1 分钟）· id={first_id}"

    lines = [
        action,
        "",
        f"状态：待拍板 {len(views)} 条 · 下面只列 {len(show)} 条",
        "",
    ]
    for i, item in enumerate(show, 1):
        if item.get("kind") == "boundary_pending":
            topic = item.get("topic") or item.get("feature") or "-"
            pid = item.get("proposal_id") or "-"
            lines.append(f"{i}. {topic}")
            lines.append(f"   id={pid}")
        else:
            lines.append(
                f"{i}. [{item.get('kind')}] {item.get('project')}/{item.get('feature')}"
            )
            lines.append(
                f"   stage={item.get('stage')} expires={item.get('expires_at') or '-'}"
            )
    if overflow:
        lines.extend(["", f"另有 {overflow} 条 later（需要时再看全文）"])
    lines.extend(["", "下一步：approve 或 reject 上面任意一条 id"])
    return {
        "channel": "feishu",
        "msg_type": "interactive",
        "title": "ADB 待拍板",
        "text": "\n".join(lines),
        "items": show,
        "overflow": overflow,
    }
