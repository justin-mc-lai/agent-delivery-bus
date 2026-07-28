"""Adapters package.

Core code should import protocols from ``spi``. Concrete backends under this
package are examples and may be swapped through the factory.
"""

from .beacon import BeaconAdapter
from .factory import adapters_from_config, create_executor, create_truth_gate
from .hermes import HermesAdapter, board_slug
from .spi import ExecutorAdapter, TruthGateAdapter, as_check

__all__ = [
    "BeaconAdapter",
    "ExecutorAdapter",
    "HermesAdapter",
    "TruthGateAdapter",
    "adapters_from_config",
    "as_check",
    "board_slug",
    "create_executor",
    "create_truth_gate",
]
