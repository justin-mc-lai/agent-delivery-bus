from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.pi_curator import CuratorService
from agent_delivery_bus.storage import Storage


def _proposal(proposal_id: str, topic: str, status: str = "approved") -> dict:
    return {
        "id": proposal_id,
        "topic": topic,
        "query_hints": ["opensource ai agent"],
        "sources": ["demo://github"],
        "rationale": "值得盯的开源雷达",
        "status": status,
        "project_profile_ref": "proj-demo",
        "account_profile_ref": "acct-demo-gzh",
        "created_at": "2026-08-11T00:00:00+00:00",
        "updated_at": "2026-08-11T00:00:00+00:00",
    }


class CuratorServiceTests(unittest.TestCase):
    def _service(self, tmp: Path) -> tuple[CuratorService, Storage]:
        storage = Storage(":memory:")
        knowledge = tmp / "knowledge"
        (knowledge / "ideas").mkdir(parents=True)
        (knowledge / "daily").mkdir(parents=True)
        (knowledge / "ideas" / "agent-radar.md").write_text(
            "本周开源 AI agent 框架更新：LangGraph/CrewAI 对比\n", encoding="utf-8"
        )
        svc = CuratorService(storage, knowledge_root=knowledge, state_root=tmp / "state")
        return svc, storage

    def test_approved_pool_and_empty_tick(self):
        with tempfile.TemporaryDirectory() as tmp:
            svc, storage = self._service(Path(tmp))
            self.assertEqual(svc.tick(limit=5)["approved_count"], 0)
            storage.upsert_boundary_proposal(_proposal("sbp-1", "开源 AI Agent 框架更新"))
            self.assertEqual(len(svc.proposals()), 1)

    def test_request_contains_anchors(self):
        with tempfile.TemporaryDirectory() as tmp:
            svc, storage = self._service(Path(tmp))
            storage.upsert_boundary_proposal(_proposal("sbp-1", "开源 AI Agent 框架更新"))
            proposal = svc.proposals()[0]
            request = svc.build_request(proposal)
            self.assertEqual(request["schema"], "curator-request.v1")
            self.assertTrue(request["anchors"])
            self.assertIn("agent-radar.md", request["anchors"][0]["path"])

    def test_knowledge_scan_fallback_without_agentmemory(self):
        with tempfile.TemporaryDirectory() as tmp:
            svc, _ = self._service(Path(tmp))
            anchors = svc.knowledge_scan(topic="AI agent 开源 框架", query_hints=["github"])
            self.assertTrue(anchors)

    def test_card_validate_rejects_without_anchor(self):
        with tempfile.TemporaryDirectory() as tmp:
            svc, storage = self._service(Path(tmp))
            storage.upsert_boundary_proposal(_proposal("sbp-1", "开源 AI Agent 框架更新"))
            proposal = svc.proposals()[0]
            request = svc.build_request(proposal)
            bad = {
                "schema": "curator-card.v1",
                "topic": "t",
                "sources": [],
                "knowledge_refs": ["docs/none.md"],
                "market_signals": [],
                "status": "curated",
                "created_at": "2026-08-11T00:00:00+00:00",
            }
            self.assertFalse(svc.validate_fill(request, bad)["pass"])

    def test_write_card_and_outside_root_blocked(self):
        with tempfile.TemporaryDirectory() as tmp:
            svc, storage = self._service(Path(tmp))
            storage.upsert_boundary_proposal(_proposal("sbp-1", "开源 AI Agent 框架更新"))
            proposal = svc.proposals()[0]
            request = svc.build_request(proposal)
            anchor = request["anchors"][0]["path"]
            card = {
                "schema": "curator-card.v1",
                "topic": "开源 AI Agent 框架更新",
                "sources": ["demo://github"],
                "knowledge_refs": [anchor],
                "market_signals": ["github trending"],
                "status": "curated",
                "created_at": "2026-08-11T00:00:00+00:00",
            }
            written = svc.apply("sbp-1", card, dispatch_id="adb_x")
            self.assertTrue(written["pass"])
            target = svc.knowledge_root / written["path"]
            self.assertTrue(target.is_file())
            self.assertIn("adb_x", target.read_text(encoding="utf-8"))
            svc.knowledge_root = Path("/")
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.write_card(proposal, card, dispatch_id="adb_y")
            self.assertEqual(ctx.exception.reason_code, "curator_write_outside_root")

    def test_apply_requires_approved(self):
        with tempfile.TemporaryDirectory() as tmp:
            svc, storage = self._service(Path(tmp))
            storage.upsert_boundary_proposal(_proposal("sbp-1", "t", status="pending"))
            with self.assertRaises(DeliveryBusError) as ctx:
                svc.apply("sbp-1", {"schema": "curator-card.v1"}, dispatch_id="")
            self.assertEqual(ctx.exception.reason_code, "curator_proposal_not_approved")

    def test_cli_surface(self):
        from agent_delivery_bus.cli import main
        from .helpers import make_project, write_registry

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            storage = Storage(":memory:")
            storage.upsert_boundary_proposal(_proposal("sbp-1", "开源 AI Agent 框架更新"))
            knowledge = root / "knowledge"
            (knowledge / "ideas").mkdir(parents=True)
            (knowledge / "ideas" / "agent-radar.md").write_text("开源 AI agent 框架更新\n", encoding="utf-8")
            config = write_registry(root / "projects.json", [make_project(root, slug="demo")])
            code = main(
                [
                    "--config",
                    str(config),
                    "--db",
                    ":memory:",
                    "curator",
                    "list",
                    "--status",
                    "approved",
                    "--knowledge-root",
                    str(knowledge),
                    "--json",
                ]
            )
            self.assertEqual(code, 0)


if __name__ == "__main__":
    unittest.main()
