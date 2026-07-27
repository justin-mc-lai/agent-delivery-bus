from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

from .adapters.beacon import BeaconAdapter
from .adapters.hermes import HermesAdapter
from .approvals import ApprovalService
from .errors import DeliveryBusError
from .install import install_skill
from .preflight import Preflight
from .registry import ProjectRegistry
from .service import DeliveryService
from .storage import Storage


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_CONFIG = ROOT / "config" / "projects.json"
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
    else:
        print(f"{payload['status']}: {payload.get('reason_code') or 'ok'}")
        if payload.get("data") is not None:
            print(json.dumps(payload["data"], ensure_ascii=False, indent=2))


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

    installer = sub.add_parser("install-skills")
    installer.add_argument("--dry-run", action="store_true")
    installer.add_argument("--json", action="store_true")
    return parser


def execute(args: argparse.Namespace) -> dict[str, Any]:
    registry = ProjectRegistry.load(args.config)
    hermes = HermesAdapter()
    beacon = BeaconAdapter()
    preflight = Preflight(beacon, hermes)
    if args.command == "projects":
        if args.projects_command == "list":
            rows = [item.to_dict() for item in registry.list(dispatchable_only=args.dispatchable_only)]
            return envelope(status="pass", data={"projects": rows, "count": len(rows)})
        project = registry.resolve(slug=args.slug, alias=args.alias, path=args.path)
        return envelope(status="pass", data=project.to_dict())

    if args.command == "doctor":
        projects = [registry.resolve(slug=args.project)] if args.project else registry.list(dispatchable_only=True)
        results = [preflight.run(project, stage="plan") for project in projects]
        blocked = any(item["blocked"] for item in results)
        first = next((item for item in results if item["blocked"]), {})
        return envelope(
            status="blocked" if blocked else "pass",
            blocked=blocked,
            reason_code=str(first.get("reason_code") or ""),
            resume_action=str(first.get("resume_action") or ""),
            data={"results": results},
        )

    if args.command == "boards":
        projects = [registry.resolve(slug=args.project)] if args.project else registry.list(dispatchable_only=True)
        rows = [hermes.ensure_board(project) for project in projects]
        return envelope(status="pass", data={"boards": rows})

    if args.command == "install-skills":
        return envelope(
            status="pass",
            data=install_skill(ROOT / "skills" / "agent-delivery-bus", dry_run=args.dry_run),
        )

    storage = Storage(args.db)
    service = DeliveryService(
        registry,
        storage,
        preflight=preflight,
        hermes=hermes,
        beacon=beacon,
    )
    try:
        if args.command == "approve":
            project = registry.resolve(slug=args.project)
            approval = ApprovalService(storage).issue(
                actor=args.actor,
                project_slug=project.slug,
                stage=args.stage,
                feature=args.feature,
                ttl_seconds=args.ttl,
            )
            return envelope(status="issued", data=approval)

        if args.command == "dispatch":
            result = service.dispatch(
                project_slug=args.project,
                stage=args.stage,
                feature=args.feature,
                approval_token=args.approval_token,
                dry_run=args.dry_run,
            )
            return envelope(
                status=str(result.get("status") or "unknown"),
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
