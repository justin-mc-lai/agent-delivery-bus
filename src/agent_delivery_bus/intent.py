"""Natural-language intent envelope: parse + confirm gate (no auto-dispatch)."""

from __future__ import annotations

import hashlib
import re
from dataclasses import dataclass
from typing import Any

from .assign import AssignmentScorer
from .errors import DeliveryBusError
from .registry import ProjectRegistry

SCHEMA_VERSION = "1.0"
ENVELOPE_SCHEMA = "adb-intent-envelope.v1"

KNOWN_STAGES = ("plan", "implement", "qa", "freeze", "release")
KNOWN_ACTIONS = (
    "parse",
    "assign",
    "approve",
    "dispatch",
    "reconcile",
    "fleet",
    "doctor",
    "boards",
    "status",
    "projects",
    "register",
    "delete",
    "restore",
)

# Verbs / stage words that are not project aliases.
_STOPWORDS = {
    "adb",
    "intent",
    "parse",
    "please",
    "帮我",
    "请",
    "一下",
    "运行",
    "执行",
    "开始",
    "做",
    "跑",
    "项目",
    "新",
    "登记",
    "注册",
    "新增",
    "立项",
    "删除",
    "移除",
    "归档",
    "恢复",
    *KNOWN_STAGES,
    *KNOWN_ACTIONS,
}


def _utterance_hash(utterance: str) -> str:
    return hashlib.sha256(utterance.encode("utf-8")).hexdigest()[:16]


def _tokenize(utterance: str) -> list[str]:
    # Keep CJK runs and alnum/hyphen tokens.
    parts = re.findall(r"[\w\-]+|[\u4e00-\u9fff]+", utterance, flags=re.UNICODE)
    return [p.strip() for p in parts if p.strip()]


@dataclass
class IntentParser:
    """Deterministic registry-backed parser. Never writes dispatch/approval state."""

    registry: ProjectRegistry

    def parse(
        self,
        utterance: str,
        *,
        project: str | None = None,
        require_project: bool = True,
    ) -> dict[str, Any]:
        text = (utterance or "").strip()
        if not text:
            return self._blocked(
                utterance=text,
                reason_code="intent_utterance_empty",
                resume_action="provide a non-empty utterance",
                action="",
                stage="",
                feature="",
                project_slug="",
                candidates=[],
            )

        tokens = _tokenize(text)
        action = self._detect_action(tokens, text)
        stage = self._detect_stage(tokens, text)
        feature = self._detect_feature(tokens, stage=stage, action=action)

        if project:
            try:
                resolved = self.registry.resolve(slug=project.strip())
            except DeliveryBusError as exc:
                return self._blocked(
                    utterance=text,
                    reason_code=str(exc.reason_code or "intent_project_unresolved"),
                    resume_action=str(exc.resume_action or "run `adb projects list`"),
                    action=action,
                    stage=stage,
                    feature=feature,
                    project_slug="",
                    candidates=[],
                )
            return self._resolved(
                utterance=text,
                action=action,
                stage=stage,
                feature=feature,
                project_slug=resolved.slug,
                candidates=[],
                confidence=0.95,
            )

        candidates = self._match_projects(tokens, text)
        if action == "register":
            # Registration targets a NEW project; no existing project resolution.
            return self._resolved(
                utterance=text,
                action="register",
                stage="",
                feature=feature,
                project_slug="",
                candidates=[],
                confidence=0.9,
            )
        if not require_project and not candidates:
            return self._resolved(
                utterance=text,
                action=action or "parse",
                stage=stage,
                feature=feature,
                project_slug="",
                candidates=[],
                confidence=0.4,
            )

        if len(candidates) > 1:
            return self._blocked(
                utterance=text,
                reason_code="intent_project_ambiguous",
                resume_action="disambiguate with `adb intent parse --utterance ... --project <slug>`",
                action=action,
                stage=stage,
                feature=feature,
                project_slug="",
                candidates=candidates,
                ambiguity_codes=["project_ambiguous"],
            )
        if len(candidates) == 0:
            return self._blocked(
                utterance=text,
                reason_code="intent_project_unresolved",
                resume_action="include a registered project slug/alias, or pass --project",
                action=action,
                stage=stage,
                feature=feature,
                project_slug="",
                candidates=[],
                ambiguity_codes=["project_unresolved"],
            )

        if not action:
            return self._blocked(
                utterance=text,
                reason_code="intent_action_unknown",
                resume_action="include an action such as plan/implement/assign/approve/dispatch",
                action="",
                stage=stage,
                feature=feature,
                project_slug=candidates[0]["slug"],
                candidates=candidates,
                ambiguity_codes=["action_unknown"],
            )

        return self._resolved(
            utterance=text,
            action=action,
            stage=stage,
            feature=feature,
            project_slug=candidates[0]["slug"],
            candidates=candidates,
            confidence=0.85 if len(candidates) == 1 else 0.6,
        )

    def _detect_action(self, tokens: list[str], text: str) -> str:
        lowered = text.casefold()
        # Prefer explicit English action tokens.
        for token in tokens:
            key = token.casefold()
            if key in KNOWN_ACTIONS:
                return key
        # Chinese / shorthand heuristics (deterministic, not NLU).
        mapping = (
            ("派工", "dispatch"),
            ("调度", "dispatch"),
            ("登记", "register"),
            ("注册", "register"),
            ("新增", "register"),
            ("立项", "register"),
            ("删除", "delete"),
            ("移除", "delete"),
            ("归档", "delete"),
            ("恢复", "restore"),
            ("批准", "approve"),
            ("拍板", "approve"),
            ("候选", "assign"),
            ("分配", "assign"),
            ("对账", "reconcile"),
            ("预检", "doctor"),
            ("看板", "boards"),
            ("舰队", "fleet"),
            ("实现", "implement"),
            ("规划", "plan"),
            ("验收", "qa"),
            ("冻结", "freeze"),
        )
        for needle, action in mapping:
            if needle in text:
                # Stage-like words map to structured stage intents, not always actions.
                if action in KNOWN_STAGES:
                    return "dispatch" if action in {"implement", "freeze"} else action
                return action
        for stage in KNOWN_STAGES:
            if stage in lowered:
                return "dispatch" if stage in {"implement", "freeze", "qa"} else stage
        return ""

    def _detect_stage(self, tokens: list[str], text: str) -> str:
        lowered = text.casefold()
        for stage in KNOWN_STAGES:
            if stage in lowered:
                return stage
        zh = {
            "规划": "plan",
            "实现": "implement",
            "验收": "qa",
            "冻结": "freeze",
            "发布": "release",
        }
        for needle, stage in zh.items():
            if needle in text:
                return stage
        for token in tokens:
            if token.casefold() in KNOWN_STAGES:
                return token.casefold()
        return ""

    def _detect_feature(self, tokens: list[str], *, stage: str, action: str) -> str:
        reserved = set(_STOPWORDS)
        reserved.update(KNOWN_STAGES)
        reserved.update(KNOWN_ACTIONS)
        # Prefer hyphenated / snake tokens that look like feature slugs.
        for token in tokens:
            key = token.casefold()
            if key in reserved:
                continue
            if any(ch in token for ch in ("-", "_")) and len(token) >= 3:
                # Skip if it exactly matches a project slug/alias.
                if self._token_project_hits(token):
                    continue
                return token
        # Fallback: last non-stopword token that is not a project hit.
        for token in reversed(tokens):
            key = token.casefold()
            if key in reserved or len(key) < 2:
                continue
            if self._token_project_hits(token):
                continue
            return token
        return ""

    def _token_project_hits(self, token: str) -> list[str]:
        key = token.strip().casefold()
        hits: list[str] = []
        for project in self.registry.list():
            names = {project.slug.casefold(), *(a.casefold() for a in project.aliases)}
            if key in names:
                hits.append(project.slug)
        return hits

    def _match_projects(self, tokens: list[str], text: str) -> list[dict[str, str]]:
        found: dict[str, dict[str, str]] = {}
        # Fixed index tokens ("5", "#5") match project.index (machine-enforced numbering).
        for token in tokens:
            digits = token.lstrip("#")
            if not digits.isdigit():
                continue
            for project in self.registry.list():
                if project.index == int(digits):
                    found.setdefault(project.slug, {"slug": project.slug, "matched": f"#{digits}"})
        # Token exact matches against slug/alias.
        for token in tokens:
            for slug in self._token_project_hits(token):
                found[slug] = {"slug": slug, "matched": token}
        # Whole-text contains slug/alias (for Chinese aliases without tokenization gaps).
        lowered = text.casefold()
        for project in self.registry.list():
            for name in (project.slug, *project.aliases):
                if name and name.casefold() in lowered:
                    found.setdefault(project.slug, {"slug": project.slug, "matched": name})
        return sorted(found.values(), key=lambda row: row["slug"])

    def _envelope(
        self,
        *,
        utterance: str,
        action: str,
        stage: str,
        feature: str,
        project_slug: str,
        candidates: list[dict[str, str]],
        confidence: float,
        ambiguity_codes: list[str],
        requires_confirmation: bool,
    ) -> dict[str, Any]:
        return {
            "schema": ENVELOPE_SCHEMA,
            "schema_version": SCHEMA_VERSION,
            "utterance_hash": _utterance_hash(utterance),
            "utterance": utterance,
            "action": action,
            "project_slug": project_slug,
            "project_candidates": candidates,
            "stage": stage,
            "feature": feature,
            "confidence": confidence,
            "ambiguity_codes": ambiguity_codes,
            "requires_confirmation": requires_confirmation,
            "requires_approval": stage in {"implement", "freeze", "release"},
            "confirmed": False,
        }

    def _resolved(
        self,
        *,
        utterance: str,
        action: str,
        stage: str,
        feature: str,
        project_slug: str,
        candidates: list[dict[str, str]],
        confidence: float,
    ) -> dict[str, Any]:
        envelope = self._envelope(
            utterance=utterance,
            action=action,
            stage=stage,
            feature=feature,
            project_slug=project_slug,
            candidates=candidates,
            confidence=confidence,
            ambiguity_codes=[],
            requires_confirmation=True,
        )
        return {
            "schema_version": SCHEMA_VERSION,
            "status": "pass",
            "blocked": False,
            "reason_code": "",
            "resume_action": "present envelope to human; confirm before assign/approve/dispatch",
            "data": {"envelope": envelope},
        }

    def _blocked(
        self,
        *,
        utterance: str,
        reason_code: str,
        resume_action: str,
        action: str,
        stage: str,
        feature: str,
        project_slug: str,
        candidates: list[dict[str, str]],
        ambiguity_codes: list[str] | None = None,
    ) -> dict[str, Any]:
        envelope = self._envelope(
            utterance=utterance,
            action=action,
            stage=stage,
            feature=feature,
            project_slug=project_slug,
            candidates=candidates,
            confidence=0.0,
            ambiguity_codes=list(ambiguity_codes or []),
            requires_confirmation=True,
        )
        return {
            "schema_version": SCHEMA_VERSION,
            "status": "blocked",
            "blocked": True,
            "reason_code": reason_code,
            "resume_action": resume_action,
            "data": {"envelope": envelope, "project_candidates": candidates},
        }


class ConfirmGate:
    """Fail-closed gate: structured CLI may run only after actor ack."""

    DISPATCH_ACTIONS = {"dispatch", "approve"}

    @staticmethod
    def allow_structured_cli(envelope: dict[str, Any], *, actor_ack: bool) -> dict[str, Any]:
        if not isinstance(envelope, dict) or envelope.get("schema") != ENVELOPE_SCHEMA:
            return {
                "allowed": False,
                "reason_code": "intent_envelope_invalid",
                "resume_action": "run `adb intent parse` and obtain a valid envelope",
            }
        if envelope.get("blocked"):
            return {
                "allowed": False,
                "reason_code": "intent_envelope_blocked",
                "resume_action": str(envelope.get("resume_action") or "resolve ambiguity first"),
            }
        if envelope.get("requires_confirmation", True) and not actor_ack and not envelope.get("confirmed"):
            return {
                "allowed": False,
                "reason_code": "intent_confirm_required",
                "resume_action": "show envelope to human and set confirmed/actor_ack before dispatch",
            }
        return {"allowed": True, "reason_code": "", "resume_action": ""}

    @staticmethod
    def assert_no_dispatch_without_confirm(envelope: dict[str, Any], *, actor_ack: bool) -> None:
        decision = ConfirmGate.allow_structured_cli(envelope, actor_ack=actor_ack)
        if not decision["allowed"]:
            raise DeliveryBusError(
                str(decision["reason_code"]),
                "Confirm gate blocked structured CLI",
                resume_action=str(decision["resume_action"]),
            )


def assign_from_envelope(registry: ProjectRegistry, envelope: dict[str, Any]) -> list[dict[str, Any]]:
    """Bridge confirmed/resolved envelope fields into assign candidates (no task_id)."""
    if not isinstance(envelope, dict):
        raise DeliveryBusError("intent_envelope_invalid", "envelope must be an object")
    project = str(envelope.get("project_slug") or "").strip()
    stage = str(envelope.get("stage") or "implement").strip() or "implement"
    feature = str(envelope.get("feature") or "").strip()
    if not feature:
        raise DeliveryBusError("feature_required", "envelope.feature is required for assign bridge")
    scorer = AssignmentScorer(registry)
    rows = scorer.candidates(stage=stage, feature=feature, project_slug=project or None)
    scorer.assert_candidates_only(rows)
    return rows


def silent_first_candidate_forbidden(candidates: list[dict[str, str]]) -> None:
    """Illegal path guard: never silently pick candidates[0] when ambiguous."""
    if len(candidates) > 1:
        raise DeliveryBusError(
            "intent_project_ambiguous",
            "Refusing silent first-candidate pick",
            resume_action="require explicit --project or human clarify",
            data={"project_candidates": candidates},
        )
