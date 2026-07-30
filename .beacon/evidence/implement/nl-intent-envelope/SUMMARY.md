# Implement summary — nl-intent-envelope

- IntentParser + ConfirmGate in `src/agent_delivery_bus/intent.py`
- CLI: `adb intent parse --utterance ... [--project] --json`
- Skill confirm-gate contract updated
- Tests: `tests/test_intent_parse.py`, `tests/test_intent_confirm_gate.py`
- Full suite: 50 passed
