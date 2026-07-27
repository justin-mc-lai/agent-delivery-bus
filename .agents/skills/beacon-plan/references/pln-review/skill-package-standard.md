# Skill Package Standard

Beacon public skills follow the qi-dev, OpenAI, and Anthropic skill-creator shape:

- `SKILL.md` frontmatter has `name` matching the directory.
- `description` says what the skill does, when it triggers, and its boundary.
- `SKILL.md` stays thin and links direct `references/`; large variant logic moves out of the entry file.
- References are one level deep from `SKILL.md`, with no hidden deep reference chain.
- Repeatable deterministic operations belong in scripts or Beacon CLI commands.
- Public Codex/OpenAI skills provide `agents/openai.yaml` or equivalent metadata.
- Non-trivial skills have eval, smoke, or benchmark contracts.
- Eval evidence is runtime evidence, not package truth, formal QA, or release verdict.

## Beacon-Specific Checks

- Shared v1.6.0 preamble is present.
- Harness HARD GATE is present and specific.
- Planner skills do not write truth, code, `.machine`, QA verdict, or release verdict.
- Generator skills do not self-certify completion.
- Evaluator skills do not repair implementation while giving verdict.
- Governor skills do not become a main lifecycle stage.

## `beacon-pln-review` Eval Contract

Benchmark or smoke cases must cover:

- full parity intent is not converted to MVP;
- source parity matrix missing blocks freeze/implement route;
- deferral ledger missing blocks scope downgrade;
- state-machine required but truth/tests landing missing blocks implementation route;
- planner write-truth/write-code/QA/release requests hit HARD GATE;
- findings include required schema fields;
- final output remains route recommendation, not formal QA/release verdict.
