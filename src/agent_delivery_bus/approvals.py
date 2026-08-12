from __future__ import annotations

import hashlib
import secrets
import uuid
from datetime import datetime, timedelta, timezone
from typing import Any

from .errors import DeliveryBusError
from .storage import Storage, now_iso


RESTRICTED_STAGES = {"implement", "freeze", "release"}


def token_hash(token: str) -> str:
    return hashlib.sha256(token.encode("utf-8")).hexdigest()


class ApprovalService:
    def __init__(self, storage: Storage):
        self.storage = storage

    def issue(
        self,
        *,
        actor: str,
        project_slug: str,
        stage: str,
        feature: str,
        ttl_seconds: int,
        channel_actor: str = "",
    ) -> dict[str, Any]:
        if stage not in RESTRICTED_STAGES:
            raise DeliveryBusError(
                "approval_stage_invalid",
                f"Approval is only defined for: {', '.join(sorted(RESTRICTED_STAGES))}",
            )
        if not actor.strip() or not feature.strip():
            raise DeliveryBusError("approval_scope_invalid", "actor and feature are required")
        if ttl_seconds < 30 or ttl_seconds > 86400:
            raise DeliveryBusError("approval_ttl_invalid", "ttl must be between 30 and 86400 seconds")
        token = f"adb1_{secrets.token_urlsafe(32)}"
        approval_id = f"apr_{uuid.uuid4().hex[:20]}"
        expires_at = (datetime.now(timezone.utc) + timedelta(seconds=ttl_seconds)).isoformat()
        self.storage.conn.execute(
            """
            INSERT INTO approvals(
              approval_id,token_hash,actor,channel_actor,project_slug,stage,feature,
              expires_at,state,created_at
            ) VALUES(?,?,?,?,?,?,?,?,'issued',?)
            """,
            (
                approval_id,
                token_hash(token),
                actor.strip(),
                str(channel_actor or "").strip(),
                project_slug,
                stage,
                feature,
                expires_at,
                now_iso(),
            ),
        )
        return {
            "approval_id": approval_id,
            "token": token,
            "actor": actor.strip(),
            "channel_actor": str(channel_actor or "").strip(),
            "project_slug": project_slug,
            "stage": stage,
            "feature": feature,
            "expires_at": expires_at,
            "state": "issued",
        }

    def reserve(
        self,
        token: str,
        *,
        dispatch_id: str,
        project_slug: str,
        stage: str,
        feature: str,
        channel_actor: str = "",
    ) -> dict[str, Any]:
        digest = token_hash(token)
        with self.storage.transaction():
            row = self.storage.conn.execute(
                "SELECT * FROM approvals WHERE token_hash=?",
                (digest,),
            ).fetchone()
            if row is None:
                raise DeliveryBusError("approval_invalid", "Approval token is invalid")
            approval = dict(row)
            if (
                approval["project_slug"] != project_slug
                or approval["stage"] != stage
                or approval["feature"] != feature
            ):
                raise DeliveryBusError("approval_scope_mismatch", "Approval token scope does not match")
            if datetime.fromisoformat(approval["expires_at"]) <= datetime.now(timezone.utc):
                raise DeliveryBusError("approval_expired", "Approval token has expired")
            if approval["state"] == "consumed":
                raise DeliveryBusError("approval_already_consumed", "Approval token was already consumed")
            if approval["state"] == "reserved":
                if approval["reserved_by"] == dispatch_id:
                    return approval
                raise DeliveryBusError("approval_in_flight", "Approval token is reserved by another dispatch")
            if approval["state"] != "issued":
                raise DeliveryBusError("approval_invalid", f"Approval state is {approval['state']}")
            bound_channel_actor = str(approval.get("channel_actor") or "").strip()
            if bound_channel_actor and channel_actor and bound_channel_actor != str(channel_actor or "").strip():
                raise DeliveryBusError(
                    "approval_channel_actor_mismatch",
                    "Approval channel actor does not match the dispatching actor",
                    resume_action="re-issue the approval from the originating channel identity",
                )
            updated = self.storage.conn.execute(
                """
                UPDATE approvals
                SET state='reserved', reserved_by=?, reserved_at=?
                WHERE approval_id=? AND state='issued'
                """,
                (dispatch_id, now_iso(), approval["approval_id"]),
            )
            if updated.rowcount != 1:
                raise DeliveryBusError("approval_in_flight", "Approval token reservation lost a race")
        return self.get(approval["approval_id"])

    def finalize(self, approval_id: str, *, dispatch_id: str) -> dict[str, Any]:
        updated = self.storage.conn.execute(
            """
            UPDATE approvals
            SET state='consumed', consumed_at=?
            WHERE approval_id=? AND state='reserved' AND reserved_by=?
            """,
            (now_iso(), approval_id, dispatch_id),
        )
        if updated.rowcount != 1:
            raise DeliveryBusError("approval_finalize_invalid", "Approval is not reserved by this dispatch")
        return self.get(approval_id)

    def release(self, approval_id: str, *, dispatch_id: str) -> dict[str, Any]:
        updated = self.storage.conn.execute(
            """
            UPDATE approvals
            SET state='issued', reserved_by=NULL, reserved_at=NULL
            WHERE approval_id=? AND state='reserved' AND reserved_by=?
            """,
            (approval_id, dispatch_id),
        )
        if updated.rowcount != 1:
            raise DeliveryBusError("approval_release_invalid", "Approval is not releasable by this dispatch")
        return self.get(approval_id)

    def get(self, approval_id: str) -> dict[str, Any]:
        row = self.storage.conn.execute(
            "SELECT * FROM approvals WHERE approval_id=?",
            (approval_id,),
        ).fetchone()
        if row is None:
            raise DeliveryBusError("approval_not_found", f"Approval not found: {approval_id}")
        return dict(row)
