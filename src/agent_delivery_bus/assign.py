"""Auto-assignment: rules + scorer produce dispatch candidates only."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from .approvals import RESTRICTED_STAGES
from .errors import DeliveryBusError
from .registry import Project, ProjectRegistry


@dataclass(frozen=True)
class ScoreRule:
    name: str
    weight: float
    reason: str


DEFAULT_RULES: tuple[ScoreRule, ...] = (
    ScoreRule("dispatchable", 40.0, "project is marked dispatchable"),
    ScoreRule("has_docs_version", 20.0, "project has docs_version"),
    ScoreRule("managed_class", 15.0, "project class is managed"),
    ScoreRule("restricted_needs_approve", 10.0, "restricted stage will still require approve"),
)


class AssignmentScorer:
    """Rule/scorer that never creates executor tasks or consumes approvals."""

    def __init__(self, registry: ProjectRegistry, *, rules: tuple[ScoreRule, ...] | None = None):
        self.registry = registry
        self.rules = rules or DEFAULT_RULES

    def score_candidate(
        self,
        project: Project,
        *,
        stage: str,
        feature: str,
    ) -> dict[str, Any]:
        stage = stage.strip().lower()
        feature = feature.strip()
        reasons: list[str] = []
        score = 0.0
        if project.dispatchable:
            score += 40.0
            reasons.append("dispatchable")
        if project.docs_version:
            score += 20.0
            reasons.append("has_docs_version")
        if project.project_class == "managed":
            score += 15.0
            reasons.append("managed_class")
        if stage in RESTRICTED_STAGES:
            score += 10.0
            reasons.append("restricted_needs_approve")
        if feature:
            score += 5.0
            reasons.append("feature_present")
        return {
            "project": project.slug,
            "stage": stage,
            "feature": feature,
            "score": score,
            "reasons": reasons,
            "requires_approval": stage in RESTRICTED_STAGES,
            # Explicitly absent fields that would imply side-effects:
            # task_id / approval_token / executor_board must never appear.
        }

    def candidates(
        self,
        *,
        stage: str,
        feature: str,
        project_slug: str | None = None,
    ) -> list[dict[str, Any]]:
        if not feature.strip():
            raise DeliveryBusError("feature_required", "feature is required for assign candidates")
        stage = stage.strip().lower() or "implement"
        if project_slug:
            projects = [self.registry.resolve(slug=project_slug)]
        else:
            projects = self.registry.list(dispatchable_only=False)
        rows = [
            self.score_candidate(project, stage=stage, feature=feature)
            for project in projects
            if project.dispatchable or project_slug
        ]
        rows.sort(key=lambda item: (-float(item["score"]), str(item["project"])))
        return rows

    def assert_candidates_only(self, rows: list[dict[str, Any]]) -> None:
        for row in rows:
            if "task_id" in row or row.get("executor_task_id"):
                raise DeliveryBusError(
                    "illegal_assign_side_effect",
                    "AssignmentScorer must not create executor tasks",
                )
            if row.get("approval_token") or row.get("token"):
                raise DeliveryBusError(
                    "illegal_assign_side_effect",
                    "AssignmentScorer must not consume or hold approve tokens",
                )
