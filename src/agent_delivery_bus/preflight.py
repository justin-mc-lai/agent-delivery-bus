"""Read-only preflight orchestration over core checks + adapter checks."""

from __future__ import annotations

from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

from .adapters.spi import ExecutorAdapter, TruthGateAdapter, as_check
from .process import CommandRunner
from .registry import Project


@dataclass(frozen=True)
class Check:
    name: str
    passed: bool
    reason_code: str = ""
    resume_action: str = ""
    detail: dict[str, Any] | None = None

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


def _from_dict(item: dict[str, Any]) -> Check:
    return Check(
        name=str(item.get("name") or "check"),
        passed=bool(item.get("passed")),
        reason_code=str(item.get("reason_code") or ""),
        resume_action=str(item.get("resume_action") or ""),
        detail=item.get("detail") if isinstance(item.get("detail"), dict) else {},
    )


class Preflight:
    def __init__(
        self,
        truth_gate: TruthGateAdapter | None = None,
        executor: ExecutorAdapter | None = None,
        runner: CommandRunner | None = None,
        *,
        # Backward-compatible constructor aliases used by older tests/call sites.
        beacon: TruthGateAdapter | None = None,
        hermes: ExecutorAdapter | None = None,
        which_command=None,
    ):
        del which_command  # CLI availability is owned by adapters now.
        self.runner = runner or CommandRunner()
        self.truth_gate = truth_gate or beacon
        self.executor = executor or hermes
        if self.truth_gate is None or self.executor is None:
            raise ValueError("Preflight requires both truth_gate and executor adapters")

    def run(self, project: Project, *, stage: str) -> dict[str, Any]:
        checks: list[Check] = []
        repo = Path(project.repo)
        checks.append(
            _from_dict(
                as_check(
                    "repo_exists",
                    repo.is_dir(),
                    reason_code="repo_missing",
                    resume_action=f"restore or correct the registered repo path for {project.slug}",
                )
            )
        )
        git_ok = False
        git_detail: dict[str, Any] = {}
        if repo.is_dir():
            git = self.runner.run(
                ["git", "-C", project.repo, "rev-parse", "--is-inside-work-tree"],
                timeout=15,
            )
            git_ok = git.returncode == 0 and git.stdout.strip() == "true"
            git_detail = {"returncode": git.returncode, "stderr": git.stderr[-1000:]}
        checks.append(
            Check(
                "repo_git",
                git_ok,
                "" if git_ok else "repo_not_git",
                "initialize/restore the git repository before dispatch",
                git_detail,
            )
        )

        for item in self.truth_gate.preflight_checks(project, stage=stage):
            checks.append(_from_dict(item))
        for item in self.executor.preflight_checks(project, stage=stage):
            checks.append(_from_dict(item))

        failed = next((item for item in checks if not item.passed), None)
        return {
            "status": "pass" if failed is None else "blocked",
            "blocked": failed is not None,
            "reason_code": failed.reason_code if failed else "",
            "resume_action": failed.resume_action if failed else "",
            "project": project.slug,
            "stage": stage,
            "checks": [item.to_dict() for item in checks],
        }
