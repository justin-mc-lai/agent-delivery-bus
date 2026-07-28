from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass
class DeliveryBusError(Exception):
    reason_code: str
    message: str
    resume_action: str = ""
    data: dict[str, Any] | None = None

    def __str__(self) -> str:
        return self.message


class CommandFailed(DeliveryBusError):
    pass


class CommandTimedOut(DeliveryBusError):
    pass
