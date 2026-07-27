from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.storage import Storage


class StorageTests(unittest.TestCase):
    def test_persists_and_sequences_events_after_restart(self):
        with tempfile.TemporaryDirectory() as tmp:
            db = Path(tmp) / "state.sqlite3"
            storage = Storage(db)
            row, created = storage.create_dispatch(
                idempotency_key="key",
                request_hash="hash",
                request={"a": 1},
                project_slug="demo",
                stage="plan",
                feature="feature",
            )
            self.assertTrue(created)
            storage.transition(
                row["dispatch_id"],
                expected_from="draft",
                to_state="queued",
                event_type="submit_open",
            )
            storage.close()
            reopened = Storage(db)
            restored = reopened.get_dispatch(row["dispatch_id"])
            self.assertEqual(restored["state"], "queued")
            self.assertEqual([event["sequence"] for event in restored["events"]], [1, 2])
            reopened.close()

    def test_idempotency_conflict_and_transaction_rollback(self):
        storage = Storage(":memory:")
        first, created = storage.create_dispatch(
            idempotency_key="same",
            request_hash="hash-1",
            request={"a": 1},
            project_slug="demo",
            stage="plan",
            feature="feature",
        )
        again, created_again = storage.create_dispatch(
            idempotency_key="same",
            request_hash="hash-1",
            request={"a": 1},
            project_slug="demo",
            stage="plan",
            feature="feature",
        )
        self.assertFalse(created_again)
        self.assertEqual(first["dispatch_id"], again["dispatch_id"])
        with self.assertRaises(DeliveryBusError):
            storage.create_dispatch(
                idempotency_key="same",
                request_hash="hash-2",
                request={"a": 2},
                project_slug="demo",
                stage="plan",
                feature="feature",
            )
        self.assertEqual(len(storage.list_dispatches()), 1)
        storage.close()


if __name__ == "__main__":
    unittest.main()
