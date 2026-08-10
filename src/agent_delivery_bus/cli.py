from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

from .adapters.factory import AdapterResolver
from .approvals import ApprovalService
from .assign import AssignmentScorer
from .boundary import BoundaryService, hermes_boundary_tick_script
from .errors import DeliveryBusError
from .install import install_skill
from .intent import IntentParser
from .pending import pending_approval_views, render_pending_channel
from .preflight import Preflight
from .registry import ALLOWED_CLASSES, ProjectRegistry
from .schedule import ScheduleService, hermes_cron_tick_script
from .service import DeliveryService
from .storage import Storage
from .workflows import (
    PRESET_SOURCE as wf_presets,
    get_workflow,
    install_workflow,
    remove_workflow,
    workflow_names,
)


ROOT = Path(__file__).resolve().parents[2]
_LOCAL_CONFIG = ROOT / "config" / "projects.local.json"
_EXAMPLE_CONFIG = ROOT / "config" / "projects.json"
DEFAULT_CONFIG = _LOCAL_CONFIG if _LOCAL_CONFIG.is_file() else _EXAMPLE_CONFIG
DEFAULT_DB = ROOT / "data" / "agent-delivery-bus.sqlite3"


def envelope(
    *,
    status: str,
    blocked: bool = False,
    reason_code: str = "",
    resume_action: str = "",
    data: Any = None,
) -> dict[str, Any]:
    return {
        "schema_version": "1.0",
        "status": status,
        "blocked": blocked,
        "reason_code": reason_code,
        "resume_action": resume_action,
        "data": data,
    }


def emit(payload: dict[str, Any], *, as_json: bool) -> None:
    if as_json:
        print(json.dumps(payload, ensure_ascii=False, indent=2))
        return
    print(f"{payload['status']}: {payload.get('reason_code') or 'ok'}")
    data = payload.get("data")
    if isinstance(data, dict) and isinstance(data.get("text"), str):
        print(data["text"])
        return
    if data is not None:
        print(json.dumps(data, ensure_ascii=False, indent=2))


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="adb", description="Agent Delivery Bus")
    parser.add_argument("--config", default=str(DEFAULT_CONFIG))
    parser.add_argument("--db", default=str(DEFAULT_DB))
    sub = parser.add_subparsers(dest="command", required=True)

    projects = sub.add_parser("projects")
    projects_sub = projects.add_subparsers(dest="projects_command", required=True)
    projects_list = projects_sub.add_parser("list")
    projects_list.add_argument("--dispatchable-only", action="store_true")
    projects_list.add_argument("--numbered", action="store_true", help="prefix rows with fixed [#N] index")
    projects_list.add_argument("--json", action="store_true")
    projects_resolve = projects_sub.add_parser("resolve")
    group = projects_resolve.add_mutually_exclusive_group(required=True)
    group.add_argument("--slug")
    group.add_argument("--alias")
    group.add_argument("--path")
    group.add_argument("--index", type=int)
    projects_resolve.add_argument("--json", action="store_true")
    projects_register = projects_sub.add_parser("register", help="register a new project (index auto-assigned)")
    projects_register.add_argument("--slug", required=True)
    projects_register.add_argument("--title", default="")
    projects_register.add_argument(
        "--class", dest="project_class", required=True, choices=sorted(ALLOWED_CLASSES)
    )
    projects_register.add_argument("--repo", required=True)
    projects_register.add_argument("--aliases", default="", help="comma-separated aliases")
    projects_register.add_argument("--docs-root", default="")
    projects_register.add_argument("--docs-version", default="")
    projects_register.add_argument("--truth-gate", default="")
    projects_register.add_argument("--executor", default="")
    projects_register.add_argument("--binding-profile", default="")
    projects_register.add_argument("--json", action="store_true")
    projects_delete = projects_sub.add_parser("delete", help="soft-delete (archive) a project by index/slug")
    projects_delete.add_argument("target")
    projects_delete.add_argument("--yes", action="store_true", help="required confirmation for deletion")
    projects_delete.add_argument("--json", action="store_true")
    projects_restore = projects_sub.add_parser("restore", help="restore an archived project by index/slug")
    projects_restore.add_argument("target")
    projects_restore.add_argument("--json", action="store_true")

    workflow = sub.add_parser("workflow", help="manage third-party enforced workflows (presets + local)")
    workflow_sub = workflow.add_subparsers(dest="workflow_command", required=True)
    workflow_list = workflow_sub.add_parser("list", help="list presets and configured workflows")
    workflow_list.add_argument("--json", action="store_true")
    workflow_show = workflow_sub.add_parser("show", help="show one workflow by name")
    workflow_show.add_argument("name")
    workflow_show.add_argument("--json", action="store_true")
    workflow_install = workflow_sub.add_parser("install", help="install a preset as a named workflow")
    workflow_install.add_argument("--name", required=True)
    workflow_install.add_argument("--preset", required=True, choices=sorted(wf_presets))
    workflow_install.add_argument("--force", action="store_true", help="overwrite an existing workflow")
    workflow_install.add_argument("--json", action="store_true")
    workflow_remove = workflow_sub.add_parser("remove", help="remove a configured workflow")
    workflow_remove.add_argument("name")
    workflow_remove.add_argument("--yes", action="store_true")
    workflow_remove.add_argument("--json", action="store_true")

    doctor = sub.add_parser("doctor")
    doctor.add_argument("--project")
    doctor.add_argument("--json", action="store_true")

    boards = sub.add_parser("boards")
    boards_sub = boards.add_subparsers(dest="boards_command", required=True)
    boards_sync = boards_sub.add_parser("sync")
    boards_sync.add_argument("--project")
    boards_sync.add_argument("--json", action="store_true")
    boards_status = boards_sub.add_parser("status", help="expand Hermes/kanban columns for one or more projects")
    boards_status.add_argument("--project")
    boards_status.add_argument("--json", action="store_true")
    boards_status.add_argument("--limit", type=int, default=8, help="max tasks shown per column in text mode")
    boards_status.add_argument("--sync-board", action="store_true", help="ensure board exists before reading")

    fleet = sub.add_parser("fleet", help="multi-project kanban + dispatch fleet status")
    fleet.add_argument("--project")
    fleet.add_argument("--json", action="store_true")
    fleet.add_argument("--sync-boards", action="store_true", help="ensure boards exist before summarizing")

    approve = sub.add_parser("approve")
    approve.add_argument("--actor", required=True)
    approve.add_argument("--project", required=True)
    approve.add_argument("--stage", required=True, choices=["implement", "freeze", "release"])
    approve.add_argument("--feature", required=True)
    approve.add_argument("--ttl", type=int, default=900)
    approve.add_argument("--json", action="store_true")

    approvals = sub.add_parser("approvals", help="list pending / awaiting human approvals")
    approvals_sub = approvals.add_subparsers(dest="approvals_command", required=True)
    approvals_awaiting = approvals_sub.add_parser("awaiting", help="待拍板列表（CLI/飞书载荷）")
    approvals_awaiting.add_argument("--project")
    approvals_awaiting.add_argument("--channel", default="text", choices=["text", "feishu"])
    approvals_awaiting.add_argument("--json", action="store_true")

    assign = sub.add_parser("assign", help="auto-assign scorer (candidates only)")
    assign_sub = assign.add_subparsers(dest="assign_command", required=True)
    assign_candidates = assign_sub.add_parser("candidates", help="score dispatch candidates; never creates tasks")
    assign_candidates.add_argument("--project")
    assign_candidates.add_argument("--stage", default="implement")
    assign_candidates.add_argument("--feature", default="memory-adapter-auto-assign")
    assign_candidates.add_argument("--json", action="store_true")

    intent = sub.add_parser("intent", help="natural-language intent envelope (parse only; no dispatch)")
    intent_sub = intent.add_subparsers(dest="intent_command", required=True)
    intent_parse = intent_sub.add_parser("parse", help="parse utterance into IntentEnvelope JSON")
    intent_parse.add_argument("--utterance", required=True)
    intent_parse.add_argument("--project", default="", help="optional forced project slug")
    intent_parse.add_argument("--json", action="store_true")

    dispatch = sub.add_parser("dispatch")
    dispatch.add_argument("--project", required=True)
    dispatch.add_argument("--stage", required=True)
    dispatch.add_argument("--feature", required=True)
    dispatch.add_argument("--approval-token", default="")
    dispatch.add_argument("--dry-run", action="store_true")
    dispatch.add_argument("--json", action="store_true")

    task = sub.add_parser("task")
    task_sub = task.add_subparsers(dest="task_command", required=True)
    task_list = task_sub.add_parser("list")
    task_list.add_argument("--project")
    task_list.add_argument("--json", action="store_true")
    task_show = task_sub.add_parser("show")
    task_show.add_argument("dispatch_id")
    task_show.add_argument("--json", action="store_true")

    reconcile = sub.add_parser("reconcile")
    reconcile.add_argument("dispatch_id", nargs="?")
    reconcile.add_argument("--json", action="store_true")

    schedule = sub.add_parser("schedule", help="schedule heartbeat layer (register / should-run / quota)")
    schedule_sub = schedule.add_subparsers(dest="schedule_command", required=True)
    schedule_register = schedule_sub.add_parser("register", help="register a timed entry (no embedded daemon)")
    schedule_register.add_argument("--slug", required=True)
    schedule_register.add_argument("--command", required=True, dest="entry_command")
    schedule_register.add_argument("--engine", required=True, help="registered engine only (hermes)")
    schedule_register.add_argument("--cron", required=True, dest="cron_expr")
    schedule_register.add_argument("--quota-limit", type=int, required=True)
    schedule_register.add_argument("--health", default="healthy", choices=["healthy", "unhealthy"])
    schedule_register.add_argument("--json", action="store_true")
    schedule_list = schedule_sub.add_parser("list", help="list registered entries with quota status")
    schedule_list.add_argument("--json", action="store_true")
    schedule_show = schedule_sub.add_parser("show", help="show one schedule entry")
    schedule_show.add_argument("slug")
    schedule_show.add_argument("--json", action="store_true")
    schedule_should = schedule_sub.add_parser("should-run", help="deterministic quota→health gate")
    schedule_should.add_argument("slug")
    schedule_should.add_argument("--json", action="store_true")
    schedule_ledger = schedule_sub.add_parser("ledger", help="append-only heartbeat dispatch ledger")
    schedule_ledger.add_argument("--slug", default="")
    schedule_ledger.add_argument("--json", action="store_true")
    schedule_tick = schedule_sub.add_parser("cron-template", help="print hermes cron tick script fixture")
    schedule_tick.add_argument("--json", action="store_true")

    boundary = sub.add_parser("boundary", help="search-boundary curation (ingest → pending → decide)")
    boundary_sub = boundary.add_subparsers(dest="boundary_command", required=True)
    boundary_ingest = boundary_sub.add_parser("ingest", help="ingest a proposal into pending (never active)")
    boundary_ingest.add_argument("--topic", required=True)
    boundary_ingest.add_argument("--query", action="append", default=[], dest="query_hints")
    boundary_ingest.add_argument("--source", action="append", default=[], dest="sources")
    boundary_ingest.add_argument("--rationale", default="")
    boundary_ingest.add_argument("--project-profile-ref", default="", dest="project_profile_ref")
    boundary_ingest.add_argument("--account-profile-ref", default="", dest="account_profile_ref")
    boundary_ingest.add_argument("--provenance", default="in-vertical-fixture")
    boundary_ingest.add_argument("--auto-activate", action="store_true", help="illegal; always rejected")
    boundary_ingest.add_argument("--json", action="store_true")
    boundary_pending = boundary_sub.add_parser("pending", help="list pending proposals")
    boundary_pending.add_argument("--json", action="store_true")
    boundary_show = boundary_sub.add_parser("show", help="show one proposal")
    boundary_show.add_argument("proposal_id")
    boundary_show.add_argument("--json", action="store_true")
    boundary_decide = boundary_sub.add_parser("decide", help="human approve|reject")
    boundary_decide.add_argument("proposal_id")
    boundary_decide.add_argument("--actor", required=True)
    boundary_decide.add_argument("--decision", required=True, choices=["approve", "reject"])
    boundary_decide.add_argument("--note", default="")
    boundary_decide.add_argument("--json", action="store_true")
    boundary_list = boundary_sub.add_parser("list", help="list proposals (default: approved/active)")
    boundary_list.add_argument(
        "--status",
        default="approved",
        choices=["pending", "approved", "rejected", "all"],
    )
    boundary_list.add_argument("--json", action="store_true")
    boundary_tick = boundary_sub.add_parser("cron-template", help="print hermes search-boundary tick script")
    boundary_tick.add_argument("--json", action="store_true")
    boundary_fixture = boundary_sub.add_parser("tick-fixture", help="run fixture ingest-only tick")
    boundary_fixture.add_argument("--json", action="store_true")

    install = sub.add_parser("install-skills")
    install.add_argument("--dry-run", action="store_true")
    install.add_argument("--json", action="store_true")
    return parser



def _count_by(items, key="state"):
    counts = {}
    for item in items:
        value = str(item.get(key) or item.get("status") or "unknown").lower() or "unknown"
        counts[value] = counts.get(value, 0) + 1
    return counts


def _summarize_project(*, project, dispatches, executor, sync_boards: bool = False) -> dict:
    board_slug = ""
    board_exists = False
    board_error = ""
    task_counts = {}
    task_total = 0
    try:
        board_slug = executor.board_for(project)
        if sync_boards:
            board = executor.ensure_board(project)
            board_slug = str(board.get("slug") or board_slug)
            board_exists = True
        else:
            # Prefer non-mutating discovery when adapter supports list_boards.
            if hasattr(executor, "list_boards"):
                boards = executor.list_boards()
                board_exists = any(
                    isinstance(item, dict)
                    and str(item.get("slug") or "") == board_slug
                    and not item.get("archived")
                    for item in boards
                )
            else:
                # Null/demo adapters keep boards in memory; treat as present once named.
                board_exists = True
        if board_exists and hasattr(executor, "list_tasks"):
            tasks = executor.list_tasks(board_slug)
            task_total = len(tasks)
            task_counts = _count_by(tasks, key="status")
            if not task_counts:
                task_counts = _count_by(tasks, key="state")
    except Exception as exc:  # noqa: BLE001 - fleet must stay partial-success
        board_error = str(exc)

    local_counts = _count_by(dispatches, key="state")
    blocked = [d for d in dispatches if d.get("state") in {"blocked", "failed", "reconciling"}]
    latest_blocked = ""
    if blocked:
        latest = sorted(blocked, key=lambda row: str(row.get("updated_at") or ""), reverse=True)[0]
        latest_blocked = str(latest.get("last_reason_code") or latest.get("state") or "")

    active_local = sum(local_counts.get(k, 0) for k in ("draft", "awaiting_approval", "queued", "dispatched", "reconciling"))
    active_remote = sum(
        v for k, v in task_counts.items()
        if k not in {"done", "completed", "success", "succeeded", "cancelled", "archived"}
    )
    if board_error:
        health = "error"
    elif local_counts.get("blocked", 0) or local_counts.get("failed", 0) or task_counts.get("blocked", 0) or task_counts.get("failed", 0):
        health = "attention"
    elif active_local or active_remote:
        health = "active"
    else:
        health = "idle"

    return {
        "index": project.index,
        "slug": project.slug,
        "title": project.title,
        "class": project.project_class,
        "repo": project.repo,
        "docs_version": project.docs_version,
        "dispatchable": project.dispatchable,
        "board": board_slug,
        "board_exists": board_exists,
        "board_error": board_error,
        "local": {
            "total": len(dispatches),
            "counts": local_counts,
            "active": active_local,
            "latest_blocked_reason": latest_blocked,
        },
        "kanban": {
            "total": task_total,
            "counts": task_counts,
            "active": active_remote,
        },
        "health": health,
    }


def render_fleet_text(payload: dict[str, Any]) -> str:
    lines = [
        f"adapters: executor={payload.get('executor')} truth_gate={payload.get('truth_gate')}",
        f"projects: {payload.get('project_count')}  active={payload.get('active_projects')}  attention={payload.get('attention_projects')}  idle={payload.get('idle_projects')}",
        "",
        f"{'IDX':<5} {'PROJECT':<22} {'HEALTH':<10} {'BOARD':<18} {'LOCAL':<16} {'KANBAN':<16} BLOCKED",
        f"{'-'*5} {'-'*22} {'-'*10} {'-'*18} {'-'*16} {'-'*16} {'-'*24}",
    ]
    for row in payload.get("projects", []):
        local = row.get("local") or {}
        kanban = row.get("kanban") or {}
        local_txt = f"{local.get('active', 0)}/{local.get('total', 0)}"
        kanban_txt = f"{kanban.get('active', 0)}/{kanban.get('total', 0)}"
        board = str(row.get("board") or "-")
        if not row.get("board_exists"):
            board = f"{board}!"
        blocked = str((local.get("latest_blocked_reason") or row.get("board_error") or "-"))[:24]
        lines.append(
            f"{str(row.get('index') or '?'):<5} {str(row.get('slug') or ''):<22} {str(row.get('health') or ''):<10} {board:<18} {local_txt:<16} {kanban_txt:<16} {blocked}"
        )
    return "\n".join(lines)



HERMES_COLUMNS = (
    "triage",
    "todo",
    "ready",
    "scheduled",
    "running",
    "review",
    "blocked",
    "done",
    "archived",
)

SIMPLIFIED_BUCKETS = {
    "todo": ("triage", "todo"),
    "doing": ("ready", "scheduled", "running", "review"),
    "blocked": ("blocked",),
    "done": ("done", "archived", "completed", "success", "succeeded"),
}


def _task_status(task: dict[str, Any]) -> str:
    return str(task.get("status") or task.get("state") or "unknown").lower() or "unknown"


def _task_title(task: dict[str, Any]) -> str:
    return str(
        task.get("title")
        or task.get("name")
        or task.get("summary")
        or task.get("id")
        or task.get("task_id")
        or "(untitled)"
    )


def _task_id(task: dict[str, Any]) -> str:
    return str(task.get("id") or task.get("task_id") or "")


def build_board_status(
    *,
    project,
    executor,
    dispatches: list[dict[str, Any]] | None = None,
    sync_board: bool = False,
    limit: int = 8,
) -> dict[str, Any]:
    board_slug = executor.board_for(project)
    board_meta: dict[str, Any] = {"slug": board_slug}
    error = ""
    tasks: list[dict[str, Any]] = []
    stats_payload: dict[str, Any] = {}
    try:
        if sync_board:
            board_meta = dict(executor.ensure_board(project))
            board_slug = str(board_meta.get("slug") or board_slug)
        elif hasattr(executor, "list_boards"):
            boards = executor.list_boards()
            matched = next(
                (
                    item
                    for item in boards
                    if isinstance(item, dict)
                    and str(item.get("slug") or "") == board_slug
                    and not item.get("archived")
                ),
                None,
            )
            if matched:
                board_meta = dict(matched)
            else:
                error = f"board not found: {board_slug}"
        if not error and hasattr(executor, "list_tasks"):
            tasks = list(executor.list_tasks(board_slug) or [])
        if not error and hasattr(executor, "stats"):
            try:
                stats_payload = dict(executor.stats(board_slug) or {})
            except Exception as exc:  # noqa: BLE001
                stats_payload = {"error": str(exc)}
    except Exception as exc:  # noqa: BLE001
        error = str(exc)

    columns: dict[str, list[dict[str, Any]]] = {name: [] for name in HERMES_COLUMNS}
    extra_columns: dict[str, list[dict[str, Any]]] = {}
    for task in tasks:
        status = _task_status(task)
        item = {
            "id": _task_id(task),
            "title": _task_title(task),
            "status": status,
            "assignee": task.get("assignee") or task.get("owner") or "",
            "updated_at": task.get("updated_at") or task.get("updated") or "",
        }
        if status in columns:
            columns[status].append(item)
        else:
            extra_columns.setdefault(status, []).append(item)

    # Prefer hermes stats counts when present; fall back to listed tasks.
    raw_counts = {}
    if isinstance(stats_payload.get("by_status"), dict):
        raw_counts = {
            str(k).lower(): int(v)
            for k, v in stats_payload.get("by_status", {}).items()
        }
    if not raw_counts:
        raw_counts = {name: len(items) for name, items in columns.items() if items}
        raw_counts.update({name: len(items) for name, items in extra_columns.items()})

    column_counts = {name: int(raw_counts.get(name, 0)) for name in HERMES_COLUMNS}
    for name, value in raw_counts.items():
        if name not in column_counts:
            column_counts[name] = int(value)

    simplified = {
        bucket: sum(column_counts.get(status, 0) for status in statuses)
        for bucket, statuses in SIMPLIFIED_BUCKETS.items()
    }
    unknown = sum(
        count
        for status, count in column_counts.items()
        if status not in HERMES_COLUMNS
        and status not in {"completed", "success", "succeeded"}
    )
    # completed aliases fold into done already via simplified mapping when present in raw counts
    simplified["done"] = simplified.get("done", 0) + sum(
        int(column_counts.get(alias, 0))
        for alias in ("completed", "success", "succeeded")
        if alias in column_counts and alias not in HERMES_COLUMNS
    )

    local = dispatches or []
    local_counts = _count_by(local, key="state")
    return {
        "project": project.slug,
        "title": project.title,
        "repo": project.repo,
        "board": board_slug,
        "board_meta": board_meta,
        "error": error,
        "total_tasks": sum(column_counts.values()) if column_counts else len(tasks),
        "simplified": simplified,
        "columns": {
            name: {
                "count": column_counts.get(name, 0),
                "tasks": columns.get(name, [])[: max(limit, 0) if limit else None],
            }
            for name in HERMES_COLUMNS
        },
        "extra_columns": {
            name: {
                "count": column_counts.get(name, len(items)),
                "tasks": items[: max(limit, 0) if limit else None],
            }
            for name, items in extra_columns.items()
        },
        "stats": stats_payload,
        "local_dispatches": {
            "total": len(local),
            "counts": local_counts,
        },
        "unknown_count": unknown,
    }


def render_board_status_text(payload: dict[str, Any]) -> str:
    rows = payload.get("boards") or []
    lines = [
        f"adapters: executor={payload.get('executor')} truth_gate={payload.get('truth_gate')}",
        f"boards: {len(rows)}",
        "",
    ]
    for row in rows:
        simplified = row.get("simplified") or {}
        lines.append(
            f"## {row.get('project')}  board={row.get('board')}  total={row.get('total_tasks', 0)}"
        )
        if row.get("error"):
            lines.append(f"error: {row.get('error')}")
        lines.append(
            "summary: "
            f"todo={simplified.get('todo', 0)}  "
            f"doing={simplified.get('doing', 0)}  "
            f"blocked={simplified.get('blocked', 0)}  "
            f"done={simplified.get('done', 0)}"
        )
        # compact column strip
        cols = row.get("columns") or {}
        strip = " | ".join(
            f"{name}:{((cols.get(name) or {}).get('count') or 0)}"
            for name in HERMES_COLUMNS
        )
        lines.append(f"columns: {strip}")
        for name in HERMES_COLUMNS:
            bucket = cols.get(name) or {}
            tasks = bucket.get("tasks") or []
            count = int(bucket.get("count") or 0)
            if count <= 0 and not tasks:
                continue
            lines.append(f"  [{name}] {count}")
            for task in tasks:
                title = str(task.get("title") or "")[:72]
                tid = str(task.get("id") or "")
                assignee = str(task.get("assignee") or "")
                suffix = f" @{assignee}" if assignee else ""
                lines.append(f"    - {tid}: {title}{suffix}".rstrip())
        extra = row.get("extra_columns") or {}
        for name, bucket in extra.items():
            count = int(bucket.get("count") or 0)
            tasks = bucket.get("tasks") or []
            if count <= 0 and not tasks:
                continue
            lines.append(f"  [{name}] {count}")
            for task in tasks:
                lines.append(
                    f"    - {task.get('id')}: {str(task.get('title') or '')[:72]}"
                )
        local = row.get("local_dispatches") or {}
        lines.append(
            f"local_dispatches: total={local.get('total', 0)} counts={local.get('counts') or {}}"
        )
        lines.append("")
    return "\n".join(lines).rstrip() + "\n"


def _resolve_project_ref(registry: ProjectRegistry, raw: str | None) -> str | None:
    """Map a user-supplied project reference to a canonical slug.

    Accepts a numeric index (``--project 5`` / ``--slug 5``), a slug, or an
    alias. Returns the canonical slug, or None when raw is empty.
    """
    if not raw:
        return None
    text = str(raw).strip()
    if text.isdigit():
        return registry.resolve(index=int(text)).slug
    try:
        return registry.resolve(slug=text).slug
    except DeliveryBusError:
        return registry.resolve(alias=text).slug


def execute(args: argparse.Namespace) -> dict[str, Any]:
    if args.command == "install-skills":
        skill = ROOT / "skills" / "agent-delivery-bus"
        return envelope(status="pass", data=install_skill(skill, dry_run=args.dry_run))

    registry = ProjectRegistry.load(args.config)
    storage = Storage(args.db)
    try:
        # 统一把 --project 的纯数字编号解析为 canonical slug（dispatch/approve/assign/task 等共用）。
        args.project = _resolve_project_ref(registry, getattr(args, "project", None)) or ""
        resolver = AdapterResolver(registry.raw)
        wired = resolver.global_adapters()
        executor = wired["executor"]
        truth_gate = wired["truth_gate"]
        preflight = Preflight(truth_gate, executor)
        service = DeliveryService(
            registry,
            storage,
            preflight=preflight,
            executor=executor,
            truth_gate=truth_gate,
            memory=wired["memory"],
            adapter_resolver=resolver.for_project,
        )
        approvals = ApprovalService(storage)
        scorer = AssignmentScorer(registry)

        if args.command == "projects":
            if args.projects_command == "list":
                rows = [item.to_dict() for item in registry.list(dispatchable_only=args.dispatchable_only)]
                payload = {
                    "projects": rows,
                    "adapters": {
                        "executor": wired["executor_name"],
                        "truth_gate": wired["truth_gate_name"],
                    },
                }
                if args.numbered:
                    payload["numbered"] = True
                    payload["text"] = "\n".join(
                        f"[#{row['index']}] {row['slug']} — {row['title']}"
                        for row in rows
                    )
                else:
                    payload["text"] = "\n".join(
                        f"{row['index']:<5} {row['slug']:<28} {row['title']}"
                        for row in rows
                    )
                return envelope(status="pass", data=payload)
            if args.projects_command == "register":
                aliases = tuple(item.strip() for item in args.aliases.split(",") if item.strip())
                project = registry.register(
                    slug=args.slug,
                    title=args.title,
                    project_class=args.project_class,
                    repo=args.repo,
                    aliases=aliases,
                    docs_root=args.docs_root,
                    docs_version=args.docs_version,
                    truth_gate=args.truth_gate,
                    executor=args.executor,
                    binding_profile=args.binding_profile,
                )
                payload = project.to_dict()
                payload["text"] = f"项目已登记：#{project.index} {project.slug}（{project.title}）"
                return envelope(status="pass", data=payload)
            if args.projects_command == "delete":
                if not args.yes:
                    return envelope(
                        status="blocked",
                        blocked=True,
                        reason_code="project_delete_confirmation_required",
                        resume_action="show the target to the human and re-run with --yes after confirmation",
                        data={"target": args.target},
                    )
                project = registry.delete(args.target)
                payload = {"project": project.to_dict(), "deleted": True}
                payload["text"] = (
                    f"项目已归档（软删除）：{project.slug}（#{project.index}），"
                    "编号保留、不可派发；可用 restore 恢复"
                )
                return envelope(status="pass", data=payload)
            if args.projects_command == "restore":
                project = registry.restore(args.target)
                payload = {"project": project.to_dict(), "restored": True}
                payload["text"] = f"项目已恢复：{project.slug}（#{project.index}），可派发"
                return envelope(status="pass", data=payload)
            # --slug 支持纯数字：自动解释为项目编号（index），与 --index 等价。
            # 优先级：显式 --index > --path > --alias > --slug（数字→index，否则当 slug）。
            resolved_index = args.index
            resolved_slug = args.slug
            if resolved_slug is not None and str(resolved_slug).strip().isdigit() and resolved_index is None:
                resolved_index = int(str(resolved_slug).strip())
                resolved_slug = None
            project = registry.resolve(slug=resolved_slug, alias=args.alias, path=args.path, index=resolved_index)
            return envelope(status="pass", data=project.to_dict())

        if args.command == "workflow":
            if args.workflow_command == "list":
                configured = (
                    registry.raw.get("workflows")
                    if isinstance(registry.raw.get("workflows"), dict)
                    else {}
                )
                rows = []
                for name in workflow_names(registry.raw):
                    wf = get_workflow(registry.raw, name)
                    rows.append(
                        {
                            "name": name,
                            "source": "preset" if name in wf_presets else "configured",
                            "description": wf.get("description", ""),
                            "skills": wf.get("skills", []),
                        }
                    )
                payload = {"workflows": rows, "presets": sorted(wf_presets)}
                payload["text"] = "\n".join(
                    f"{'[预设]' if row['source'] == 'preset' else '[本地]'} {row['name']} — {row['description']}"
                    for row in rows
                )
                return envelope(status="pass", data=payload)
            if args.workflow_command == "show":
                wf = get_workflow(registry.raw, args.name)
                return envelope(status="pass", data=wf)
            if args.workflow_command == "install":
                workflows_cfg = (
                    registry.raw.get("workflows")
                    if isinstance(registry.raw.get("workflows"), dict)
                    else {}
                )
                if args.name in workflows_cfg and not args.force:
                    return envelope(
                        status="blocked",
                        blocked=True,
                        reason_code="workflow_exists",
                        resume_action="re-run with --force to overwrite",
                        data={"name": args.name},
                    )
                installed = install_workflow(registry.raw, name=args.name, preset=args.preset)
                registry.save()
                payload = dict(installed)
                payload["name"] = args.name
                payload["text"] = f"工作流已安装：{args.name}（preset={args.preset}）"
                return envelope(status="pass", data=payload)
            if args.workflow_command == "remove":
                if not args.yes:
                    return envelope(
                        status="blocked",
                        blocked=True,
                        reason_code="workflow_remove_confirmation_required",
                        resume_action="re-run with --yes after human confirmation",
                        data={"name": args.name},
                    )
                removed = remove_workflow(registry.raw, args.name)
                registry.save()
                return envelope(
                    status="pass",
                    data={
                        "removed": args.name,
                        "workflow": removed,
                        "text": f"工作流已移除：{args.name}",
                    },
                )

        if args.command == "doctor":
            projects = [registry.resolve(slug=args.project)] if args.project else registry.list(dispatchable_only=True)
            results = []
            for project in projects:
                project_adapters = resolver.for_project(project)
                results.append(
                    Preflight(project_adapters["truth_gate"], project_adapters["executor"]).run(
                        project, stage="plan"
                    )
                )
            blocked = any(item["blocked"] for item in results)
            return envelope(
                status="blocked" if blocked else "pass",
                blocked=blocked,
                reason_code=next((item["reason_code"] for item in results if item["blocked"]), ""),
                resume_action=next((item["resume_action"] for item in results if item["blocked"]), ""),
                data={"results": results},
            )


        if args.command == "fleet":
            projects = (
                [registry.resolve(slug=args.project)]
                if args.project
                else registry.list(dispatchable_only=True)
            )
            rows = []
            for project in projects:
                dispatches = storage.list_dispatches(project_slug=project.slug)
                project_adapters = resolver.for_project(project)
                rows.append(
                    _summarize_project(
                        project=project,
                        dispatches=dispatches,
                        executor=project_adapters["executor"],
                        sync_boards=bool(getattr(args, "sync_boards", False)),
                    )
                )
            health_counts = _count_by(rows, key="health")
            payload = {
                "executor": wired["executor_name"],
                "truth_gate": wired["truth_gate_name"],
                "project_count": len(rows),
                "active_projects": health_counts.get("active", 0),
                "attention_projects": health_counts.get("attention", 0) + health_counts.get("error", 0),
                "idle_projects": health_counts.get("idle", 0),
                "projects": rows,
            }
            if not bool(getattr(args, "json", False)):
                # Human-readable summary still travels in envelope data.
                payload["text"] = render_fleet_text(payload)
            return envelope(status="pass", data=payload)

        if args.command == "boards":
            if args.boards_command == "sync":
                projects = [registry.resolve(slug=args.project)] if args.project else registry.list(dispatchable_only=True)
                boards = [executor.ensure_board(project) for project in projects]
                return envelope(status="pass", data={"boards": boards})
            if args.boards_command == "status":
                projects = (
                    [registry.resolve(slug=args.project)]
                    if args.project
                    else registry.list(dispatchable_only=True)
                )
                rows = []
                for project in projects:
                    project_adapters = resolver.for_project(project)
                    rows.append(
                        build_board_status(
                            project=project,
                            executor=project_adapters["executor"],
                            dispatches=storage.list_dispatches(project_slug=project.slug),
                            sync_board=bool(getattr(args, "sync_board", False)),
                            limit=int(getattr(args, "limit", 8) or 0),
                        )
                    )
                payload = {
                    "executor": wired["executor_name"],
                    "truth_gate": wired["truth_gate_name"],
                    "boards": rows,
                }
                if not bool(getattr(args, "json", False)):
                    payload["text"] = render_board_status_text(payload)
                blocked = any(bool(row.get("error")) for row in rows)
                return envelope(
                    status="blocked" if blocked else "pass",
                    blocked=blocked,
                    reason_code="board_status_partial" if blocked else "",
                    resume_action="inspect board_error fields, then rerun with --sync-board if needed" if blocked else "",
                    data=payload,
                )
            raise DeliveryBusError("boards_command_invalid", f"Unknown boards command: {args.boards_command}")

        if args.command == "approve":
            issued = approvals.issue(
                actor=args.actor,
                project_slug=args.project,
                stage=args.stage,
                feature=args.feature,
                ttl_seconds=args.ttl,
            )
            return envelope(status="pass", data=issued)

        if args.command == "approvals":
            if args.approvals_command == "awaiting":
                views = pending_approval_views(storage, project_slug=args.project)
                rendered = render_pending_channel(views, channel=args.channel)
                return envelope(status="pass", data=rendered)
            raise DeliveryBusError("approvals_command_invalid", f"Unknown approvals command: {args.approvals_command}")

        if args.command == "assign":
            if args.assign_command == "candidates":
                rows = scorer.candidates(
                    stage=args.stage,
                    feature=args.feature,
                    project_slug=args.project,
                )
                scorer.assert_candidates_only(rows)
                return envelope(status="pass", data={"candidates": rows})
            raise DeliveryBusError("assign_command_invalid", f"Unknown assign command: {args.assign_command}")

        if args.command == "intent":
            if args.intent_command == "parse":
                parser = IntentParser(registry)
                parsed = parser.parse(
                    args.utterance,
                    project=(args.project or None),
                )
                return envelope(
                    status=str(parsed.get("status") or "pass"),
                    blocked=bool(parsed.get("blocked")),
                    reason_code=str(parsed.get("reason_code") or ""),
                    resume_action=str(parsed.get("resume_action") or ""),
                    data=parsed.get("data"),
                )
            raise DeliveryBusError("intent_command_invalid", f"Unknown intent command: {args.intent_command}")

        if args.command == "dispatch":
            result = service.dispatch(
                project_slug=args.project,
                stage=args.stage,
                feature=args.feature,
                approval_token=args.approval_token,
                dry_run=args.dry_run,
            )
            return envelope(
                status=str(result.get("status") or "pass"),
                blocked=bool(result.get("blocked")),
                reason_code=str(result.get("reason_code") or ""),
                resume_action=str(result.get("resume_action") or ""),
                data=result,
            )

        if args.command == "task":
            if args.task_command == "list":
                return envelope(status="pass", data={"dispatches": storage.list_dispatches(project_slug=args.project)})
            return envelope(status="pass", data=storage.get_dispatch(args.dispatch_id))

        if args.command == "reconcile":
            ids = [args.dispatch_id] if args.dispatch_id else [
                row["dispatch_id"]
                for row in storage.list_dispatches()
                if row["state"] in {"dispatched", "reconciling"}
            ]
            results = [service.reconcile(dispatch_id) for dispatch_id in ids]
            blocked = any(item.get("blocked") for item in results)
            return envelope(status="blocked" if blocked else "pass", blocked=blocked, data={"results": results})

        if args.command == "schedule":
            schedules = ScheduleService(storage)
            if args.schedule_command == "register":
                entry = schedules.register(
                    slug=args.slug,
                    command=args.entry_command,
                    engine=args.engine,
                    cron_expr=args.cron_expr,
                    quota_limit=args.quota_limit,
                    health=args.health,
                )
                return envelope(status="pass", data={"entry": schedules.show(entry["slug"])})
            if args.schedule_command == "list":
                return envelope(status="pass", data={"entries": schedules.list_entries()})
            if args.schedule_command == "show":
                return envelope(status="pass", data={"entry": schedules.show(args.slug)})
            if args.schedule_command == "should-run":
                decision = schedules.should_run(args.slug)
                return envelope(
                    status=str(decision.get("status") or "pass"),
                    blocked=bool(decision.get("blocked")),
                    reason_code=str(decision.get("reason_code") or ""),
                    resume_action=str(decision.get("resume_action") or ""),
                    data=decision,
                )
            if args.schedule_command == "ledger":
                rows = schedules.ledger(slug=args.slug or None)
                return envelope(status="pass", data={"runs": rows})
            if args.schedule_command == "cron-template":
                script = hermes_cron_tick_script()
                return envelope(status="pass", data={"text": script, "cron_owner": "hermes"})
            raise DeliveryBusError("schedule_command_invalid", f"Unknown schedule command: {args.schedule_command}")

        if args.command == "boundary":
            boundaries = BoundaryService(storage)
            if args.boundary_command == "ingest":
                row = boundaries.ingest(
                    topic=args.topic,
                    query_hints=list(args.query_hints or []),
                    sources=list(args.sources or []),
                    rationale=args.rationale,
                    project_profile_ref=str(getattr(args, "project_profile_ref", "") or ""),
                    account_profile_ref=str(getattr(args, "account_profile_ref", "") or ""),
                    provenance=str(getattr(args, "provenance", "") or "in-vertical-fixture"),
                    auto_activate=bool(args.auto_activate),
                )
                return envelope(status="pass", data={"proposal": row})
            if args.boundary_command == "pending":
                return envelope(status="pass", data={"proposals": boundaries.pending()})
            if args.boundary_command == "show":
                return envelope(status="pass", data={"proposal": boundaries.show(args.proposal_id)})
            if args.boundary_command == "decide":
                row = boundaries.decide(
                    args.proposal_id,
                    actor=args.actor,
                    decision=args.decision,
                    note=args.note,
                )
                return envelope(status="pass", data={"proposal": row})
            if args.boundary_command == "list":
                return envelope(
                    status="pass",
                    data={"proposals": boundaries.list(status=args.status)},
                )
            if args.boundary_command == "cron-template":
                script = hermes_boundary_tick_script()
                return envelope(status="pass", data={"text": script, "cron_owner": "hermes"})
            if args.boundary_command == "tick-fixture":
                result = boundaries.run_tick_fixture()
                return envelope(status="pass", data=result)
            raise DeliveryBusError(
                "boundary_command_invalid",
                f"Unknown boundary command: {args.boundary_command}",
            )

        raise DeliveryBusError("command_invalid", "Unknown command")
    finally:
        storage.close()


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    as_json = bool(getattr(args, "json", False))
    try:
        payload = execute(args)
    except DeliveryBusError as exc:
        payload = envelope(
            status="blocked",
            blocked=True,
            reason_code=exc.reason_code,
            resume_action=exc.resume_action,
            data={"message": exc.message, **(exc.data or {})},
        )
    emit(payload, as_json=as_json)
    return 2 if payload.get("blocked") else 0


if __name__ == "__main__":
    sys.exit(main())
