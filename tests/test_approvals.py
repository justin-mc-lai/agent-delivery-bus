from __future__ import annotations

import unittest

from agent_delivery_bus.approvals import ApprovalService, token_hash
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.storage import Storage


class ApprovalTests(unittest.TestCase):
    def setUp(self):
        self.storage = Storage(":memory:")
        self.approvals = ApprovalService(self.storage)

    def tearDown(self):
        self.storage.close()

    def test_token_is_hash_only_and_one_time(self):
        issued = self.approvals.issue(
            actor="apple",
            project_slug="beacon",
            stage="implement",
            feature="demo",
            ttl_seconds=300,
        )
        row = self.storage.conn.execute("SELECT * FROM approvals").fetchone()
        self.assertEqual(row["token_hash"], token_hash(issued["token"]))
        self.assertNotIn(issued["token"], tuple(row))
        reserved = self.approvals.reserve(
            issued["token"],
            dispatch_id="d1",
            project_slug="beacon",
            stage="implement",
            feature="demo",
        )
        self.assertEqual(reserved["state"], "reserved")
        consumed = self.approvals.finalize(issued["approval_id"], dispatch_id="d1")
        self.assertEqual(consumed["state"], "consumed")
        with self.assertRaises(DeliveryBusError) as replay:
            self.approvals.reserve(
                issued["token"],
                dispatch_id="d2",
                project_slug="beacon",
                stage="implement",
                feature="demo",
            )
        self.assertEqual(replay.exception.reason_code, "approval_already_consumed")

    def test_scope_and_reservation_ownership(self):
        issued = self.approvals.issue(
            actor="apple",
            project_slug="beacon",
            stage="freeze",
            feature="demo",
            ttl_seconds=300,
        )
        with self.assertRaises(DeliveryBusError) as mismatch:
            self.approvals.reserve(
                issued["token"],
                dispatch_id="d1",
                project_slug="beacon",
                stage="freeze",
                feature="other",
            )
        self.assertEqual(mismatch.exception.reason_code, "approval_scope_mismatch")
        self.approvals.reserve(
            issued["token"],
            dispatch_id="d1",
            project_slug="beacon",
            stage="freeze",
            feature="demo",
        )
        with self.assertRaises(DeliveryBusError) as busy:
            self.approvals.reserve(
                issued["token"],
                dispatch_id="d2",
                project_slug="beacon",
                stage="freeze",
                feature="demo",
            )
        self.assertEqual(busy.exception.reason_code, "approval_in_flight")
        released = self.approvals.release(issued["approval_id"], dispatch_id="d1")
        self.assertEqual(released["state"], "issued")


if __name__ == "__main__":
    unittest.main()
