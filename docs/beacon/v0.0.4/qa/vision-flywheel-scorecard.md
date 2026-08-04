{
  "schema_version": "1.0",
  "feature": "vision-flywheel",
  "feature_slug": "vision-flywheel",
  "run_id": "run-20260804T111902421974",
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
  "project_profile_gate_status": "not_configured",
  "project_profile_id": "",
  "coverage_current": {
    "required_scenarios": 9,
    "executed_scenarios": 9,
    "coverage_rate": 1.0,
    "required_scenario_families": [
      "boundary",
      "compatibility",
      "happy",
      "negative",
      "recovery"
    ],
    "covered_scenario_families": [
      "primary"
    ],
    "missing_scenario_families": [],
    "missing_test_case_ids": []
  },
  "coverage_gap": {
    "missing_axes": [],
    "missing_replay_scenarios": [],
    "missing_browser_evidence_scenarios": [],
    "missing_scenario_families": [],
    "missing_test_case_ids": [],
    "open_closure_outcomes": []
  },
  "boundary_evidence": {
    "boundary_principles": [],
    "assumption_capture_required": true,
    "goal_driven_verification_required": true,
    "verification_item_count": 9,
    "targeted_scope_only": true
  },
  "design_truth_verdict": {
    "schema_version": "1.0",
    "status": "not_applicable",
    "blocked": false,
    "reason_codes": [],
    "source_surface": "implement-to-qa-handoff",
    "verification_surface": "qa",
    "docs_version": "v0.0.4",
    "runtime_version": "v1.6.11",
    "findings": []
  },
  "qa_outcome_taxonomy": {
    "status": "not_applicable",
    "failure_classes": [],
    "primary_class": "",
    "stable_taxonomy": [
      "product_failed",
      "test_failed",
      "runner_admission_blocked",
      "environment_blocked",
      "auth_session_blocked",
      "evidence_binding_missing",
      "coverage_gap",
      "truth_projection_gap"
    ]
  },
  "qa_attribution": {
    "schema_version": "1.0",
    "status": "not_applicable",
    "support_evidence_only": true,
    "runtime_evidence_only": true,
    "truth_authority": false,
    "verdict_authority": false,
    "release_pass_evidence": false,
    "attribution_count": 0,
    "accepted_count": 0,
    "blocked_count": 0,
    "release_blocking_count": 0,
    "proposal_count": 0,
    "family_counts": {
      "implementation_gap": 0,
      "truth_drift": 0,
      "coverage_gap": 0,
      "execution_lapse": 0,
      "skill_defect": 0,
      "runner_admission_blocker": 0,
      "evidence_insufficient": 0
    },
    "route_counts": {},
    "attributions": []
  },
  "qa_acceptance_capability": {
    "schema_version": "1.0",
    "status": "blocked",
    "blocked": true,
    "taxonomy": {
      "schema_version": "1.0",
      "status": "pass",
      "blocked": false,
      "full_layers": [
        "unit",
        "interface",
        "contract",
        "integration",
        "smoke",
        "functional",
        "regression",
        "exception",
        "compatibility",
        "browser_e2e"
      ],
      "non_browser_layers": [
        "unit",
        "interface",
        "contract",
        "integration",
        "smoke",
        "functional",
        "regression",
        "exception",
        "compatibility"
      ],
      "browser_layer": "browser_e2e",
      "qa9_layers": [
        "unit",
        "interface",
        "contract",
        "integration",
        "smoke",
        "functional",
        "regression",
        "exception",
        "compatibility"
      ],
      "requested_layers": [
        "contract"
      ],
      "unknown_layers": []
    },
    "profile_completeness": {
      "schema_version": "1.0",
      "profile": "feature-matrix",
      "browser_required": false,
      "required_layers": [
        "contract"
      ],
      "executed_layers": [
        "contract"
      ],
      "skipped_layers": [],
      "blocked_layers": [],
      "not_applicable_layers": [],
      "not_applicable_reasons": {},
      "missing_layers": [],
      "confidence_chain": {
        "schema_version": "1.0",
        "layers": [
          {
            "layer": "unit",
            "status": "missing",
            "confidence": 0.0,
            "depends_on": [],
            "gap_markers": [
              "downstream_gap"
            ],
            "downstream_gap_sources": [
              "unit"
            ]
          },
          {
            "layer": "interface",
            "status": "missing",
            "confidence": 0.0,
            "depends_on": [
              "unit"
            ],
            "gap_markers": [
              "downstream_gap"
            ],
            "downstream_gap_sources": [
              "unit",
              "interface"
            ]
          },
          {
            "layer": "contract",
            "status": "executed",
            "confidence": 0.0,
            "depends_on": [
              "unit",
              "interface"
            ],
            "gap_markers": [
              "downstream_gap"
            ],
            "downstream_gap_sources": [
              "unit",
              "interface"
            ]
          },
          {
            "layer": "integration",
            "status": "missing",
            "confidence": 0.0,
            "depends_on": [
              "unit",
              "interface",
              "contract"
            ],
            "gap_markers": [
              "downstream_gap"
            ],
            "downstream_gap_sources": [
              "unit",
              "interface",
              "integration"
            ]
          },
          {
            "layer": "smoke",
            "status": "missing",
            "confidence": 0.0,
            "depends_on": [
              "unit",
              "interface",
              "contract",
              "integration"
            ],
            "gap_markers": [
              "downstream_gap"
            ],
            "downstream_gap_sources": [
              "unit",
              "interface",
              "integration",
              "smoke"
            ]
          },
          {
            "layer": "functional",
            "status": "missing",
            "confidence": 0.0,
            "depends_on": [
              "unit",
              "interface",
              "contract",
              "integration",
              "smoke"
            ],
            "gap_markers": [
              "downstream_gap"
            ],
            "downstream_gap_sources": [
              "unit",
              "interface",
              "integration",
              "smoke",
              "functional"
            ]
          },
          {
            "layer": "regression",
            "status": "missing",
            "confidence": 0.0,
            "depends_on": [
              "unit",
              "interface",
              "contract",
              "integration",
              "smoke",
              "functional"
            ],
            "gap_markers": [
              "downstream_gap"
            ],
            "downstream_gap_sources": [
              "unit",
              "interface",
              "integration",
              "smoke",
              "functional",
              "regression"
            ]
          },
          {
            "layer": "exception",
            "status": "missing",
            "confidence": 0.0,
            "depends_on": [
              "unit",
              "interface",
              "contract",
              "integration",
              "smoke",
              "functional",
              "regression"
            ],
            "gap_markers": [
              "downstream_gap"
            ],
            "downstream_gap_sources": [
              "unit",
              "interface",
              "integration",
              "smoke",
              "functional",
              "regression",
              "exception"
            ]
          },
          {
            "layer": "compatibility",
            "status": "missing",
            "confidence": 0.0,
            "depends_on": [
              "unit",
              "interface",
              "contract",
              "integration",
              "smoke",
              "functional",
              "regression",
              "exception"
            ],
            "gap_markers": [
              "downstream_gap"
            ],
            "downstream_gap_sources": [
              "unit",
              "interface",
              "integration",
              "smoke",
              "functional",
              "regression",
              "exception",
              "compatibility"
            ]
          },
          {
            "layer": "browser_e2e",
            "status": "missing",
            "confidence": 0.0,
            "depends_on": [
              "unit",
              "interface",
              "contract",
              "integration",
              "smoke",
              "functional",
              "regression",
              "exception",
              "compatibility"
            ],
            "gap_markers": [
              "downstream_gap"
            ],
            "downstream_gap_sources": [
              "unit",
              "interface",
              "integration",
              "smoke",
              "functional",
              "regression",
              "exception",
              "compatibility",
              "browser_e2e"
            ]
          }
        ],
        "chain_direction": "unit_to_browser_e2e",
        "terminal_layer": "browser_e2e",
        "chain_broken": true,
        "downstream_gap_layers": [
          "browser_e2e",
          "compatibility",
          "exception",
          "functional",
          "integration",
          "interface",
          "regression",
          "smoke",
          "unit"
        ]
      },
      "state_machine_coverage": {},
      "is_complete": false,
      "partial": true,
      "status": "partial"
    },
    "browser_evidence_contract": {
      "status": "not_required",
      "blocked": false
    },
    "browser_runner_registry": [
      "playwright",
      "playwright-extension",
      "playwright-cli",
      "playwright-cdp",
      "browser-use",
      "agent-browser",
      "attached-browser",
      "web-access",
      "obscura-cdp"
    ],
    "non_acceptance_evidence_kinds": [
      "listing",
      "placeholder",
      "documentation_only",
      "dry_run"
    ],
    "failure_attribution_routes": {
      "product_bug": "beacon-implement",
      "implementation_gap": "beacon-implement",
      "test_script_bug": "beacon-test-case",
      "environment_blocker": "beacon-qa",
      "auth_session_blocker": "beacon-qa",
      "runner_admission_blocker": "blocked",
      "coverage_gap": "beacon-test-case",
      "truth_drift": "beacon-change",
      "evidence_insufficient": "beacon-qa"
    },
    "truth_authority": false,
    "release_pass_evidence": false
  },
  "reconciliation": {
    "required": false,
    "reason_code": "",
    "artifact_path": "",
    "artifact": {}
  },
  "deep_absorption": {
    "schema_version": "1.0",
    "status": "not_applicable",
    "eligible_for_release_supplement": false,
    "findings": [],
    "absorbed_count": 0,
    "deferred_count": 0,
    "blocked_count": 0,
    "not_applicable_count": 0
  },
  "release_supplement_input": {
    "deep_findings_absorbed": false,
    "absorbed_finding_count": 0,
    "advisory_or_blocked_finding_count": 0,
    "policy": "absorbed_deep_findings_only"
  },
  "continue_decision": "stop_for_repair",
  "stop_reasons": [
    "domain_fsm_legal_walk_missing"
  ],
  "test_case_backfill_required": false,
  "resume_command": "beacon help --project-root /Users/apple/Developer/Personal/products/agent-delivery-bus/.beacon/worktrees/v0.0.4/vision-flywheel --version v0.0.4 --feature \"vision-flywheel\"",
  "repair_handoff": {
    "action": "qa-repair",
    "reason": "qa_round_incomplete",
    "command": "beacon help --project-root /Users/apple/Developer/Personal/products/agent-delivery-bus/.beacon/worktrees/v0.0.4/vision-flywheel --version v0.0.4 --feature \"vision-flywheel\""
  },
  "release_ready": false,
  "domain_fsm_coverage": {
    "schema_version": "1.0",
    "required": true,
    "blocked": true,
    "status": "blocked",
    "reason_code": "domain_fsm_legal_walk_missing",
    "findings": [
      {
        "kind": "legal_walk_missing",
        "reason_code": "domain_fsm_legal_walk_missing",
        "text": "domain QA matrix requires ≥1 legal FSM walk row"
      },
      {
        "kind": "coverage_below_threshold",
        "reason_code": "domain_fsm_coverage_below_threshold",
        "text": "domain_fsm_coverage 0.0 < threshold 1.0"
      }
    ],
    "threshold": 1.0,
    "coverage_ratio": 0.0,
    "legal_walk": {
      "required": 1,
      "present": 0,
      "covered": 0,
      "rows": []
    },
    "illegal": {
      "required": 3,
      "present": 3,
      "covered": 3,
      "rows": [
        {
          "ac_id": "TC-FLY-001",
          "tc_id": "AC-FLY-001",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k register --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-001 | AC-FLY-001 | `python3 -m pytest -q tests/test_schedule.py -k register --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；register 成功写入调度注册表；重复 slug 幂等更新；未知引擎拒绝"
        },
        {
          "ac_id": "TC-FLY-ILL-001",
          "tc_id": "AC-FLY-007",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k illegal_dispatch --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-ILL-001 | AC-FLY-007 | `python3 -m pytest -q tests/test_schedule.py -k illegal_dispatch --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；心跳→dispatch/approve 非法"
        },
        {
          "ac_id": "TC-FLY-ILL-002",
          "tc_id": "AC-FLY-006",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k skip_should_run --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-ILL-002 | AC-FLY-006 | `python3 -m pytest -q tests/test_schedule.py -k skip_should_run --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；跳过 should-run 直达执行非法"
        }
      ]
    },
    "matrix": {
      "legal_walk_rows": [],
      "illegal_rows": [
        {
          "ac_id": "TC-FLY-001",
          "tc_id": "AC-FLY-001",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k register --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-001 | AC-FLY-001 | `python3 -m pytest -q tests/test_schedule.py -k register --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；register 成功写入调度注册表；重复 slug 幂等更新；未知引擎拒绝"
        },
        {
          "ac_id": "TC-FLY-ILL-001",
          "tc_id": "AC-FLY-007",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k illegal_dispatch --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-ILL-001 | AC-FLY-007 | `python3 -m pytest -q tests/test_schedule.py -k illegal_dispatch --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；心跳→dispatch/approve 非法"
        },
        {
          "ac_id": "TC-FLY-ILL-002",
          "tc_id": "AC-FLY-006",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k skip_should_run --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-ILL-002 | AC-FLY-006 | `python3 -m pytest -q tests/test_schedule.py -k skip_should_run --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；跳过 should-run 直达执行非法"
        }
      ],
      "other_rows": [
        {
          "ac_id": "TC-FLY-002",
          "tc_id": "AC-FLY-002",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k list_show --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-002 | AC-FLY-002 | `python3 -m pytest -q tests/test_schedule.py -k list_show --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；list 含全部条目与 quota 状态；show 返回单条目"
        },
        {
          "ac_id": "TC-FLY-003",
          "tc_id": "AC-FLY-003",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k should_run --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-003 | AC-FLY-003 | `python3 -m pytest -q tests/test_schedule.py -k should_run --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；quota 充足→run；耗尽/不健康→blocked+reason_code；无 LLM 调用"
        },
        {
          "ac_id": "TC-FLY-004",
          "tc_id": "AC-FLY-004",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k quota --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-004 | AC-FLY-004 | `python3 -m pytest -q tests/test_schedule.py -k quota --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；slot 按白名单来源记账；耗尽→throttled；无证据不计配额"
        },
        {
          "ac_id": "TC-FLY-005",
          "tc_id": "AC-FLY-005",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k ledger --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-005 | AC-FLY-005 | `python3 -m pytest -q tests/test_schedule.py -k ledger --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；心跳运行追加写事件流（entry_slug/status/evidence_refs/quota_spent）"
        },
        {
          "ac_id": "TC-FLY-006",
          "tc_id": "AC-FLY-006",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k reconcile --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-006 | AC-FLY-006 | `python3 -m pytest -q tests/test_schedule.py -k reconcile --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；缺证据保持 reconciling；证据齐→completed"
        },
        {
          "ac_id": "TC-FLY-007",
          "tc_id": "AC-FLY-007",
          "path_type": "`python3 -m pytest -q tests/test_schedule.py -k no_auto --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"`",
          "raw": "TC-FLY-007 | AC-FLY-007 | `python3 -m pytest -q tests/test_schedule.py -k no_auto --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\"` | exit_code==0；心跳不自动 approve/dispatch"
        }
      ],
      "has_legal_walk": false,
      "has_illegal": true
    },
    "aggregates": [
      "ScheduleHeartbeat"
    ],
    "state_machine_coverage": {},
    "deep_route_recommended": false
  },
  "domain_fsm_qa_blocked": true,
  "domain_fsm_qa_reason_code": "domain_fsm_legal_walk_missing",
  "design_alignment": {},
  "design_md_lint": {},
  "ui_state_matrix": {},
  "ui_design_ship_blocked": false
}
