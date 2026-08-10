"""Channel-agnostic canonical keyword map tests (AC-WF-002)."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.cli import main
from agent_delivery_bus.intent import IntentParser
from agent_delivery_bus.keywords import canonical_keywords, stage_from_keyword
from agent_delivery_bus.registry import ProjectRegistry

from .helpers import make_project, write_registry


class KeywordMapTests(unittest.TestCase):
    def test_canonical_map_covers_beacon_stages(self):
        keywords = canonical_keywords()
        self.assertEqual(keywords["schema"], "adb-keyword-map.v1")
        self.assertIn("feishu", keywords["channels"])
        self.assertIn("weixin", keywords["channels"])
        self.assertIn("line", keywords["channels"])
        for stage in ("plan", "truth", "implement", "qa", "freeze", "goal"):
            self.assertIn(stage, keywords["stages"])
            self.assertTrue(keywords["stages"][stage]["channel"])

    def test_stage_from_keyword_channel_agnostic(self):
        self.assertEqual(stage_from_keyword("规划"), "plan")
        self.assertEqual(stage_from_keyword("plan"), "plan")
        self.assertEqual(stage_from_keyword("需求"), "truth")
        self.assertEqual(stage_from_keyword("实现"), "implement")
        self.assertEqual(stage_from_keyword("验收"), "qa")
        self.assertEqual(stage_from_keyword("冻结"), "freeze")
        self.assertEqual(stage_from_keyword("长程"), "goal")

    def test_same_sentence_same_envelope_any_channel(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [make_project(root)]))
            parser = IntentParser(registry)
            envelopes = [
                parser.parse("规划 demo 的 feat-x", project="demo")["data"]["envelope"],
                parser.parse("plan demo feat-x", project="demo")["data"]["envelope"],
                parser.parse("规划 demo feat-x", project="demo")["data"]["envelope"],
            ]
            for env in envelopes[1:]:
                self.assertEqual(env["action"], envelopes[0]["action"])
                self.assertEqual(env["stage"], envelopes[0]["stage"])
                self.assertEqual(env["feature"], envelopes[0]["feature"])
                self.assertEqual(env["project_slug"], envelopes[0]["project_slug"])

    def test_cli_keywords_json(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config = write_registry(root / "projects.json", [make_project(root)])
            code = main(["--config", str(config), "--db", ":memory:", "intent", "keywords", "--json"])
            self.assertEqual(code, 0)


if __name__ == "__main__":
    unittest.main()
