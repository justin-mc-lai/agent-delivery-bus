"""Tests for boundary approve → auto-dispatch (auto-promote) wiring.

AC-SBC-012: decide approve only auto-dispatches when the bound vertical profile
opts in (dispatch_auto: true + dispatch_project); restricted stages are never
auto-dispatched; reject never dispatches; default profiles keep old behavior.
"""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.boundary import (
    DEFAULT_ACCOUNT_PROFILE_REF,
    DEFAULT_PROJECT_PROFILE_REF,
    BoundaryService,
    derive_feature_slug,
    load_vertical_profile,
)
from agent_delivery_bus.cli import _auto_dispatch_on_approve
from agent_delivery_bus.preflight import Preflight
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from tests.helpers import FakeExecutor, FakeTruthGate, PassingPreflight, make_project, write_registry

_PROFILE_KW = {
    "project_profile_ref": DEFAULT_PROJECT_PROFILE_REF,
    "account_profile_ref": DEFAULT_ACCOUNT_PROFILE_REF,
    "rationale": "示例·开源 AI / AI Spec 价值选题",
}


def _make_service(tmp: str) -> DeliveryService:
    root = Path(tmp)
    project = make_project(root, slug="selfmedia-creator")
    registry_path = write_registry(root / "projects.json", [project])
    registry = ProjectRegistry.load(registry_path)
    storage = Storage(root / "db.sqlite3")
    executor = FakeExecutor()
    truth_gate = FakeTruthGate()
    preflight = PassingPreflight()
    return DeliveryService(
        registry,
        storage,
        preflight=preflight,
        executor=executor,
        truth_gate=truth_gate,
    )


class DeriveFeatureSlugTests(unittest.TestCase):
    def test_ascii_tokens_joined(self):
        self.assertEqual(
            derive_feature_slug("Agent 编排框架选型：LangGraph / CrewAI / AutoGen 怎么挑", "sbp-abc"),
            "agent-langgraph-crewai-autogen",
        )

    def test_stopwords_dropped(self):
        self.assertEqual(
            derive_feature_slug("The Rise of Open Source AI Agents", "sbp-abc"),
            "rise-open-source-ai-agents",
        )

    def test_pure_cjk_falls_back_to_topic_id(self):
        slug = derive_feature_slug("系统提示写作规范与边界", "sbp-9c34b9b7e086")
        self.assertTrue(slug.startswith("topic-"))
        self.assertIn("9c34b9b7e086", slug)

    def test_mixed_cjk_uses_ascii_tokens(self):
        # "给 AI 写一份好的系统提示" carries a single ASCII token → falls back
        self.assertEqual(derive_feature_slug("给 AI 写一份好的系统提示", "sbp-9c34b9b7e086"), "topic-9c34b9b7e086")

    def test_single_token_falls_back(self):
        # one ASCII token is too short/non-unique → topic-id fallback
        slug = derive_feature_slug("开源向量库横向对比：chunk 策略与召回率怎么权衡", "sbp-abc123")
        self.assertTrue(slug.startswith("topic-"))
        self.assertIn("abc123", slug)

    def test_empty_topic_never_raises(self):
        self.assertEqual(derive_feature_slug("", "sbp-abc"), "topic-abc")


class AutoDispatchOnApproveTests(unittest.TestCase):
    def test_default_profile_does_not_dispatch(self):
        with tempfile.TemporaryDirectory() as tmp:
            service = _make_service(tmp)
            profile = load_vertical_profile(DEFAULT_PROJECT_PROFILE_REF)
            self.assertFalse(profile.get("dispatch_auto"))
            proposal = {
                "id": "sbp-1",
                "topic": "GitHub 开源 AI 库盘点",
                "project_profile_ref": DEFAULT_PROJECT_PROFILE_REF,
            }
            self.assertIsNone(_auto_dispatch_on_approve(service, proposal, actor="you"))
            service.storage.close()

    def test_optin_dispatches_plan(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            # local profile fixture: opt in to selfmedia-creator
            profiles = root / "fixtures" / "vertical-profiles"
            profiles.mkdir(parents=True)
            profile = dict(load_vertical_profile(DEFAULT_PROJECT_PROFILE_REF))
            profile["dispatch_auto"] = True
            profile["dispatch_project"] = "selfmedia-creator"
            profile["dispatch_stage"] = "plan"
            (profiles / f"{DEFAULT_PROJECT_PROFILE_REF}.json").write_text(
                __import__("json").dumps(profile, ensure_ascii=False, indent=2),
                encoding="utf-8",
            )
            # patch _profiles_root to the tmp fixture dir
            import agent_delivery_bus.boundary as boundary_mod

            orig = boundary_mod._profiles_root
            boundary_mod._profiles_root = lambda: profiles
            service = _make_service(tmp)
            try:
                result = _auto_dispatch_on_approve(
                    service,
                    {"id": "sbp-9c34b9b7e086", "topic": "给 AI 写一份好的系统提示", "project_profile_ref": DEFAULT_PROJECT_PROFILE_REF},
                    actor="you",
                )
            finally:
                boundary_mod._profiles_root = orig
                service.storage.close()
            self.assertIsNotNone(result)
            self.assertFalse(result["blocked"])
            self.assertEqual(result["project"], "selfmedia-creator")
            self.assertEqual(result["stage"], "plan")
            self.assertTrue(result["feature"].startswith("topic-"))
            self.assertTrue(result["dispatch_id"])

    def test_restricted_stage_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            profiles = root / "fixtures" / "vertical-profiles"
            profiles.mkdir(parents=True)
            profile = dict(load_vertical_profile(DEFAULT_PROJECT_PROFILE_REF))
            profile["dispatch_auto"] = True
            profile["dispatch_project"] = "selfmedia-creator"
            profile["dispatch_stage"] = "implement"
            (profiles / f"{DEFAULT_PROJECT_PROFILE_REF}.json").write_text(
                __import__("json").dumps(profile, ensure_ascii=False, indent=2),
                encoding="utf-8",
            )
            import agent_delivery_bus.boundary as boundary_mod

            orig = boundary_mod._profiles_root
            boundary_mod._profiles_root = lambda: profiles
            service = _make_service(tmp)
            try:
                result = _auto_dispatch_on_approve(
                    service,
                    {"id": "sbp-1", "topic": "agent framework langgraph", "project_profile_ref": DEFAULT_PROJECT_PROFILE_REF},
                    actor="you",
                )
            finally:
                boundary_mod._profiles_root = orig
                service.storage.close()
            self.assertIsNotNone(result)
            self.assertTrue(result["blocked"])
            self.assertEqual(result["reason_code"], "auto_dispatch_restricted_stage")


class DecideCliffTests(unittest.TestCase):
    def test_decide_approve_returns_proposal_without_auto_key_by_default(self):
        with tempfile.TemporaryDirectory() as tmp:
            storage = Storage(Path(tmp) / "db.sqlite3")
            svc = BoundaryService(storage)
            row = svc.ingest(topic="approve-me github ai", **_PROFILE_KW)
            approved = svc.decide(row["id"], actor="you", decision="approve")
            self.assertEqual(approved["status"], "approved")
            # service-level decide has no auto-dispatch side effects
            self.assertNotIn("auto_dispatch", approved)
            storage.close()


if __name__ == "__main__":
    unittest.main()
