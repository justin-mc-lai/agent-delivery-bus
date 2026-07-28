"""Agent Delivery Bus tests.

The frozen test contract invokes modules directly from a source checkout. Add
the src-layout package root before individual test modules are imported.
"""

from pathlib import Path
import sys


SRC = Path(__file__).resolve().parents[1] / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))
