"""Pi curator: approved topic pool -> knowledge anchors -> host-filled topic cards.

ADB stays deterministic: it inventories knowledge anchors, builds a curation
request, validates the host (pi) fill, and writes the topic card into the
declared knowledge root. ADB never calls an external LLM itself.
"""

from __future__ import annotations

import json
import os
import re
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from .boundary import BoundaryService
from .errors import DeliveryBusError
from .storage import Storage


REQUEST_SCHEMA = "curator-request.v1"
CARD_SCHEMA = "curator-card.v1"

CARD_FIELDS = ("topic", "sources", "knowledge_refs", "market_signals", "status", "created_at")
SEARCH_DIRS = ("ideas", "daily", "bases")


def _now() -> str:
    return datetime.now(timezone.utc).isoformat()


def _slugify(text: str) -> str:
    slug = re.sub(r"[^a-z0-9\u4e00-\u9fff]+", "-", text.strip().lower()).strip("-")
    return slug[:60] or "topic"


class CuratorService:
    def __init__(
        self,
        storage: Storage,
        *,
        boundary: BoundaryService | None = None,
        knowledge_root: str | Path | None = None,
        state_root: str | Path | None = None,
    ):
        self.storage = storage
        self.boundary = boundary or BoundaryService(storage)
        self.knowledge_root = Path(knowledge_root).expanduser().resolve() if knowledge_root else Path.cwd() / "knowledge"
        self.state_root = Path(state_root).expanduser().resolve() if state_root else Path.cwd() / ".beacon" / "state" / "curator"

    def proposals(self, *, status: str = "approved", limit: int | None = None) -> list[dict[str, Any]]:
        rows = self.boundary.list(status=status)
        if limit is not None:
            rows = rows[: max(1, int(limit))]
        return rows

    def knowledge_scan(self, *, topic: str, query_hints: list[str] | None = None, limit: int = 8) -> list[dict[str, str]]:
        if not self.knowledge_root.is_dir():
            return []
        tokens = {
            token
            for raw in (topic, *(query_hints or []))
            for token in re.findall(r"[\w\u4e00-\u9fff]{2,}", str(raw or "").casefold())
        }
        anchors: list[dict[str, str]] = []
        for sub in SEARCH_DIRS:
            root = self.knowledge_root / sub
            if not root.is_dir():
                continue
            for path in sorted(root.rglob("*.md")):
                if path.stat().st_size > 128 * 1024:
                    continue
                try:
                    text = path.read_text(encoding="utf-8", errors="replace")
                except OSError:
                    continue
                lowered = text.casefold()
                hits = [t for t in tokens if t in lowered]
                if hits:
                    anchors.append(
                        {
                            "path": str(path.relative_to(self.knowledge_root)),
                            "kind": sub,
                            "hits": ",".join(sorted(hits)[:6]),
                            "excerpt": re.sub(r"\s+", " ", text)[:400],
                        }
                    )
                if len(anchors) >= max(1, int(limit)):
                    break
            if len(anchors) >= max(1, int(limit)):
                break
        return anchors

    def build_request(self, proposal: dict[str, Any], *, anchors: list[dict[str, str]] | None = None) -> dict[str, Any]:
        proposal_id = str(proposal.get("id") or proposal.get("proposal_id") or "")
        if not proposal_id:
            raise DeliveryBusError("curator_proposal_invalid", "proposal id is required")
        anchors = anchors if anchors is not None else self.knowledge_scan(topic=str(proposal.get("topic") or ""), query_hints=proposal.get("query_hints") or [])
        return {
            "schema": REQUEST_SCHEMA,
            "schema_version": "1.0",
            "proposal_id": proposal_id,
            "topic": str(proposal.get("topic") or ""),
            "sources": list(proposal.get("sources") or []),
            "query_hints": list(proposal.get("query_hints") or []),
            "rationale": str(proposal.get("rationale") or ""),
            "anchors": anchors,
            "prompt": (
                f"Curate topic card for proposal {proposal_id}. "
                "Fill a curator-card.v1 response: topic, sources[], knowledge_refs[] "
                "(each ref must cite an anchor path above), market_signals[], status, created_at."
            ),
        }

    def validate_fill(self, request: dict[str, Any], response: dict[str, Any]) -> dict[str, Any]:
        problems: list[str] = []
        if response.get("schema") != CARD_SCHEMA:
            problems.append(f"response schema must be {CARD_SCHEMA}")
        for field in CARD_FIELDS:
            if field not in response:
                problems.append(f"missing field: {field}")
        anchor_paths = {str(a.get("path") or "") for a in (request.get("anchors") or [])}
        refs = response.get("knowledge_refs") if isinstance(response.get("knowledge_refs"), list) else []
        if not refs or not any(str(r) in anchor_paths for r in refs):
            problems.append("knowledge_refs must cite at least one anchor path")
        if not response.get("topic"):
            problems.append("topic is required")
        return {"pass": not problems, "problems": problems, "problems_count": len(problems)}

    def write_card(
        self,
        proposal: dict[str, Any],
        card: dict[str, Any],
        *,
        dispatch_id: str = "",
    ) -> dict[str, Any]:
        root = self.knowledge_root.resolve()
        target_dir = root / "ideas"
        target = (target_dir / f"{_slugify(str(proposal.get('topic') or 'topic'))}-{_now()[:10]}.md").resolve()
        if root == Path("/") or not str(target).startswith(str(root) + os.sep):
            raise DeliveryBusError(
                "curator_write_outside_root",
                "topic card target escapes knowledge_root",
                resume_action="keep knowledge_root/ideas as the only write target",
            )
        target_dir.mkdir(parents=True, exist_ok=True)
        frontmatter = {
            "schema": CARD_SCHEMA,
            "proposal_id": str(proposal.get("id") or ""),
            "dispatch_id": dispatch_id,
            "topic": card.get("topic") or proposal.get("topic") or "",
            "sources": card.get("sources") or proposal.get("sources") or [],
            "knowledge_refs": card.get("knowledge_refs") or [],
            "market_signals": card.get("market_signals") or [],
            "status": str(card.get("status") or "curated"),
            "created_at": card.get("created_at") or _now(),
        }
        body = (
            "---\n"
            + "\n".join(f"{k}: {json.dumps(v, ensure_ascii=False)}" for k, v in frontmatter.items())
            + "\n---\n\n"
            + f"# {frontmatter['topic']}\n\n"
            + f"Rationale: {proposal.get('rationale') or ''}\n"
        )
        target.write_text(body, encoding="utf-8")
        self._append_ledger(
            {
                "event": "card_written",
                "proposal_id": frontmatter["proposal_id"],
                "dispatch_id": dispatch_id,
                "path": str(target.relative_to(root)),
                "topic": frontmatter["topic"],
                "status": frontmatter["status"],
                "created_at": frontmatter["created_at"],
            }
        )
        return {"pass": True, "path": str(target.relative_to(root)), "frontmatter": frontmatter}

    def apply(
        self,
        proposal_id: str,
        response: dict[str, Any],
        *,
        dispatch_id: str = "",
    ) -> dict[str, Any]:
        proposal = self.boundary.show(proposal_id)
        if str(proposal.get("status") or "") != "approved":
            raise DeliveryBusError(
                "curator_proposal_not_approved",
                f"proposal {proposal_id} is not approved",
                resume_action="approve the boundary proposal before curation",
            )
        request = self.build_request(proposal)
        validation = self.validate_fill(request, response)
        if not validation["pass"]:
            raise DeliveryBusError(
                "curator_fill_invalid",
                "host fill failed curation validation",
                resume_action="fix the response fields and retry apply",
                data={"problems": validation["problems"]},
            )
        return self.write_card(proposal, response, dispatch_id=dispatch_id)

    def tick(self, *, limit: int = 5) -> dict[str, Any]:
        approved = self.proposals(status="approved", limit=limit)
        self.state_root.mkdir(parents=True, exist_ok=True)
        requests: list[dict[str, Any]] = []
        for proposal in approved:
            req = self.build_request(proposal)
            req_path = self.state_root / f"{req['proposal_id']}.request.json"
            req_path.write_text(json.dumps(req, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
            requests.append({"proposal_id": req["proposal_id"], "request_path": str(req_path.relative_to(self.state_root))})
        return {"pass": True, "approved_count": len(approved), "requests": requests, "auto_apply": False}

    def _append_ledger(self, row: dict[str, Any]) -> None:
        self.state_root.mkdir(parents=True, exist_ok=True)
        with (self.state_root / "ledger.jsonl").open("a", encoding="utf-8") as fh:
            fh.write(json.dumps({"schema": "curator-ledger.v1", **row}, ensure_ascii=False) + "\n")
