"""Memory adapters: in-process (tests/null) and agentmemory REST."""

from __future__ import annotations

import json
import urllib.error
import urllib.parse
import urllib.request
import uuid
from typing import Any

from ..errors import DeliveryBusError


SCOPE_MARKER = "project_slug="


def _record(
    *,
    project_slug: str,
    content: str,
    kind: str = "fact",
    payload: dict[str, Any] | None = None,
    record_id: str = "",
) -> dict[str, Any]:
    return {
        "id": record_id or f"mem_{uuid.uuid4().hex[:16]}",
        "project": project_slug,
        "project_slug": project_slug,
        "kind": kind,
        "content": content,
        "payload": payload or {},
    }


def enforce_scope(records: list[dict[str, Any]], *, project_slug: str) -> list[dict[str, Any]]:
    """Fail closed if any hit is tagged for a different project."""
    kept: list[dict[str, Any]] = []
    for item in records:
        tagged = str(
            item.get("project")
            or item.get("project_slug")
            or item.get("projectId")
            or ""
        ).strip()
        if tagged and tagged != project_slug:
            raise DeliveryBusError(
                "memory_acl_denied",
                f"Cross-project memory hit: wanted {project_slug!r}, got {tagged!r}",
                resume_action="verify MemoryAdapter scope tags; do not broaden recall without ACL",
                data={"wanted": project_slug, "got": tagged, "record": item},
            )
        if not tagged:
            # Untagged hits are excluded, not returned under another project's scope.
            continue
        if tagged == project_slug:
            kept.append(item)
    return kept


def summarize(records: list[dict[str, Any]], *, project_slug: str) -> str:
    if not records:
        return f"Memory scope={project_slug}: (none)"
    lines = [f"Memory scope={project_slug}:"]
    for item in records:
        content = str(item.get("content") or item.get("title") or "").strip()
        if content:
            lines.append(f"- {content[:240]}")
    return "\n".join(lines)


class InMemoryMemoryAdapter:
    """Process-local store used for null/demo and unit tests."""

    name = "memory-inprocess"

    def __init__(self) -> None:
        self._records: list[dict[str, Any]] = []
        self.recall_calls: list[dict[str, Any]] = []
        self.writeback_calls: list[dict[str, Any]] = []
        self.fail_writeback = False
        self.unavailable = False

    def health(self) -> dict[str, Any]:
        if self.unavailable:
            return {"ok": False, "reason_code": "memory_unavailable"}
        return {"ok": True, "records": len(self._records)}

    def recall(
        self,
        *,
        project_slug: str,
        query: str,
        limit: int = 8,
        agent_id: str = "",
    ) -> dict[str, Any]:
        self.recall_calls.append(
            {"project_slug": project_slug, "query": query, "limit": limit, "agent_id": agent_id}
        )
        if self.unavailable:
            raise DeliveryBusError(
                "memory_unavailable",
                "Memory backend is unavailable",
                resume_action="start memory backend or switch adapters.memory to inprocess",
            )
        needle = query.strip().lower()
        hits = [
            item
            for item in self._records
            if item.get("project_slug") == project_slug
            and (not needle or needle in str(item.get("content") or "").lower())
        ]
        # Simulate leakage attempts: if any foreign records match the query text,
        # enforce_scope must fail closed when they are presented as candidates.
        foreign = [
            item
            for item in self._records
            if item.get("project_slug") != project_slug
            and needle
            and needle in str(item.get("content") or "").lower()
        ]
        candidates = hits + foreign
        scoped = enforce_scope(candidates, project_slug=project_slug)[: max(limit, 0)]
        injection_ref = f"inprocess:{project_slug}:{uuid.uuid4().hex[:8]}"
        return {
            "records": scoped,
            "summary": summarize(scoped, project_slug=project_slug),
            "injection_ref": injection_ref,
        }

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
        call = {
            "project_slug": project_slug,
            "stage": stage,
            "feature": feature,
            "dispatch_id": dispatch_id,
            "reason_code": reason_code,
            "payload": payload or {},
            "agent_id": agent_id,
        }
        self.writeback_calls.append(call)
        if self.unavailable or self.fail_writeback:
            raise DeliveryBusError(
                "memory_writeback_failed",
                "Memory writeback failed",
                resume_action="retry writeback; reconcile status must remain unchanged",
                data=call,
            )
        content = (
            f"[{SCOPE_MARKER}{project_slug}] dispatch={dispatch_id} "
            f"stage={stage} feature={feature} reason={reason_code}"
        )
        record = _record(
            project_slug=project_slug,
            content=content,
            kind="dispatch_evidence",
            payload={
                "project_slug": project_slug,
                "stage": stage,
                "feature": feature,
                "dispatch_id": dispatch_id,
                "reason_code": reason_code,
                **(payload or {}),
            },
        )
        self._records.append(record)
        return {"ok": True, "record": record}

    def seed(self, project_slug: str, content: str, **payload: Any) -> dict[str, Any]:
        record = _record(project_slug=project_slug, content=content, payload=payload)
        self._records.append(record)
        return record


class AgentMemoryAdapter:
    """HTTP client for local agentmemory REST (:3111)."""

    name = "agentmemory"

    def __init__(self, base_url: str = "http://127.0.0.1:3111", *, timeout: float = 8.0):
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout

    def _url(self, path: str, query: dict[str, Any] | None = None) -> str:
        base = f"{self.base_url}/agentmemory/{path.lstrip('/')}"
        if not query:
            return base
        return f"{base}?{urllib.parse.urlencode({k: v for k, v in query.items() if v is not None})}"

    def _request(
        self,
        method: str,
        path: str,
        *,
        query: dict[str, Any] | None = None,
        body: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        data = None
        headers = {"Accept": "application/json"}
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(self._url(path, query), data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                raw = resp.read().decode("utf-8")
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode("utf-8", errors="replace")
            raise DeliveryBusError(
                "memory_unavailable",
                f"agentmemory HTTP {exc.code}",
                resume_action="check agentmemory health on :3111",
                data={"path": path, "detail": detail[:500]},
            ) from exc
        except urllib.error.URLError as exc:
            raise DeliveryBusError(
                "memory_unavailable",
                f"agentmemory unreachable: {exc.reason}",
                resume_action="start `agentmemory` then retry",
            ) from exc
        if not raw.strip():
            return {}
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError as exc:
            raise DeliveryBusError(
                "memory_unavailable",
                "agentmemory returned non-JSON",
                resume_action="inspect agentmemory logs",
            ) from exc
        if not isinstance(payload, dict):
            return {"data": payload}
        return payload

    def health(self) -> dict[str, Any]:
        try:
            payload = self._request("GET", "health")
        except DeliveryBusError as exc:
            return {"ok": False, "reason_code": exc.reason_code, "message": str(exc)}
        status = str((payload.get("health") or {}).get("status") or payload.get("status") or "")
        return {"ok": status in {"healthy", "ok", ""}, "payload": payload}

    def recall(
        self,
        *,
        project_slug: str,
        query: str,
        limit: int = 8,
        agent_id: str = "",
    ) -> dict[str, Any]:
        search = self._request(
            "POST",
            "smart-search",
            body={
                "query": query or project_slug,
                "limit": max(limit, 1),
                "project": project_slug,
                "agentId": agent_id or project_slug,
            },
        )
        results = list(search.get("results") or [])
        # Enrich with /memories list so we can read project tags for ACL.
        listed = self._request("GET", "memories", query={"limit": max(limit * 4, 20), "project": project_slug})
        by_id = {
            str(item.get("id") or ""): item
            for item in (listed.get("memories") or [])
            if isinstance(item, dict)
        }
        candidates: list[dict[str, Any]] = []
        for hit in results:
            if not isinstance(hit, dict):
                continue
            mem_id = str(hit.get("obsId") or hit.get("id") or "")
            full = dict(by_id.get(mem_id) or {})
            merged = {
                "id": mem_id,
                "title": hit.get("title") or full.get("title") or "",
                "content": full.get("content") or hit.get("title") or "",
                "project": full.get("project") or hit.get("project") or "",
                "project_slug": full.get("project") or hit.get("project") or "",
                "score": hit.get("score"),
                "raw": hit,
            }
            # Infer scope marker from content when API omitted project.
            if not merged["project"] and SCOPE_MARKER in str(merged["content"]):
                marker = str(merged["content"]).split(SCOPE_MARKER, 1)[1]
                merged["project"] = marker.split("]", 1)[0].split()[0].strip()
                merged["project_slug"] = merged["project"]
            candidates.append(merged)
        # Also include listed memories for this project that did not appear in search.
        for item in by_id.values():
            mid = str(item.get("id") or "")
            if any(c.get("id") == mid for c in candidates):
                continue
            candidates.append(
                {
                    "id": mid,
                    "title": item.get("title") or "",
                    "content": item.get("content") or "",
                    "project": item.get("project") or "",
                    "project_slug": item.get("project") or "",
                }
            )
        scoped = enforce_scope(candidates, project_slug=project_slug)[: max(limit, 0)]
        injection_ref = f"agentmemory:{project_slug}:{uuid.uuid4().hex[:8]}"
        return {
            "records": scoped,
            "summary": summarize(scoped, project_slug=project_slug),
            "injection_ref": injection_ref,
        }

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
        content = (
            f"[{SCOPE_MARKER}{project_slug}] dispatch={dispatch_id} "
            f"stage={stage} feature={feature} reason={reason_code}"
        )
        body = {
            "content": content,
            "title": content[:120],
            "project": project_slug,
            "agentId": agent_id or project_slug,
            "metadata": {
                "project_slug": project_slug,
                "stage": stage,
                "feature": feature,
                "dispatch_id": dispatch_id,
                "reason_code": reason_code,
                **(payload or {}),
            },
        }
        result = self._request("POST", "remember", body=body)
        if not result.get("success") and "memory" not in result:
            raise DeliveryBusError(
                "memory_writeback_failed",
                "agentmemory remember did not succeed",
                resume_action="retry writeback; leave reconcile status unchanged",
                data={"result": result},
            )
        return {"ok": True, "record": result.get("memory") or result}
