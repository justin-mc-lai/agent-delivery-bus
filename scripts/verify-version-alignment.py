#!/usr/bin/env python3
"""Verify version-truth-catalog projections (CI + local release gate).

Usage:
    python3 scripts/verify-version-alignment.py [--check-tag] [--root <repo>]
"""

from __future__ import annotations

import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "src"))

from agent_delivery_bus.version_truth import main  # noqa: E402


if __name__ == "__main__":
    raise SystemExit(main())
