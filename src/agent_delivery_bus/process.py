from __future__ import annotations

import json
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Sequence

from .errors import CommandFailed, CommandTimedOut


@dataclass(frozen=True)
class CommandResult:
    argv: tuple[str, ...]
    returncode: int
    stdout: str
    stderr: str

    def json(self) -> Any:
        try:
            return json.loads(self.stdout)
        except json.JSONDecodeError as exc:
            raise CommandFailed(
                "external_json_invalid",
                f"Command did not return valid JSON: {' '.join(self.argv)}",
                data={"stdout": self.stdout[-2000:], "stderr": self.stderr[-2000:]},
            ) from exc


class CommandRunner:
    def run(
        self,
        argv: Sequence[str],
        *,
        cwd: str | Path | None = None,
        timeout: float = 30.0,
        check: bool = False,
    ) -> CommandResult:
        args = tuple(str(item) for item in argv)
        try:
            proc = subprocess.run(
                args,
                cwd=str(cwd) if cwd else None,
                capture_output=True,
                text=True,
                timeout=timeout,
                check=False,
            )
        except subprocess.TimeoutExpired as exc:
            raise CommandTimedOut(
                "external_command_timeout",
                f"Command timed out: {' '.join(args)}",
                resume_action="reconcile before retrying",
                data={
                    "argv": list(args),
                    "stdout": exc.stdout if isinstance(exc.stdout, str) else "",
                    "stderr": exc.stderr if isinstance(exc.stderr, str) else "",
                },
            ) from exc
        except OSError as exc:
            raise CommandFailed(
                "external_command_unavailable",
                f"Command is unavailable: {args[0]}",
                resume_action=f"install or repair `{args[0]}` and rerun preflight",
                data={"argv": list(args), "error": str(exc)},
            ) from exc
        result = CommandResult(args, proc.returncode, proc.stdout or "", proc.stderr or "")
        if check and result.returncode != 0:
            raise CommandFailed(
                "external_command_failed",
                f"Command failed ({result.returncode}): {' '.join(args)}",
                data={"argv": list(args), "stdout": result.stdout[-2000:], "stderr": result.stderr[-2000:]},
            )
        return result
