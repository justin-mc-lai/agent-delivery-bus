"""Project lifecycle state machine: index enforcement, register/delete/restore, intent + CLI."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.cli import main
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.intent import IntentParser
from agent_delivery_bus.registry import ProjectRegistry
from agent_delivery_bus.service import DeliveryService
from agent_delivery_bus.storage import Storage

from .helpers import FakeHermes, PassingPreflight


def indexed_registry(
    path: Path,
    slugs: list[str],
    *,
    archived: set[str] | None = None,
    start_index: int = 1,
) -> ProjectRegistry:
    archived = archived or set()
    rows: list[dict] = []
    repos_root = path.parent / "repos"
    for i, slug in enumerate(slugs):
        repo = repos_root / slug
        repo.mkdir(parents=True)
        (repo / ".git").mkdir()
        status = "archived" if slug in archived else "active"
        rows.append(
            {
                "index": start_index + i,
                "slug": slug,
                "title": slug.title(),
                "class": "managed",
                "repo": str(repo.resolve()),
                "aliases": [f"{slug}-alias"],
                "dispatchable": status != "archived",
                "status": status,
            }
        )
        if status == "archived":
            rows[-1]["archived"] = True
    payload = {
        "schema_version": "1.0",
        "adapters": {"executor": "null", "truth_gate": "null"},
        "projects": rows,
    }
    path.write_text(json.dumps(payload, ensure_ascii=False), encoding="utf-8")
    return ProjectRegistry.load(path)


class ProjectLifecycleTests(unittest.TestCase):
    def test_list_rows_carry_machine_index(self):
        with tempfile.TemporaryDirectory() as tmp:
            registry = indexed_registry(Path(tmp) / "projects.json", ["beacon", "rolo", "creator"])
            rows = [item.to_dict() for item in registry.list()]
            self.assertEqual([row["slug"] for row in rows], ["beacon", "creator", "rolo"])
            self.assertEqual([row["index"] for row in rows], [1, 3, 2])
            self.assertEqual(rows[0]["status"], "active")

    def test_register_assigns_max_plus_one_and_persists(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = indexed_registry(root / "projects.json", ["beacon", "rolo"])
            new_repo = root / "toolx"
            new_repo.mkdir()
            project = registry.register(
                slug="toolx",
                title="ToolX",
                project_class="managed",
                repo=str(new_repo),
                aliases=("tool",),
                truth_gate="null",
            )
            self.assertEqual(project.index, 3)
            self.assertEqual(project.status, "active")
            reloaded = ProjectRegistry.load(root / "projects.json")
            self.assertEqual(reloaded.resolve(index=3).slug, "toolx")
            self.assertEqual(reloaded.resolve(alias="tool").slug, "toolx")

    def test_register_rejects_duplicate_slug_and_missing_repo(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = indexed_registry(root / "projects.json", ["beacon"])
            with self.assertRaises(DeliveryBusError) as ctx:
                registry.register(
                    slug="beacon",
                    project_class="managed",
                    repo=str(root / "beacon"),
                )
            self.assertEqual(ctx.exception.reason_code, "project_slug_duplicate")
            with self.assertRaises(DeliveryBusError) as ctx:
                registry.register(
                    slug="ghost",
                    project_class="managed",
                    repo=str(root / "does-not-exist"),
                )
            self.assertEqual(ctx.exception.reason_code, "repo_missing")

    def test_delete_soft_archives_keeps_index_idempotent(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = indexed_registry(root / "projects.json", ["beacon", "rolo"])
            deleted = registry.delete(1)
            self.assertEqual(deleted.status, "archived")
            self.assertFalse(deleted.dispatchable)
            self.assertEqual(deleted.index, 1)
            # Idempotent + resolves by alias too.
            again = registry.delete("beacon-alias")
            self.assertEqual(again.status, "archived")
            reloaded = ProjectRegistry.load(root / "projects.json")
            self.assertEqual(reloaded.resolve(index=1).slug, "beacon")
            self.assertEqual(reloaded.resolve(slug="beacon").status, "archived")
            self.assertEqual(reloaded.resolve(slug="beacon").index, 1)

    def test_restore_reactivates(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = indexed_registry(root / "projects.json", ["beacon", "rolo"], archived={"beacon"})
            restored = registry.restore(1)
            self.assertEqual(restored.status, "active")
            self.assertTrue(restored.dispatchable)
            reloaded = ProjectRegistry.load(root / "projects.json")
            self.assertEqual(reloaded.resolve(slug="beacon").status, "active")

    def test_duplicate_index_fail_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            path = root / "projects.json"
            registry = indexed_registry(path, ["beacon", "rolo"])
            rows = registry.raw["projects"]
            rows[1]["index"] = rows[0]["index"]
            path.write_text(json.dumps(registry.raw, ensure_ascii=False), encoding="utf-8")
            with self.assertRaises(DeliveryBusError) as ctx:
                ProjectRegistry.load(path)
            self.assertEqual(ctx.exception.reason_code, "project_index_duplicate")

    def test_archived_project_dispatch_blocked(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = indexed_registry(root / "projects.json", ["beacon"], archived={"beacon"})
            storage = Storage(":memory:")
            service = DeliveryService(
                registry,
                storage,
                preflight=PassingPreflight(),
                executor=FakeHermes(),
                truth_gate=FakeHermes(),
            )
            with self.assertRaises(DeliveryBusError) as ctx:
                service.dispatch(project_slug="beacon", stage="plan", feature="feature")
            self.assertEqual(ctx.exception.reason_code, "project_not_dispatchable")
            storage.close()


class ProjectLifecycleIntentTests(unittest.TestCase):
    def test_register_intent_does_not_resolve_existing_project(self):
        with tempfile.TemporaryDirectory() as tmp:
            registry = indexed_registry(Path(tmp) / "projects.json", ["beacon"])
            parsed = IntentParser(registry).parse("登记新项目 my-tool")
            self.assertFalse(parsed["blocked"], parsed)
            env = parsed["data"]["envelope"]
            self.assertEqual(env["action"], "register")
            self.assertEqual(env["feature"], "my-tool")
            self.assertEqual(env["project_slug"], "")
            self.assertTrue(env["requires_confirmation"])

    def test_delete_and_restore_intent_by_index(self):
        with tempfile.TemporaryDirectory() as tmp:
            registry = indexed_registry(
                Path(tmp) / "projects.json",
                ["beacon", "alpha", "beta", "gamma", "delta"],
            )
            parsed = IntentParser(registry).parse("删除 5")
            self.assertFalse(parsed["blocked"], parsed)
            env = parsed["data"]["envelope"]
            self.assertEqual(env["action"], "delete")
            self.assertEqual(env["project_slug"], "delta")
            restored = IntentParser(registry).parse("恢复 5")
            self.assertEqual(restored["data"]["envelope"]["action"], "restore")
            self.assertEqual(restored["data"]["envelope"]["project_slug"], "delta")


class ProjectLifecycleCliTests(unittest.TestCase):
    def test_list_numbered_and_register_delete_restore(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            config = root / "projects.json"
            indexed_registry(config, ["beacon", "rolo"])
            self.assertEqual(
                main(["--config", str(config), "--db", ":memory:", "projects", "list", "--numbered", "--json"]),
                0,
            )
            new_repo = root / "toolx"
            new_repo.mkdir()
            self.assertEqual(
                main(
                    [
                        "--config", str(config), "--db", ":memory:",
                        "projects", "register",
                        "--slug", "toolx", "--class", "managed", "--repo", str(new_repo),
                        "--json",
                    ]
                ),
                0,
            )
            registry = ProjectRegistry.load(config)
            self.assertEqual(registry.resolve(slug="toolx").index, 3)
            # Delete requires --yes (blocked exit code 2).
            self.assertEqual(
                main(["--config", str(config), "--db", ":memory:", "projects", "delete", "3", "--json"]),
                2,
            )
            self.assertEqual(
                main(
                    ["--config", str(config), "--db", ":memory:", "projects", "delete", "3", "--yes", "--json"]
                ),
                0,
            )
            self.assertEqual(ProjectRegistry.load(config).resolve(index=3).status, "archived")
            self.assertEqual(
                main(["--config", str(config), "--db", ":memory:", "projects", "restore", "3", "--json"]),
                0,
            )
            self.assertEqual(ProjectRegistry.load(config).resolve(index=3).status, "active")


if __name__ == "__main__":
    unittest.main()
