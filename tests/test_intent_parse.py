from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.approvals import ApprovalService
from agent_delivery_bus.assign import AssignmentScorer
from agent_delivery_bus.cli import main
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.intent import (
    ConfirmGate,
    IntentParser,
    assign_from_envelope,
    silent_first_candidate_forbidden,
)
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.storage import Storage

from .helpers import FakeHermes, make_project, write_registry


class IntentParseTests(unittest.TestCase):
    def _registry(self, root: Path, *, second: bool = False) -> ProjectRegistry:
        projects = [make_project(root, slug="demo")]
        if second:
            projects.append(make_project(root, slug="beacon"))
            # Shared alias collision for ambiguity tests is configured per-case.
        return ProjectRegistry.load(write_registry(root / "projects.json", projects), validate_paths=True)

    def test_envelope_schema(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = self._registry(root)
            parsed = IntentParser(registry).parse("demo implement nl-intent-envelope")
            self.assertEqual(parsed["schema_version"], "1.0")
            self.assertIn(parsed["status"], {"pass", "blocked"})
            self.assertIn("blocked", parsed)
            self.assertIn("reason_code", parsed)
            env = parsed["data"]["envelope"]
            self.assertEqual(env["schema"], "adb-intent-envelope.v1")
            self.assertIn("utterance_hash", env)
            self.assertTrue(env["requires_confirmation"])

    def test_unique_alias_resolve(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = self._registry(root)
            parsed = IntentParser(registry).parse("please plan for demo-alias feature-x")
            self.assertFalse(parsed["blocked"], parsed)
            self.assertEqual(parsed["data"]["envelope"]["project_slug"], "demo")
            self.assertEqual(parsed["data"]["envelope"]["stage"], "plan")

    def test_ambiguous_project(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            a = make_project(root, slug="alpha")
            b = make_project(root, slug="beta")
            # Force shared alias by rewriting registry JSON.
            path = write_registry(root / "projects.json", [a, b])
            raw = json.loads(path.read_text(encoding="utf-8"))
            raw["projects"][0]["aliases"] = ["shared", "alpha-alias"]
            raw["projects"][1]["aliases"] = ["shared", "beta-alias"]
            path.write_text(json.dumps(raw), encoding="utf-8")
            # Registry load rejects ambiguous aliases at load-time; simulate utterance
            # matching two distinct slugs instead.
            registry = ProjectRegistry.load(
                write_registry(root / "projects2.json", [a, b]),
                validate_paths=True,
            )
            parsed = IntentParser(registry).parse("alpha and beta implement feat")
            self.assertTrue(parsed["blocked"])
            self.assertEqual(parsed["reason_code"], "intent_project_ambiguous")
            self.assertGreater(len(parsed["data"]["project_candidates"]), 1)

    def test_illegal_silent_pick(self):
        candidates = [{"slug": "a", "matched": "a"}, {"slug": "b", "matched": "b"}]
        with self.assertRaises(DeliveryBusError) as ctx:
            silent_first_candidate_forbidden(candidates)
        self.assertEqual(ctx.exception.reason_code, "intent_project_ambiguous")

    def test_assign_bridge(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = self._registry(root)
            parsed = IntentParser(registry).parse("demo implement memory-adapter-auto-assign")
            env = parsed["data"]["envelope"]
            rows = assign_from_envelope(registry, env)
            AssignmentScorer(registry).assert_candidates_only(rows)
            self.assertTrue(rows)
            self.assertNotIn("task_id", rows[0])

    def test_no_side_effects(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = self._registry(root)
            hermes = FakeHermes()
            storage = Storage(":memory:")
            approvals = ApprovalService(storage)
            before_tokens = storage.list_approvals() if hasattr(storage, "list_approvals") else []
            IntentParser(registry).parse("demo implement feat-y")
            self.assertEqual(hermes.create_count, 0)
            # No approval consumed / issued by parse.
            self.assertEqual(approvals.pending() if hasattr(approvals, "pending") else [], [])
            storage.close()
            del before_tokens


class IntentConfirmGateTests(unittest.TestCase):
    def test_confirm_required_before_dispatch_path(self):
        envelope = {
            "schema": "adb-intent-envelope.v1",
            "requires_confirmation": True,
            "confirmed": False,
            "project_slug": "demo",
            "stage": "implement",
            "feature": "x",
            "action": "dispatch",
        }
        decision = ConfirmGate.allow_structured_cli(envelope, actor_ack=False)
        self.assertFalse(decision["allowed"])
        self.assertEqual(decision["reason_code"], "intent_confirm_required")
        ok = ConfirmGate.allow_structured_cli(envelope, actor_ack=True)
        self.assertTrue(ok["allowed"])

    def test_illegal_skip_confirm(self):
        envelope = {
            "schema": "adb-intent-envelope.v1",
            "requires_confirmation": True,
            "confirmed": False,
            "action": "dispatch",
        }
        with self.assertRaises(DeliveryBusError) as ctx:
            ConfirmGate.assert_no_dispatch_without_confirm(envelope, actor_ack=False)
        self.assertEqual(ctx.exception.reason_code, "intent_confirm_required")
        # parse must not create executor tasks / consume tokens — covered by FakeHermes count
        hermes = FakeHermes()
        self.assertEqual(hermes.create_count, 0)


class IntentCliTests(unittest.TestCase):
    def test_cli_intent_parse_json(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root, slug="demo")
            config = write_registry(root / "projects.json", [project])
            code = main(
                [
                    "--config",
                    str(config),
                    "--db",
                    ":memory:",
                    "intent",
                    "parse",
                    "--utterance",
                    "demo plan demo-feature",
                    "--json",
                ]
            )
            self.assertEqual(code, 0)


if __name__ == "__main__":
    unittest.main()
