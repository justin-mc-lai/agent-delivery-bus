from __future__ import annotations

from dataclasses import asdict, dataclass
from pathlib import Path
from shutil import which
from typing import Any

from .adapters.beacon import BeaconAdapter
from .adapters.hermes import HermesAdapter
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


class Preflight:
    def __init__(
        self,
        beacon: BeaconAdapter | None = None,
        hermes: HermesAdapter | None = None,
        runner: CommandRunner | None = None,
        which_command=None,
    ):
        self.runner = runner or CommandRunner()
        self.beacon = beacon or BeaconAdapter(self.runner)
        self.hermes = hermes or HermesAdapter(self.runner)
        self.which_command = which_command or which

    def run(self, project: Project, *, stage: str) -> dict[str, Any]:
        checks: list[Check] = []
        repo = Path(project.repo)
        checks.append(
            Check(
                "repo_exists",
                repo.is_dir(),
                "" if repo.is_dir() else "repo_missing",
                f"restore or correct the registered repo path for {project.slug}",
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
        docs_root = Path(project.beacon_docs_root)
        docs_ok = docs_root.is_dir()
        checks.append(
            Check(
                "beacon_docs_root",
                docs_ok,
                "" if docs_ok else "beacon_docs_missing",
                "restore Beacon docs or re-register the project",
            )
        )
        version_path = docs_root / project.current_docs_version
        version_ok = version_path.is_dir()
        checks.append(
            Check(
                "beacon_docs_version",
                version_ok,
                "" if version_ok else "beacon_version_mismatch",
                f"confirm current_docs_version={project.current_docs_version} in config/projects.json",
            )
        )
        beacon_cli_ok = bool(self.which_command("beacon"))
        checks.append(
            Check(
                "beacon_cli",
                beacon_cli_ok,
                "" if beacon_cli_ok else "beacon_cli_unavailable",
                "install or repair the Beacon CLI, then rerun preflight",
            )
        )
        context = (
            self.beacon.verify_context(project)
            if repo.is_dir() and beacon_cli_ok
            else {"pass": False, "payload": {}}
        )
        actual_docs_version = str(
            ((context.get("payload") or {}).get("docs_version") if isinstance(context, dict) else "")
            or ""
        )
        declared_version_ok = not actual_docs_version or actual_docs_version == project.current_docs_version
        checks.append(
            Check(
                "beacon_declared_version",
                declared_version_ok,
                "" if declared_version_ok else "beacon_version_mismatch",
                (
                    f"update config/projects.json from {project.current_docs_version} "
                    f"to the project-reported {actual_docs_version}"
                ),
                {
                    "registered": project.current_docs_version,
                    "project_reported": actual_docs_version,
                },
            )
        )
        checks.append(
            Check(
                "beacon_context_strict",
                bool(context.get("pass")),
                "" if context.get("pass") else "beacon_context_invalid",
                f"run `beacon doctor setup-context --project-root {project.repo}` and verify manually",
                context,
            )
        )
        hermes_cli_ok = bool(self.which_command("hermes"))
        checks.append(
            Check(
                "hermes_cli",
                hermes_cli_ok,
                "" if hermes_cli_ok else "hermes_cli_unavailable",
                "install or repair the Hermes CLI, then rerun preflight",
            )
        )
        health = (
            self.hermes.health(profile="coding")
            if hermes_cli_ok
            else {"gateway_pass": False, "profile_pass": False, "profiles": []}
        )
        checks.append(
            Check(
                "hermes_gateway",
                bool(health.get("gateway_pass")),
                "" if health.get("gateway_pass") else "hermes_gateway_unavailable",
                "run `hermes gateway status` and start/restart the gateway",
                health,
            )
        )
        checks.append(
            Check(
                "hermes_profile",
                bool(health.get("profile_pass")),
                "" if health.get("profile_pass") else "hermes_profile_missing",
                "create or restore the Hermes `coding` profile",
                {"profiles": health.get("profiles", [])},
            )
        )
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
