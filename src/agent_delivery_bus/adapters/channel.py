"""Channel delivery adapter: outbound messaging back to the originating thread.

Delivery is a transport concern, intentionally decoupled from execution:
workers such as pi have no outbound chat channel, so reconcile must deliver
results through a channel adapter (e.g. Hermes gateway ``hermes send``).
"""

from __future__ import annotations

from typing import Any, Protocol, runtime_checkable

from ..errors import CommandFailed
from ..process import CommandRunner


@runtime_checkable
class ChannelAdapter(Protocol):
    """Delivers a text result back to a channel thread."""

    name: str

    def deliver(
        self,
        text: str,
        *,
        channel_thread: str = "",
        channel: str = "feishu",
    ) -> dict[str, Any]:
        """Post ``text`` to the channel thread. Returns delivery evidence."""


class HermesChannelAdapter:
    """Hermes-gateway-backed channel delivery (``hermes send``)."""

    name = "hermes-channel"

    def __init__(self, runner: CommandRunner | None = None):
        self.runner = runner or CommandRunner()

    def deliver(
        self,
        text: str,
        *,
        channel_thread: str = "",
        channel: str = "feishu",
    ) -> dict[str, Any]:
        target = f"{channel}:{channel_thread}" if channel_thread else channel
        result = self.runner.run(["hermes", "send", "--to", target, str(text)], timeout=30)
        if result.returncode != 0:
            raise CommandFailed(
                "hermes_deliver_failed",
                "hermes send failed",
                resume_action="check gateway/platform credentials and rerun reconcile delivery",
                data={"stderr": result.stderr[-2000:]},
            )
        return {"delivered": True, "target": target}
