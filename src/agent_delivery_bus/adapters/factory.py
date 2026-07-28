"""Adapter factory for example and future backends."""

from __future__ import annotations

from typing import Any

from ..errors import DeliveryBusError
from ..process import CommandRunner
from .beacon import BeaconAdapter
from .hermes import HermesAdapter
from .null import NullExecutor, NullTruthGate
from .spi import ExecutorAdapter, TruthGateAdapter


EXECUTOR_ADAPTERS = {
    "hermes": HermesAdapter,
    "null": NullExecutor,
}

TRUTH_GATE_ADAPTERS = {
    "beacon": BeaconAdapter,
    "null": NullTruthGate,
}


def create_executor(name: str = "null", *, runner: CommandRunner | None = None) -> ExecutorAdapter:
    key = (name or "null").strip().lower()
    cls = EXECUTOR_ADAPTERS.get(key)
    if cls is None:
        raise DeliveryBusError(
            "executor_adapter_unknown",
            f"Unknown executor adapter: {name!r}",
            resume_action=f"use one of: {', '.join(sorted(EXECUTOR_ADAPTERS))}",
        )
    if cls is HermesAdapter:
        return cls(runner=runner) if runner is not None else cls()
    if cls is NullExecutor:
        return cls()
    return cls(runner=runner) if runner is not None else cls()


def create_truth_gate(name: str = "null", *, runner: CommandRunner | None = None) -> TruthGateAdapter:
    key = (name or "null").strip().lower()
    cls = TRUTH_GATE_ADAPTERS.get(key)
    if cls is None:
        raise DeliveryBusError(
            "truth_gate_adapter_unknown",
            f"Unknown truth-gate adapter: {name!r}",
            resume_action=f"use one of: {', '.join(sorted(TRUTH_GATE_ADAPTERS))}",
        )
    if cls is BeaconAdapter:
        return cls(runner=runner) if runner is not None else cls()
    if cls is NullTruthGate:
        return cls()
    return cls(runner=runner) if runner is not None else cls()


def adapters_from_config(raw: dict[str, Any] | None = None, *, runner: CommandRunner | None = None) -> dict[str, Any]:
    config = raw or {}
    adapters = config.get("adapters") if isinstance(config.get("adapters"), dict) else {}
    executor_name = str(adapters.get("executor") or config.get("executor") or "null")
    truth_name = str(adapters.get("truth_gate") or config.get("truth_gate") or "null")
    return {
        "executor": create_executor(executor_name, runner=runner),
        "truth_gate": create_truth_gate(truth_name, runner=runner),
        "executor_name": executor_name,
        "truth_gate_name": truth_name,
    }
