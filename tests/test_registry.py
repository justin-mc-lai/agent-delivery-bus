from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from agent_delivery_bus.cli import main
from agent_delivery_bus.errors import DeliveryBusError
from agent_delivery_bus.registry import ProjectRegistry

from .helpers import make_project, write_registry


class RegistryTests(unittest.TestCase):
    def test_list_and_resolve_slug_alias_path(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            first = make_project(root, slug="beacon")
            second = make_project(root, slug="managed")
            registry = ProjectRegistry.load(write_registry(root / "projects.json", [first, second]))
            self.assertEqual([item.slug for item in registry.list()], ["beacon", "managed"])
            self.assertEqual(registry.resolve(slug="beacon").slug, "beacon")
            self.assertEqual(registry.resolve(alias="managed-alias").slug, "managed")
            self.assertEqual(registry.resolve(path=second.repo).slug, "managed")

    def test_cli_resolve_numeric_slug_maps_to_index(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            first = make_project(root, slug="beacon")
            second = make_project(root, slug="managed")
            object.__setattr__(first, "index", 1)
            object.__setattr__(second, "index", 2)
            registry_path = write_registry(root / "projects.json", [first, second])
            registry = ProjectRegistry.load(registry_path)
            self.assertEqual(registry.resolve(index=2).slug, "managed")
            self.assertEqual(registry.resolve(index=1).slug, "beacon")
            rc = main(["--config", str(registry_path), "projects", "resolve", "--slug", "2", "--json"])
            self.assertEqual(rc, 0)
            rc = main(["--config", str(registry_path), "projects", "resolve", "--index", "2", "--json"])
            self.assertEqual(rc, 0)

    def test_duplicate_slug_and_alias_conflict_fail_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            first = make_project(root, slug="one")
            duplicate = first
            with self.assertRaisesRegex(DeliveryBusError, "duplicate"):
                ProjectRegistry.load(write_registry(root / "dupe.json", [first, duplicate]))
            second = make_project(root, slug="two")
            object.__setattr__(second, "aliases", first.aliases)
            with self.assertRaises(DeliveryBusError) as raised:
                ProjectRegistry.load(write_registry(root / "alias.json", [first, second]))
            self.assertEqual(raised.exception.reason_code, "project_alias_ambiguous")

    def test_missing_repo_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            project = make_project(root)
            Path(project.repo).rename(root / "gone")
            with self.assertRaises(DeliveryBusError) as raised:
                ProjectRegistry.load(write_registry(root / "projects.json", [project]))
            self.assertEqual(raised.exception.reason_code, "repo_missing")


if __name__ == "__main__":
    unittest.main()
