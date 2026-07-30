from __future__ import annotations

import unittest

from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.intent import ConfirmGate


class IntentConfirmGateModuleTests(unittest.TestCase):
    """Dedicated module for TC-INT-004 / TC-INT-ILL-001 naming from tests.md."""

    def test_illegal_skip_confirm(self):
        envelope = {
            "schema": "adb-intent-envelope.v1",
            "requires_confirmation": True,
            "confirmed": False,
            "action": "dispatch",
            "project_slug": "demo",
        }
        with self.assertRaises(DeliveryBusError) as ctx:
            ConfirmGate.assert_no_dispatch_without_confirm(envelope, actor_ack=False)
        self.assertEqual(ctx.exception.reason_code, "intent_confirm_required")

    def test_confirmed_allows_structured_cli(self):
        envelope = {
            "schema": "adb-intent-envelope.v1",
            "requires_confirmation": True,
            "confirmed": True,
            "action": "dispatch",
        }
        decision = ConfirmGate.allow_structured_cli(envelope, actor_ack=False)
        self.assertTrue(decision["allowed"])


if __name__ == "__main__":
    unittest.main()
