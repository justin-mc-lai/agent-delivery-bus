"""Delivery service: approval, idempotent dispatch, and evidence reconciliation."""

from __future__ import annotations

import hashlib
import inspect
import json
from pathlib import Path
from typing import Any, Callable

from .adapters.memory import InMemoryMemoryAdapter
from .adapters.channel import ChannelAdapter
from .adapters.spi import (
    ExecutorAdapter,
    MemoryAdapter,
    TruthGateAdapter,
    adapter_capabilities,
)
from .approvals import ApprovalService, RESTRICTED_STAGES
from .errors import CommandTimedOut, DeliveryBusError
from .preflight import Preflight
from .registry import Project, ProjectRegistry
from .session import SessionRegistry, next_task_session
from .storage import Storage
from .worker_binding import (
    DEFAULT_BINDING_PROFILE,
    ENABLED_STAGES,
    assert_stage_enabled,
    format_binding_section,
    format_evidence_spec_section,
    resolve_worker_binding,
)
from .workflows import is_verified as _workflow_verified


TERMINAL_EXECUTOR_SUCCESS = {"done", "completed", "success", "succeeded"}
TERMINAL_EXECUTOR_FAILURE = {"blocked", "failed", "cancelled", "archived"}

# target_executor label -> executor adapter name (mirrors factory mapping).
# Used to validate custom adapter resolvers that do not yet accept
# ``target_executor``.
TARGET_EXECUTOR_ADAPTERS = {
    "pi": "pi",
    "hermes": "hermes",
    "coding": "hermes",
    "codex": "hermes",
    "claude": "hermes",
}

BUSINESS_IDEMPOTENCY_FIELDS = (
    "schema_version",
    "project_slug",
    "canonical_repo",
    "docs_version",
    "stage",
    "feature",
    "binding_profile",
)


def _tier_rank(level: str) -> int:
    return {"L0": 0, "L1": 1, "L2": 2, "L3": 3}.get(str(level).upper(), 1)


def _machine_reachable(name: str) -> bool:
    """Probe a worker machine over tailscale (ping -c1 -t2). Fallback: assume reachable."""
    import shutil
    import subprocess
    ts = shutil.which("tailscale")
    if not ts:
        return True  # tailscale CLI absent on this control machine -> don't block
    try:
        r = subprocess.run(
            [ts, "ping", name],
            capture_output=True, text=True, timeout=8,
        )
        return r.returncode == 0
    except Exception:
        return True  # probe failure is not an outage proof; be permissive


def normalized_request(
    project: Project,
    *,
    stage: str,
    feature: str,
    binding_profile: str = "",
    channel: str = "",
    channel_thread: str = "",
    actor_id: str = "",
    host_session_ref: str = "",
    target_executor: str = "",
    target_session_ref: str = "",
) -> dict[str, Any]:
    return {
        "schema_version": "1.1",
        "project_slug": project.slug,
        "canonical_repo": project.repo,
        "docs_version": project.docs_version,
        "stage": stage.strip().lower(),
        "feature": feature.strip(),
        "binding_profile": binding_profile or DEFAULT_BINDING_PROFILE,
        "channel": str(channel or "").strip(),
        "channel_thread": str(channel_thread or "").strip(),
        "actor_id": str(actor_id or "").strip(),
        "host_session_ref": str(host_session_ref or "").strip(),
        "target_executor": str(target_executor or "").strip(),
        "target_session_ref": str(target_session_ref or "").strip(),
        "resolution_source": "",
        "lease_required": False,
    }


def request_digest(request: dict[str, Any]) -> str:
    """Digest only the business essence of a request.

    Session/routing fields (channel, thread, actor, host session, target
    executor/session) are deliberately excluded: the same business task must
    share one idempotency key even when its routing context changes.
    """
    business = {key: request.get(key) for key in BUSINESS_IDEMPOTENCY_FIELDS}
    canonical = json.dumps(business, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


def _call_adapter_resolver(
    resolver: Callable[..., dict[str, Any]],
    project: Project,
    *,
    stage: str,
    target_executor: str,
) -> dict[str, Any]:
    """Deterministic resolver dispatch via declared capability or signature.

    Resolvers that declare ``resolver_capabilities`` are called with the
    surface they advertise. Resolvers without the attribute are called
    by their declared signature (no exception sniffing): ``lambda p: ...``
    keeps working, while a signature with ``stage`` / ``target_executor``
    receives the full context.
    """
    caps = getattr(resolver, "resolver_capabilities", None)
    if isinstance(caps, dict):
        if caps.get("target_executor"):
            return resolver(project, stage=stage, target_executor=target_executor)
        if caps.get("stage"):
            return resolver(project, stage=stage)
        return resolver(project)
    params = inspect.signature(resolver).parameters
    has_var_keyword = any(
        param.kind is inspect.Parameter.VAR_KEYWORD for param in params.values()
    )
    if "target_executor" in params or has_var_keyword:
        return resolver(project, stage=stage, target_executor=target_executor)
    if "stage" in params:
        return resolver(project, stage=stage)
    return resolver(project)


def task_body(
    project: Project,
    *,
    stage: str,
    feature: str,
    memory_summary: str = "",
    dispatch_id: str = "",
    binding_profile: str = "",
    profile_config: dict[str, Any] | None = None,
) -> str:
    binding = resolve_worker_binding(
        stage=stage,
        feature=feature,
        docs_version=project.docs_version or "",
        binding_profile=binding_profile,
        profile_config=profile_config,
        project_repo=project.repo,
        dispatch_id=dispatch_id,
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
        "",
        format_evidence_spec_section(binding.get("evidence_spec") or {}),
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
        channel_adapter: ChannelAdapter | None = None,
        adapter_resolver: Callable[[Project], dict[str, Any]] | None = None,
        workflow_root: Path | None = None,
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
        self.adapter_resolver = adapter_resolver
        self.workflow_root = workflow_root
        self.memory = memory or InMemoryMemoryAdapter()
        self.channel_adapter = channel_adapter
        self.preflight = preflight or Preflight(self.truth_gate, self.executor)
        self.approvals = ApprovalService(storage)

    def _adapters_for(
        self,
        project: Project,
        *,
        stage: str = "",
        target_executor: str = "",
    ) -> dict[str, Any]:
        """Resolve per-project adapters, falling back to the global pair.

        When ``target_executor`` is supplied (explicit flag or session
        binding), it overrides the adapter choice. Resolver calls are
        dispatched by ``resolver_capabilities`` / declared signature; a
        mismatch between the bound target and the resolved executor fails
        closed instead of probing with ``TypeError``.
        """
        if self.adapter_resolver is not None:
            adapters = _call_adapter_resolver(
                self.adapter_resolver,
                project,
                stage=stage,
                target_executor=target_executor,
            )
            requested = str(target_executor or "").strip().lower()
            if requested:
                expected = TARGET_EXECUTOR_ADAPTERS.get(requested, requested)
                actual = str(adapters.get("executor_name") or "").strip().lower()
                if actual and actual != expected:
                    raise DeliveryBusError(
                        "executor_mismatch",
                        (
                            f"session bound target_executor={requested!r} requires the "
                            f"{expected!r} executor, but the project resolved {actual!r}"
                        ),
                        resume_action=(
                            "update the session binding or the project executor_policy "
                            "so the bound target matches the executor adapter"
                        ),
                    )
            return adapters
        return {
            "executor": self.executor,
            "truth_gate": self.truth_gate,
            "binding_profile": project.binding_profile or DEFAULT_BINDING_PROFILE,
            "executor_name": getattr(self.executor, "name", ""),
            "truth_gate_name": getattr(self.truth_gate, "name", ""),
        }

    @staticmethod
    def _missing_skills(executor: ExecutorAdapter, skills: list[str]) -> list[str]:
        """Fail-closed: if the executor can verify bound skills, require them."""
        if not skills:
            return []
        check = getattr(executor, "skills_available", None)
        if not callable(check):
            return []
        try:
            result = check(skills)
        except Exception:  # noqa: BLE001 - unknown state is treated as missing
            return list(skills)
        missing = result.get("missing") if isinstance(result, dict) else None
        return [str(s) for s in (missing or []) if s]

    def _recall_for_dispatch(self, project: Project, *, stage: str, feature: str) -> dict[str, Any]:
        query = f"{project.slug} {stage} {feature}".strip()
        return self.memory.recall(project_slug=project.slug, query=query, limit=8)

    def _deliver(self, dispatch: dict[str, Any], text: str) -> dict[str, Any]:
        request = dispatch.get("request") if isinstance(dispatch.get("request"), dict) else {}
        thread = str(request.get("channel_thread") or "").strip()
        if not thread:
            return {"skipped": True}
        try:
            project = self.registry.resolve(slug=dispatch["project_slug"])
        except DeliveryBusError:
            return {"skipped": True}
        channel = str(request.get("channel") or "feishu")
        if self.channel_adapter is not None:
            deliver = self.channel_adapter.deliver
        else:
            executor = self._adapters_for(project)["executor"]
            deliver = getattr(executor, "deliver", None)
            if not callable(deliver):
                return {
                    "skipped": True,
                    "reason_code": "deliver_not_supported",
                    "reason": "no channel adapter and executor has no outbound channel",
                }
        try:
            deliver(text, channel_thread=thread, channel=channel)
            return {"delivered": True, "channel_thread": thread}
        except Exception as exc:  # noqa: BLE001 - delivery must not break reconcile
            return {"delivered": False, "reason_code": "deliver_failed", "reason": str(exc)[:200]}

    def _release_lease(self, dispatch: dict[str, Any]) -> dict[str, Any]:
        request = dispatch.get("request") if isinstance(dispatch.get("request"), dict) else {}
        session_ref = str(request.get("target_session_ref") or "").strip()
        if not session_ref or not request.get("lease_required"):
            return {}
        try:
            return SessionRegistry(self.storage).release(session_ref, dispatch["dispatch_id"])
        except DeliveryBusError:
            return {}

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

    def _pick_backup_machine(self, wanted: str, capability: str, tier: str) -> str:
        """Pick a healthy registered machine as failover for an unreachable one."""
        machines = self.storage.list_machines(capability=capability or None)
        for m in machines:
            name = str(m.get("name") or "")
            if name == wanted:
                continue
            if _tier_rank(str(m.get("permission_level") or "L1")) < _tier_rank(tier):
                continue
            if _machine_reachable(name):
                return name
        return ""

    def dispatch(
        self,
        *,
        project_slug: str,
        stage: str,
        feature: str,
        approval_token: str = "",
        dry_run: bool = False,
        reversible: bool = False,
        forced_idempotency_key: str = "",
        channel: str = "",
        channel_thread: str = "",
        actor_id: str = "",
        host_session_ref: str = "",
        target_executor: str = "",
        target_session_ref: str = "",
    ) -> dict[str, Any]:
        if channel.strip() and not channel_thread.strip():
            raise DeliveryBusError(
                "session_identity_incomplete",
                "channel requires channel_thread for session-aware dispatch",
                resume_action="pass --channel-thread or run `adb session bind` first",
            )
        project = self.registry.resolve(slug=project_slug)
        if not project.dispatchable:
            raise DeliveryBusError("project_not_dispatchable", f"Project {project.slug} is not dispatchable")
        feature = feature.strip()
        if not feature:
            raise DeliveryBusError("feature_required", "feature is required")
        stage = assert_stage_enabled(stage)

        session_registry: SessionRegistry | None = None
        resolved_target = ""
        resolution_source = ""
        if channel_thread:
            session_registry = SessionRegistry(self.storage)
            resolved_target = str(target_executor or "").strip()
            if resolved_target:
                resolution_source = "explicit"
            else:
                try:
                    binding = session_registry.resolve_by_thread(
                        channel=channel,
                        channel_thread=channel_thread,
                        actor_id=actor_id,
                        host_session=host_session_ref,
                    )
                    resolved_target = str(binding.get("target_executor") or "").strip()
                    if resolved_target:
                        resolution_source = "binding"
                except DeliveryBusError:
                    pass
            if not resolved_target:
                policy = project.metadata.get("executor_policy") if isinstance(project.metadata.get("executor_policy"), dict) else {}
                stages_map = policy.get("stages") if isinstance(policy.get("stages"), dict) else {}
                resolved_target = str(stages_map.get(stage) or "").strip()
                resolution_source = "policy" if resolved_target else "channel_default"
            resolved_target = resolved_target or "hermes"

        # Session-aware adapter resolution: a session-bound target (explicit
        # or binding) overrides the project/global executor, so a thread bound
        # to pi really dispatches through PiExecutorAdapter. Mismatches fail
        # closed in _adapters_for.
        adapters = self._adapters_for(project, stage=stage, target_executor=resolved_target)
        executor = adapters["executor"]
        truth_gate = adapters["truth_gate"]
        binding_profile = str(adapters["binding_profile"] or "")
        workflows_cfg = (
            self.registry.raw.get("workflows")
            if isinstance(self.registry.raw.get("workflows"), dict)
            else {}
        )
        profile_config = project.metadata.get("binding_profile")
        if not isinstance(profile_config, dict):
            profile_config = workflows_cfg.get(binding_profile) if isinstance(workflows_cfg, dict) else None
        profile_config = profile_config if isinstance(profile_config, dict) else None

        request = normalized_request(
            project,
            stage=stage,
            feature=feature,
            binding_profile=binding_profile,
            channel=channel,
            channel_thread=channel_thread,
            actor_id=actor_id,
            host_session_ref=host_session_ref,
            target_executor=target_executor,
            target_session_ref=target_session_ref,
        )
        if resolved_target:
            request["target_executor"] = resolved_target
            request["resolution_source"] = resolution_source
        # f9: bind project executor_machine / executor_agent into the request (fall back to project metadata)
        request.setdefault("executor_machine", str(project.metadata.get("executor_machine") or ""))
        request.setdefault("executor_agent", str(project.metadata.get("executor_agent") or "hermes"))
        # f8 R3: enforce machine permission level against task tier (fail-closed)
        if request.get("executor_machine"):
            m = self.storage.get_machine(str(request["executor_machine"]))
            if m is None:
                raise DeliveryBusError(
                    "machine_not_registered",
                    f"executor_machine {request['executor_machine']!r} is not registered",
                    resume_action="run `adb machines register` first",
                )
            task_tier = {"plan": "L1", "qa": "L1", "implement": "L2", "freeze": "L3", "release": "L3"}.get(stage, "L1")
            m_level = str(m.get("permission_level") or "L1")
            if _tier_rank(m_level) < _tier_rank(task_tier):
                raise DeliveryBusError(
                    "machine_permission_insufficient",
                    f"machine {request['executor_machine']} (level {m_level}) cannot run {stage} (needs {task_tier})",
                    resume_action="raise machine permission_level or pick another machine",
                )
        # f10 R3: tailscale health probe + failover (distributed balanced scheduling)
        if request.get("executor_machine"):
            wanted = str(request["executor_machine"])
            if not _machine_reachable(wanted):
                # preferred machine unreachable -> pick a healthy registered backup with same capability
                backup = self._pick_backup_machine(
                    wanted=wanted,
                    capability=request.get("executor_agent", ""),
                    tier=task_tier,
                )
                if backup:
                    request["executor_machine"] = backup
                    request["actual_machine"] = backup
                    request["failover_from"] = wanted
                else:
                    raise DeliveryBusError(
                        "no_healthy_executor_machine",
                        f"machine {wanted} unreachable and no healthy backup for tier {task_tier}",
                        resume_action="restore a machine or raise permissions",
                    )
            else:
                request["actual_machine"] = wanted
        digest = request_digest(request)
        if channel_thread:
            raw_session = str(target_session_ref or "").strip()
            if raw_session in ("", "auto"):
                request["target_session_ref"] = next_task_session(target_executor=resolved_target, seed=digest)
            else:
                request["target_session_ref"] = raw_session[6:] if raw_session.startswith("fixed:") else raw_session
                request["lease_required"] = True
        idempotency_key = forced_idempotency_key or f"adb-v1-{digest}"
        if self.adapter_resolver is not None:
            preflight = Preflight(truth_gate, executor).run(project, stage=stage)
        else:
            preflight = self.preflight.run(project, stage=stage)
        if dry_run:
            skills = resolve_worker_binding(
                stage=stage,
                feature=feature,
                docs_version=project.docs_version or "",
                binding_profile=binding_profile,
                profile_config=profile_config,
                project_repo=project.repo,
                dispatch_id="",
            ).get("skills") or []
            missing = self._missing_skills(executor, skills)
            if missing:
                return {
                    "status": "blocked",
                    "blocked": True,
                    "reason_code": "binding_skill_missing",
                    "resume_action": (
                        f"install worker skill(s) on the executor device: {', '.join(missing)}"
                    ),
                    "dry_run": True,
                    "request": request,
                    "idempotency_key": idempotency_key,
                    "board": executor.board_for(project),
                    "workspace": executor.workspace_for(project, stage=stage),
                    "preflight": preflight,
                    "missing_skills": missing,
                }
            return {
                "status": "blocked" if preflight["blocked"] else "pass",
                "blocked": preflight["blocked"],
                "reason_code": preflight["reason_code"],
                "resume_action": preflight["resume_action"],
                "dry_run": True,
                "request": request,
                "idempotency_key": idempotency_key,
                "board": executor.board_for(project),
                "workspace": executor.workspace_for(project, stage=stage),
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
        if created and session_registry is not None and request.get("lease_required"):
            try:
                session_registry.acquire(str(request.get("target_session_ref") or ""), dispatch_id)
            except DeliveryBusError as exc:
                dispatch = self.storage.transition(
                    dispatch_id,
                    expected_from="draft",
                    to_state="blocked",
                    event_type="session_busy",
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
        # f3 R4: reversible L2 implement is agent-autonomous (no human approval).
        # gate marks reversible=true only after denying irreversible markers; this is
        # defense-in-depth — a direct adb call without the gate must still approve.
        reversible_skip = bool(reversible) and stage == "implement"
        if stage in RESTRICTED_STAGES and not reversible_skip:
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
            executor.ensure_board(project)
            binding = resolve_worker_binding(
                stage=stage,
                feature=feature,
                docs_version=project.docs_version or "",
                binding_profile=binding_profile,
                profile_config=profile_config,
                project_repo=project.repo,
                dispatch_id=dispatch_id,
            )
            runner_profile = str(binding.get("runner_profile") or "coding")
            if resolved_target in {"codex", "claude", "coding"}:
                # Session-bound runner profiles map to executor assignees
                # (hermes kanban profiles; pi ignores assignee).
                runner_profile = resolved_target
            missing = self._missing_skills(executor, binding.get("skills") or [])
            if missing:
                if dispatch["state"] == "queued":
                    dispatch = self.storage.transition(
                        dispatch_id,
                        expected_from="queued",
                        to_state="blocked",
                        event_type="binding_skill_missing",
                        reason_code="binding_skill_missing",
                        resume_action=(
                            f"install worker skill(s) on the executor device: {', '.join(missing)}"
                        ),
                        payload={"missing_skills": missing},
                    )
                return {
                    "status": "blocked",
                    "blocked": True,
                    "reason_code": "binding_skill_missing",
                    "resume_action": (
                        f"install worker skill(s) on the executor device: {', '.join(missing)}"
                    ),
                    "dispatch": dispatch,
                    "missing_skills": missing,
                }
            if (
                self.workflow_root is not None
                and binding_profile in workflows_cfg
                and not _workflow_verified(self.workflow_root, binding_profile, workflow=profile_config)
            ):
                if dispatch["state"] == "queued":
                    dispatch = self.storage.transition(
                        dispatch_id,
                        expected_from="queued",
                        to_state="blocked",
                        event_type="workflow_verify_required",
                        reason_code="workflow_verify_required",
                        resume_action=f"run `adb workflow verify --name {binding_profile}` before real dispatch",
                    )
                return {
                    "status": "blocked",
                    "blocked": True,
                    "reason_code": "workflow_verify_required",
                    "resume_action": f"run `adb workflow verify --name {binding_profile}` before real dispatch",
                    "dispatch": dispatch,
                }
            task_body_text = self._task_body_with_session(
                    project,
                    stage=stage,
                    feature=feature,
                    memory_summary=str(memory.get("summary") or ""),
                    dispatch_id=dispatch_id,
                    binding_profile=binding_profile,
                    profile_config=profile_config,
                    request=request,
                )
            caps = adapter_capabilities(executor)
            create_kwargs: dict[str, Any] = {}
            if caps.get("task_skills", False):
                create_kwargs["skills"] = binding.get("skills") or []
            if caps.get("task_session", False):
                create_kwargs["session_id"] = request.get("target_session_ref") or ""
            receipt = executor.create_task(
                project,
                stage=stage,
                feature=feature,
                body=task_body_text,
                idempotency_key=idempotency_key,
                assignee=runner_profile,
                **create_kwargs,
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

    @staticmethod
    def _task_body_with_session(
        project: Project,
        *,
        stage: str,
        feature: str,
        memory_summary: str,
        dispatch_id: str,
        binding_profile: str,
        profile_config: dict[str, Any] | None,
        request: dict[str, Any],
    ) -> str:
        body = task_body(
            project,
            stage=stage,
            feature=feature,
            memory_summary=memory_summary,
            dispatch_id=dispatch_id,
            binding_profile=binding_profile,
            profile_config=profile_config,
        )
        if request.get("channel_thread"):
            body += (
                "\n\n### Session context\n"
                f"- channel: {request.get('channel') or ''}\n"
                f"- channel_thread: {request.get('channel_thread') or ''}\n"
                f"- actor_id: {request.get('actor_id') or ''}\n"
                f"- host_session_ref: {request.get('host_session_ref') or ''}\n"
                f"- target_executor: {request.get('target_executor') or ''}\n"
                f"- target_session_ref: {request.get('target_session_ref') or ''}\n"
                "- result must be delivered back to channel_thread after reconcile"
            )
        return body

    def reconcile(self, dispatch_id: str) -> dict[str, Any]:
        dispatch = self.storage.get_dispatch(dispatch_id)
        try:
            project = self.registry.resolve(slug=dispatch["project_slug"])
        except DeliveryBusError as exc:
            # A dispatch may reference a project that was archived/removed
            # from the registry. Park it as blocked so it leaves the pending
            # set instead of failing every reconcile round.
            try:
                dispatch = self.storage.transition(
                    dispatch_id,
                    expected_from=("dispatched", "reconciling"),
                    to_state="blocked",
                    event_type="project_unresolved",
                    reason_code=exc.reason_code,
                    resume_action=exc.resume_action,
                )
            except DeliveryBusError:
                dispatch = self.storage.get_dispatch(dispatch_id)
            return {
                "status": "blocked",
                "blocked": True,
                "reason_code": exc.reason_code,
                "resume_action": exc.resume_action,
                "dispatch": dispatch,
            }
        adapters = self._adapters_for(project)
        executor = adapters["executor"]
        truth_gate = adapters["truth_gate"]
        board = dispatch.get("executor_board") or executor.board_for(project)
        task_id = dispatch.get("executor_task_id")
        if not task_id:
            task = executor.find_by_idempotency(board, dispatch["idempotency_key"])
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
        task = executor.show_task(board, task_id)
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
            lease_release = self._release_lease(dispatch)
            delivery = self._deliver(
                dispatch,
                f"blocked {dispatch['stage']}/{dispatch['feature']} dispatch={dispatch_id} ({status})",
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
                "delivery": delivery,
                "lease_release": lease_release,
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
        closure = truth_gate.closure(
            project,
            stage=dispatch["stage"],
            feature=dispatch["feature"],
            dispatch_id=dispatch_id,
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
        lease_release = self._release_lease(dispatch)
        delivery = self._deliver(
            dispatch,
            f"completed {dispatch['stage']}/{dispatch['feature']} dispatch={dispatch_id}",
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
            "delivery": delivery,
            "lease_release": lease_release,
        }
