"""Third-party workflow presets: registry, CLI, NL intents, skill enforcement gate."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.cli import main
from agent_delivery_bus.intent import IntentParser
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage
from agent_delivery_bus.workflows import (
    PRESET_SOURCE,
    build_preset,
    get_workflow,
    install_workflow,
    remove_workflow,
    workflow_names,
)

from .helpers import FakeHermes, PassingPreflight, make_project, write_registry


class WorkflowRegistryTests(unittest.TestCase):
    def test_presets_are_open_source_and_exclude_private_workflows(self):
        self.assertEqual(sorted(PRESET_SOURCE), ["openspec", "superpowers"])
        self.assertNotIn("beacon", PRESET_SOURCE)
        self.assertNotIn("beacon-goal", PRESET_SOURCE)
        aider = build_preset("superpowers")
        self.assertEqual(aider["skills"], ["superpowers"])
        self.assertIn("implement", aider["stages"])
        self.assertTrue(aider["evidence_spec"]["dispatch_id_binding"])

    def test_install_show_remove_roundtrip(self):
        raw: dict = {}
        installed = install_workflow(raw, name="my-superpowers", preset="superpowers")
        self.assertEqual(installed["skills"], ["superpowers"])
        self.assertEqual(workflow_names(raw), ["my-superpowers", "openspec", "superpowers"])
        self.assertEqual(get_workflow(raw, "my-superpowers")["stages"]["implement"]["skill"], "superpowers")
        removed = remove_workflow(raw, "my-superpowers")
        self.assertEqual(removed["name"], "Superpowers")
        self.assertNotIn("my-superpowers", workflow_names(raw))

    def test_unknown_preset_fails_closed(self):
        with self.assertRaises(Exception) as ctx:
            build_preset("beacon-goal")
        self.assertEqual(ctx.exception.reason_code, "workflow_preset_unknown")


class WorkflowCliTests(unittest.TestCase):
    def _config(self, root: Path) -> Path:
        project = make_project(root)
        return write_registry(root / "projects.json", [project])

    def test_list_install_show_remove(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config = self._config(root)
            self.assertEqual(
                main(["--config", str(config), "--db", ":memory:", "workflow", "list", "--json"]),
                0,
            )
            self.assertEqual(
                main(
                    [
                        "--config", str(config), "--db", ":memory:",
                        "workflow", "install", "--name", "my-superpowers", "--preset", "superpowers", "--json",
                    ]
                ),
                0,
            )
            self.assertEqual(
                main(["--config", str(config), "--db", ":memory:", "workflow", "show", "my-superpowers", "--json"]),
                0,
            )
            self.assertEqual(
                main(
                    [
                        "--config", str(config), "--db", ":memory:",
                        "workflow", "remove", "my-superpowers", "--json",
                    ]
                ),
                2,  # confirmation required
            )
            self.assertEqual(
                main(
                    [
                        "--config", str(config), "--db", ":memory:",
                        "workflow", "remove", "my-superpowers", "--yes", "--json",
                    ]
                ),
                0,
            )
            registry = ProjectRegistry.load(config)
            self.assertNotIn("my-superpowers", workflow_names(registry.raw))


class WorkflowIntentTests(unittest.TestCase):
    def test_workflow_nl_intents(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [make_project(root)]))
            parser = IntentParser(registry)

            installed = parser.parse("安装工作流 speckit")
            env = installed["data"]["envelope"]
            self.assertEqual(env["action"], "workflow_install")
            self.assertEqual(env["feature"], "speckit")
            self.assertTrue(env["requires_confirmation"])

            listed = parser.parse("列出工作流")
            self.assertEqual(listed["data"]["envelope"]["action"], "workflow_list")

            removed = parser.parse("删除工作流 my-superpowers")
            self.assertEqual(removed["data"]["envelope"]["action"], "workflow_remove")
            self.assertEqual(removed["data"]["envelope"]["feature"], "my-superpowers")


class GatedExecutor(FakeHermes):
    def __init__(self, *, missing: tuple[str, ...] = ()):
        super().__init__(remote_status="done")
        self.missing = list(missing)

    def skills_available(self, skills: list[str]) -> dict:
        return {"missing": [s for s in skills if s in self.missing], "installed": []}


class WorkflowSkillGateTests(unittest.TestCase):
    def test_missing_bound_skill_blocks_dispatch(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            config = write_registry(root / "projects.json", [project])
            raw = json.loads(config.read_text(encoding="utf-8"))
            raw["workflows"] = {"my-superpowers": build_preset("superpowers")}
            raw["projects"][0]["binding_profile"] = "my-superpowers"
            config.write_text(json.dumps(raw, ensure_ascii=False), encoding="utf-8")
            registry = ProjectRegistry.load(config)
            storage = Storage(":memory:")
            hermes = GatedExecutor(missing=("superpowers",))
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                executor=hermes,
                truth_gate=GatedExecutor(),
            )
            result = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(result["status"], "blocked")
            self.assertEqual(result["reason_code"], "binding_skill_missing")
            self.assertEqual(result["missing_skills"], ["superpowers"])
            self.assertEqual(hermes.create_count, 0)
            storage.close()

    def test_bound_skill_forwarded_to_executor(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            config = write_registry(root / "projects.json", [project])
            raw = json.loads(config.read_text(encoding="utf-8"))
            raw["workflows"] = {"my-superpowers": build_preset("superpowers")}
            raw["projects"][0]["binding_profile"] = "my-superpowers"
            config.write_text(json.dumps(raw, ensure_ascii=False), encoding="utf-8")
            registry = ProjectRegistry.load(config)
            storage = Storage(":memory:")
            hermes = GatedExecutor()
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                executor=hermes,
                truth_gate=GatedExecutor(),
            )
            result = service.dispatch(project_slug="demo", stage="plan", feature="feature")
            self.assertEqual(result["status"], "dispatched")
            self.assertIn("superpowers", hermes.last_skills)
            self.assertIn("### Worker binding", hermes.last_body)
            self.assertNotIn("beacon_skill", hermes.last_body)
            storage.close()


if __name__ == "__main__":
    unittest.main()
