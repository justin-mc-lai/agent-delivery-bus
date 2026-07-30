"""Delivery service: approval, idempotent dispatch, and evidence reconciliation."""

from __future__ import annotations

import hashlib
import json
from typing import Any

from .adapters.memory import InMemoryMemoryAdapter
from .adapters.spi import ExecutorAdapter, MemoryAdapter, TruthGateAdapter
from .approvals import ApprovalService, RESTRICTED_STAGES
from .errors import CommandTimedOut, DeliveryBusError
from .preflight import Preflight
from .registry import Project, ProjectRegistry
from .storage import Storage
from .worker_binding import (
    ENABLED_STAGES,
    assert_stage_enabled,
    format_binding_section,
    resolve_worker_binding,
)


TERMINAL_EXECUTOR_SUCCESS = {"done", "completed", "success", "succeeded"}
TERMINAL_EXECUTOR_FAILURE = {"blocked", "failed", "cancelled", "archived"}


def normalized_request(project: Project, *, stage: str, feature: str) -> dict[str, Any]:
    return {
        "schema_version": "1.0",
        "project_slug": project.slug,
        "canonical_repo": project.repo,
        "docs_version": project.docs_version,
        "stage": stage.strip().lower(),
        "feature": feature.strip(),
    }


def request_digest(request: dict[str, Any]) -> str:
    canonical = json.dumps(request, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


def task_body(
    project: Project,
    *,
    stage: str,
    feature: str,
    memory_summary: str = "",
) -> str:
    binding = resolve_worker_binding(
        stage=stage,
        feature=feature,
        docs_version=project.docs_version or "",
    )
    approval_note = (
        "A matching one-time approval was reserved by Agent Delivery Bus."
        if stage in RESTRICTED_STAGES
        else "This stage is not approval-gated."
    )
    lines = [
        f"Project: {project.slug}",
        f"Repository: {project.repo}",
        f"Docs version: {project.docs_version or '(none)'}",
        f"Stage: {stage}",
        f"Feature: {feature}",
        "",
        approval_note,
        "Run the project's governed workflow for this stage and preserve its delivery gates.",
        "Do not release, merge, push, or repair project context unless a separate human instruction explicitly authorizes it.",
        "Worker success is an execution receipt only; Agent Delivery Bus will reconcile truth-gate evidence separately.",
        "",
        format_binding_section(binding),
    ]
    if memory_summary.strip():
        lines.extend(["", "### Scoped memory recall", memory_summary.strip()])
    return "\n".join(lines)


class DeliveryService:
    def __init__(
        self,
        registry: ProjectRegistry,
        storage: Storage,
        *,
        preflight: Preflight | None = None,
        executor: ExecutorAdapter | None = None,
        truth_gate: TruthGateAdapter | None = None,
        memory: MemoryAdapter | None = None,
        # Backward-compatible aliases
        hermes: ExecutorAdapter | None = None,
        beacon: TruthGateAdapter | None = None,
    ):
        self.registry = registry
        self.storage = storage
        self.executor = executor or hermes
        self.truth_gate = truth_gate or beacon
        if self.executor is None or self.truth_gate is None:
            raise ValueError("DeliveryService requires executor and truth_gate adapters")
        # Compatibility attributes for older call sites/tests.
        self.hermes = self.executor
        self.beacon = self.truth_gate
        self.memory = memory or InMemoryMemoryAdapter()
        self.preflight = preflight or Preflight(self.truth_gate, self.executor)
        self.approvals = ApprovalService(storage)

    def _recall_for_dispatch(self, project: Project, *, stage: str, feature: str) -> dict[str, Any]:
        query = f"{project.slug} {stage} {feature}".strip()
        return self.memory.recall(project_slug=project.slug, query=query, limit=8)

    def _safe_writeback(
        self,
        *,
        project_slug: str,
        stage: str,
        feature: str,
        dispatch_id: str,
        reason_code: str,
        payload: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        try:
            result = self.memory.writeback(
                project_slug=project_slug,
                stage=stage,
                feature=feature,
                dispatch_id=dispatch_id,
                reason_code=reason_code,
                payload=payload,
            )
            return {"ok": True, "result": result}
        except DeliveryBusError as exc:
            return {
                "ok": False,
                "reason_code": exc.reason_code,
                "message": str(exc),
                "resume_action": exc.resume_action,
                "data": exc.data or {},
            }
        except Exception as exc:  # noqa: BLE001 - writeback must never erase reconcile
            return {
                "ok": False,
                "reason_code": "memory_writeback_failed",
                "message": str(exc),
                "resume_action": "retry writeback; reconcile status is unchanged",
            }

    def dispatch(
        self,
        *,
        project_slug: str,
        stage: str,
        feature: str,
        approval_token: str = "",
        dry_run: bool = False,
        forced_idempotency_key: str = "",
    ) -> dict[str, Any]:
        project = self.registry.resolve(slug=project_slug)
        if not project.dispatchable:
            raise DeliveryBusError("project_not_dispatchable", f"Project {project.slug} is not dispatchable")
        feature = feature.strip()
        if not feature:
            raise DeliveryBusError("feature_required", "feature is required")
        stage = assert_stage_enabled(stage)

        request = normalized_request(project, stage=stage, feature=feature)
        digest = request_digest(request)
        idempotency_key = forced_idempotency_key or f"adb-v1-{digest}"
        preflight = self.preflight.run(project, stage=stage)
        if dry_run:
            return {
                "status": "blocked" if preflight["blocked"] else "pass",
                "blocked": preflight["blocked"],
                "reason_code": preflight["reason_code"],
                "resume_action": preflight["resume_action"],
                "dry_run": True,
                "request": request,
                "idempotency_key": idempotency_key,
                "board": self.executor.board_for(project),
                "workspace": self.executor.workspace_for(project, stage=stage),
                "preflight": preflight,
            }

        self.storage.snapshot_project(project.slug, project.to_dict())
        dispatch, created = self.storage.create_dispatch(
            idempotency_key=idempotency_key,
            request_hash=digest,
            request=request,
            project_slug=project.slug,
            stage=stage,
            feature=feature,
        )
        dispatch_id = dispatch["dispatch_id"]
        if not created and dispatch["state"] in {"blocked", "failed"}:
            if preflight["blocked"]:
                return {
                    "status": dispatch["state"],
                    "blocked": True,
                    "duplicate": True,
                    "reason_code": preflight["reason_code"],
                    "resume_action": preflight["resume_action"],
                    "dispatch": dispatch,
                }
            dispatch = self.storage.transition(
                dispatch_id,
                expected_from=("blocked", "failed"),
                to_state="draft",
                event_type="retry",
                payload={"preflight": preflight},
            )
        elif not created and dispatch["state"] == "queued":
            return {
                "status": "reconciling",
                "blocked": True,
                "duplicate": True,
                "reason_code": "reconciliation_required",
                "resume_action": "run `adb reconcile` before retrying a queued request",
                "dispatch": dispatch,
            }
        elif not created and dispatch["state"] not in {"draft", "awaiting_approval"}:
            return {
                "status": dispatch["state"],
                "blocked": dispatch["state"] == "reconciling",
                "duplicate": True,
                "reason_code": dispatch.get("last_reason_code") or "",
                "resume_action": dispatch.get("resume_action") or "",
                "dispatch": dispatch,
            }
        if preflight["blocked"]:
            if dispatch["state"] == "draft":
                dispatch = self.storage.transition(
                    dispatch_id,
                    expected_from="draft",
                    to_state="blocked",
                    event_type="preflight_failed",
                    reason_code=preflight["reason_code"],
                    resume_action=preflight["resume_action"],
                    payload={"preflight": preflight},
                )
            return {
                "status": "blocked",
                "blocked": True,
                "reason_code": preflight["reason_code"],
                "resume_action": preflight["resume_action"],
                "dispatch": dispatch,
            }

        approval_id: str | None = dispatch.get("approval_id")
        if stage in RESTRICTED_STAGES:
            if dispatch["state"] == "draft":
                dispatch = self.storage.transition(
                    dispatch_id,
                    expected_from="draft",
                    to_state="awaiting_approval",
                    event_type="submit_restricted",
                )
            if not approval_token:
                return {
                    "status": "awaiting_approval",
                    "blocked": True,
                    "reason_code": "approval_required",
                    "resume_action": "issue an approval token with `adb approve`, then repeat the same dispatch request",
                    "dispatch": dispatch,
                }
            try:
                approval = self.approvals.reserve(
                    approval_token,
                    dispatch_id=dispatch_id,
                    project_slug=project.slug,
                    stage=stage,
                    feature=feature,
                )
            except DeliveryBusError as exc:
                dispatch = self.storage.transition(
                    dispatch_id,
                    expected_from="awaiting_approval",
                    to_state="blocked",
                    event_type="approval_rejected",
                    reason_code=exc.reason_code,
                    resume_action=exc.resume_action,
                )
                return {
                    "status": "blocked",
                    "blocked": True,
                    "reason_code": exc.reason_code,
                    "resume_action": exc.resume_action,
                    "dispatch": dispatch,
                }
            approval_id = str(approval["approval_id"])
            dispatch = self.storage.transition(
                dispatch_id,
                expected_from="awaiting_approval",
                to_state="queued",
                event_type="approve",
                approval_id=approval_id,
            )
        elif dispatch["state"] == "draft":
            dispatch = self.storage.transition(
                dispatch_id,
                expected_from="draft",
                to_state="queued",
                event_type="submit_open",
            )

        try:
            memory = self._recall_for_dispatch(project, stage=stage, feature=feature)
        except DeliveryBusError as exc:
            if approval_id:
                self.approvals.release(approval_id, dispatch_id=dispatch_id)
            dispatch = self.storage.transition(
                dispatch_id,
                expected_from="queued",
                to_state="blocked",
                event_type="memory_recall_failed",
                reason_code=exc.reason_code,
                resume_action=exc.resume_action,
                payload={"error": str(exc), "data": exc.data or {}},
            )
            return {
                "status": "blocked",
                "blocked": True,
                "reason_code": exc.reason_code,
                "resume_action": exc.resume_action,
                "dispatch": dispatch,
            }

        try:
            self.executor.ensure_board(project)
            receipt = self.executor.create_task(
                project,
                stage=stage,
                feature=feature,
                body=task_body(
                    project,
                    stage=stage,
                    feature=feature,
                    memory_summary=str(memory.get("summary") or ""),
                ),
                idempotency_key=idempotency_key,
            )
            dispatch = self.storage.transition(
                dispatch_id,
                expected_from="queued",
                to_state="dispatched",
                event_type="executor_created",
                approval_id=approval_id,
                executor_board=str(receipt["board"]),
                executor_task_id=str(receipt["task_id"]),
                payload={
                    "receipt": receipt,
                    "memory_injection_ref": memory.get("injection_ref"),
                    "memory_record_count": len(memory.get("records") or []),
                },
            )
            if approval_id:
                self.approvals.finalize(approval_id, dispatch_id=dispatch_id)
            return {
                "status": "dispatched",
                "blocked": False,
                "duplicate": not created,
                "dispatch": dispatch,
                "memory": {
                    "injection_ref": memory.get("injection_ref"),
                    "record_count": len(memory.get("records") or []),
                },
            }
        except CommandTimedOut as exc:
            dispatch = self.storage.transition(
                dispatch_id,
                expected_from="queued",
                to_state="reconciling",
                event_type="executor_unknown",
                reason_code="reconciliation_required",
                resume_action="run `adb reconcile` before retrying",
                payload={"error": str(exc), "data": exc.data or {}},
            )
            return {
                "status": "reconciling",
                "blocked": True,
                "reason_code": "reconciliation_required",
                "resume_action": "run `adb reconcile` before retrying",
                "dispatch": dispatch,
            }
        except DeliveryBusError as exc:
            if approval_id:
                self.approvals.release(approval_id, dispatch_id=dispatch_id)
            dispatch = self.storage.transition(
                dispatch_id,
                expected_from="queued",
                to_state="failed",
                event_type="executor_failed",
                reason_code=exc.reason_code,
                resume_action=exc.resume_action,
                payload={"error": str(exc), "data": exc.data or {}},
            )
            return {
                "status": "failed",
                "blocked": True,
                "reason_code": exc.reason_code,
                "resume_action": exc.resume_action,
                "dispatch": dispatch,
            }

    def reconcile(self, dispatch_id: str) -> dict[str, Any]:
        dispatch = self.storage.get_dispatch(dispatch_id)
        project = self.registry.resolve(slug=dispatch["project_slug"])
        board = dispatch.get("executor_board") or self.executor.board_for(project)
        task_id = dispatch.get("executor_task_id")
        if not task_id:
            task = self.executor.find_by_idempotency(board, dispatch["idempotency_key"])
            if task is None:
                return {
                    "status": "reconciling",
                    "blocked": True,
                    "reason_code": "executor_task_not_found",
                    "resume_action": "inspect the executor board before retrying dispatch",
                    "dispatch": dispatch,
                }
            task_id = str(task.get("id") or task.get("task_id") or "")
            dispatch = self.storage.transition(
                dispatch_id,
                expected_from="reconciling",
                to_state="dispatched",
                event_type="external_task_found",
                executor_board=board,
                executor_task_id=task_id,
                payload={"task": task},
            )
            if dispatch.get("approval_id"):
                self.approvals.finalize(dispatch["approval_id"], dispatch_id=dispatch_id)
        task = self.executor.show_task(board, task_id)
        status = str(task.get("status") or task.get("state") or "").lower()
        if status in TERMINAL_EXECUTOR_FAILURE:
            if dispatch["state"] == "dispatched":
                dispatch = self.storage.transition(
                    dispatch_id,
                    expected_from="dispatched",
                    to_state="failed",
                    event_type="worker_failed",
                    reason_code="executor_terminal_failure",
                    payload={"task": task},
                )
            writeback = self._safe_writeback(
                project_slug=project.slug,
                stage=dispatch["stage"],
                feature=dispatch["feature"],
                dispatch_id=dispatch_id,
                reason_code="executor_terminal_failure",
                payload={"status": "failed", "task": task},
            )
            return {
                "status": "failed",
                "blocked": True,
                "reason_code": "executor_terminal_failure",
                "dispatch": dispatch,
                "memory_writeback": writeback,
            }
        if status not in TERMINAL_EXECUTOR_SUCCESS:
            return {"status": dispatch["state"], "blocked": False, "remote_status": status, "dispatch": dispatch}
        if dispatch["state"] == "dispatched":
            dispatch = self.storage.transition(
                dispatch_id,
                expected_from="dispatched",
                to_state="reconciling",
                event_type="worker_succeeded",
                payload={"task": task},
            )
        closure = self.truth_gate.closure(
            project,
            stage=dispatch["stage"],
            feature=dispatch["feature"],
        )
        if not closure.get("pass"):
            return {
                "status": "reconciling",
                "blocked": True,
                "reason_code": "truth_evidence_incomplete",
                "resume_action": "complete the stage-specific truth-gate evidence, then reconcile again",
                "closure": closure,
                "dispatch": dispatch,
            }
        dispatch = self.storage.transition(
            dispatch_id,
            expected_from="reconciling",
            to_state="completed",
            event_type="closure_verified",
            payload={"closure": closure, "task": task},
        )
        writeback = self._safe_writeback(
            project_slug=project.slug,
            stage=dispatch["stage"],
            feature=dispatch["feature"],
            dispatch_id=dispatch_id,
            reason_code="",
            payload={"status": "completed", "closure": closure, "task": task},
        )
        return {
            "status": "completed",
            "blocked": False,
            "closure": closure,
            "dispatch": dispatch,
            "memory_writeback": writeback,
        }
