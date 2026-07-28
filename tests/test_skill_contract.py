from __future__ import annotations

import subprocess
import unittest
from pathlib import Path

from agent_delivery_bus.install import install_skill


ROOT = Path(__file__).resolve().parents[1]
SKILL = ROOT / "skills" / "agent-delivery-bus"
TEMPLATE = ROOT / "skills" / "collaboration-rules-template"
VALIDATOR = Path.home() / ".codex" / "skills" / ".system" / "skill-creator" / "scripts" / "quick_validate.py"


class SkillContractTests(unittest.TestCase):
    def test_skill_creator_validation_and_required_files(self):
        self.assertTrue((SKILL / "SKILL.md").is_file())
        self.assertTrue((SKILL / "agents" / "openai.yaml").is_file())
        self.assertTrue((TEMPLATE / "SKILL.md").is_file())
        frontmatter = (SKILL / "SKILL.md").read_text(encoding="utf-8").split("---", 2)[1]
        keys = [line.split(":", 1)[0] for line in frontmatter.splitlines() if ":" in line]
        self.assertEqual(keys, ["name", "description"])
        if VALIDATOR.is_file():
            result = subprocess.run(
                ["python3", str(VALIDATOR), str(SKILL)],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_installer_dry_run_does_not_write(self):
        result = install_skill(SKILL, dry_run=True)
        self.assertTrue(result["dry_run"])
        self.assertEqual(len(result["actions"]), 2)


if __name__ == "__main__":
    unittest.main()
