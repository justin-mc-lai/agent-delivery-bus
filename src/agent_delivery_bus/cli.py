from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

from .adapters.factory import adapters_from_config
from .approvals import ApprovalService
from .errors import DeliveryBusError
from .install import install_skill
from .preflight import Preflight
from .registry import ProjectRegistry
from .service import DeliveryService
from .storage import Storage


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
    projects_list.add_argument("--json", action="store_true")
    projects_resolve = projects_sub.add_parser("resolve")
    group = projects_resolve.add_mutually_exclusive_group(required=True)
    group.add_argument("--slug")
    group.add_argument("--alias")
    group.add_argument("--path")
    projects_resolve.add_argument("--json", action="store_true")

    doctor = sub.add_parser("doctor")
    doctor.add_argument("--project")
    doctor.add_argument("--json", action="store_true")

    boards = sub.add_parser("boards")
    boards_sub = boards.add_subparsers(dest="boards_command", required=True)
    boards_sync = boards_sub.add_parser("sync")
    boards_sync.add_argument("--project")
    boards_sync.add_argument("--json", action="store_true")

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
        f"{'PROJECT':<22} {'HEALTH':<10} {'BOARD':<18} {'LOCAL':<16} {'KANBAN':<16} BLOCKED",
        f"{'-'*22} {'-'*10} {'-'*18} {'-'*16} {'-'*16} {'-'*24}",
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
            f"{str(row.get('slug') or ''):<22} {str(row.get('health') or ''):<10} {board:<18} {local_txt:<16} {kanban_txt:<16} {blocked}"
        )
    return "\n".join(lines)


def execute(args: argparse.Namespace) -> dict[str, Any]:
    if args.command == "install-skills":
        skill = ROOT / "skills" / "agent-delivery-bus"
        return envelope(status="pass", data=install_skill(skill, dry_run=args.dry_run))

    registry = ProjectRegistry.load(args.config)
    storage = Storage(args.db)
    try:
        wired = adapters_from_config(registry.raw)
        executor = wired["executor"]
        truth_gate = wired["truth_gate"]
        preflight = Preflight(truth_gate, executor)
        service = DeliveryService(
            registry,
            storage,
            preflight=preflight,
            executor=executor,
            truth_gate=truth_gate,
        )
        approvals = ApprovalService(storage)

        if args.command == "projects":
            if args.projects_command == "list":
                rows = [item.to_dict() for item in registry.list(dispatchable_only=args.dispatchable_only)]
                return envelope(status="pass", data={"projects": rows, "adapters": {
                    "executor": wired["executor_name"],
                    "truth_gate": wired["truth_gate_name"],
                }})
            project = registry.resolve(slug=args.slug, alias=args.alias, path=args.path)
            return envelope(status="pass", data=project.to_dict())

        if args.command == "doctor":
            projects = [registry.resolve(slug=args.project)] if args.project else registry.list(dispatchable_only=True)
            results = [preflight.run(project, stage="plan") for project in projects]
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
                rows.append(
                    _summarize_project(
                        project=project,
                        dispatches=dispatches,
                        executor=executor,
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
            projects = [registry.resolve(slug=args.project)] if args.project else registry.list(dispatchable_only=True)
            boards = [executor.ensure_board(project) for project in projects]
            return envelope(status="pass", data={"boards": boards})

        if args.command == "approve":
            issued = approvals.issue(
                actor=args.actor,
                project_slug=args.project,
                stage=args.stage,
                feature=args.feature,
                ttl_seconds=args.ttl,
            )
            return envelope(status="pass", data=issued)

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
