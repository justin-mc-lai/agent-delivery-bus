{
  "schema_version": "1.0",
  "feature": "search-boundary-curation",
  "feature_slug": "search-boundary-curation",
  "version": "v0.0.5",
  "run_id": "run-20260804T064443Z-sbc",
  "verdict": "passed",
  "ac_total": 7,
  "ac_accepted": 7,
  "scenario_total": 9,
  "scenario_passed": 9,
  "release_confidence_min": 1.0,
  "release_confidence_avg": 1.0,
  "review_required_count": 0,
  "needs_replay_count": 0,
  "false_complete_risk": 0.0,
  "unresolved_closure_outcomes": [],
  "junit": ".beacon/junit-search-boundary-curation.xml",
  "commands": [
    "python3 -m pytest -q tests/test_boundary.py --junitxml .beacon/junit-search-boundary-curation.xml"
  ],
  "tc_ids": [
    "TC-SBC-001",
    "TC-SBC-002",
    "TC-SBC-003",
    "TC-SBC-004",
    "TC-SBC-005",
    "TC-SBC-006",
    "TC-SBC-007",
    "TC-SBC-ILL-001",
    "TC-SBC-ILL-002"
  ],
  "ac_ids": [
    "AC-SBC-001",
    "AC-SBC-002",
    "AC-SBC-003",
    "AC-SBC-004",
    "AC-SBC-005",
    "AC-SBC-006",
    "AC-SBC-007"
  ],
  "generated_at": "2026-08-04T06:44:43.606187+00:00"
}
