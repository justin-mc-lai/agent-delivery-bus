---
schema_version: "1.0"
project_root: "."
topology_kind: "single_repo"
default_execution_service: "agent-delivery-bus"
transaction_id: "tx-project-governance-0aa0d7e7f7bb2d3a"
committed_at: "2026-08-12T15:28:19.186701+00:00"
source_hash: "0aa0d7e7f7bb2d3a1ffd44638988b6a8f01bce90ac13b6d6251d4134feca8175"
event_type: "freeze-version-contract"
parser_contract: "beacon-project-governance-v1"
---

# Project Version Governance

- project_root: `.`
- topology_kind: `single_repo`
- default_execution_service: `agent-delivery-bus`

## Service Bindings
- `agent-delivery-bus` repo=`.` role=`control-plane` writable=`true` worktree_policy=`follow-repo-root`

## Version Contracts

### v0.0.1
- participating_services: `agent-delivery-bus`
- branch_guard_mode: `strict`
- validation_state: `active`
- per_service_canonical_branch:
  - `agent-delivery-bus`: `main`
- truth_canonical: `main`
- branch_governance_template: `standard-feature`
- previous_version_baseline_branch: `main`
- next_version_feature_branch: `beacon/v0.0.1/<feature-slug>`
- worktree_mode: `dedicated`
- worktree_path: `.beacon/worktrees/v0.0.1`
- require_workspace_admission: `true`
- merge_direction_policy: `master->main`
- merge_direction_policy: `main->uat`
- forbidden_directions: `uat->main`
- forbidden_directions: `uat_as_feature_base`

### v0.0.2
- participating_services: `agent-delivery-bus`
- branch_guard_mode: `strict`
- validation_state: `active`
- per_service_canonical_branch:
  - `agent-delivery-bus`: `main`
- truth_canonical: `main`
- branch_governance_template: `standard-feature`
- previous_version_baseline_branch: `main`
- next_version_feature_branch: `beacon/v0.0.2/<feature-slug>`
- worktree_mode: `dedicated`
- worktree_path: `.beacon/worktrees/v0.0.2`
- require_workspace_admission: `true`
- merge_direction_policy: `master->main`
- merge_direction_policy: `main->uat`
- forbidden_directions: `uat->main`
- forbidden_directions: `uat_as_feature_base`

### v0.0.3
- participating_services: `agent-delivery-bus`
- branch_guard_mode: `strict`
- validation_state: `active`
- per_service_canonical_branch:
  - `agent-delivery-bus`: `main`
- truth_canonical: `main`
- branch_governance_template: `standard-feature`
- previous_version_baseline_branch: `main`
- next_version_feature_branch: `beacon/v0.0.3/<feature-slug>`
- worktree_mode: `dedicated`
- worktree_path: `.beacon/worktrees/v0.0.3`
- require_workspace_admission: `true`
- merge_direction_policy: `master->main`
- merge_direction_policy: `main->uat`
- forbidden_directions: `uat->main`
- forbidden_directions: `uat_as_feature_base`

### v0.0.4
- participating_services: `agent-delivery-bus`
- branch_guard_mode: `strict`
- validation_state: `active`
- per_service_canonical_branch:
  - `agent-delivery-bus`: `main`
- truth_canonical: `main`
- branch_governance_template: `standard-feature`
- previous_version_baseline_branch: `main`
- next_version_feature_branch: `beacon/v0.0.4/<feature-slug>`
- worktree_mode: `dedicated`
- worktree_path: `.beacon/worktrees/v0.0.4`
- require_workspace_admission: `true`
- merge_direction_policy: `master->main`
- merge_direction_policy: `main->uat`
- forbidden_directions: `uat->main`
- forbidden_directions: `uat_as_feature_base`

### v0.0.5
- participating_services: `agent-delivery-bus`
- branch_guard_mode: `strict`
- validation_state: `active`
- per_service_canonical_branch:
  - `agent-delivery-bus`: `beacon/v0.0.5`
- truth_canonical: `main`
- branch_governance_template: `standard-feature`
- previous_version_baseline_branch: `main`
- next_version_feature_branch: `beacon/v0.0.5/<feature-slug>`
- worktree_mode: `dedicated`
- worktree_path: `.beacon/worktrees/v0.0.5`
- require_workspace_admission: `true`
- merge_direction_policy: `master->beacon/v0.0.5`
- merge_direction_policy: `beacon/v0.0.5->uat`
- forbidden_directions: `uat->beacon/v0.0.5`
- forbidden_directions: `uat_as_feature_base`

### v0.0.6
- participating_services: `agent-delivery-bus`
- branch_guard_mode: `strict`
- validation_state: `active`
- per_service_canonical_branch:
  - `agent-delivery-bus`: `beacon/v0.0.6`
- truth_canonical: `main`
- branch_governance_template: `standard-feature`
- previous_version_baseline_branch: `main`
- next_version_feature_branch: `beacon/v0.0.6/<feature-slug>`
- worktree_mode: `dedicated`
- worktree_path: `.beacon/worktrees/v0.0.6`
- require_workspace_admission: `true`
- merge_direction_policy: `master->beacon/v0.0.6`
- merge_direction_policy: `beacon/v0.0.6->uat`
- forbidden_directions: `uat->beacon/v0.0.6`
- forbidden_directions: `uat_as_feature_base`

### v0.0.7
- participating_services: `agent-delivery-bus`
- branch_guard_mode: `strict`
- validation_state: `active`
- per_service_canonical_branch:
  - `agent-delivery-bus`: `beacon/v0.0.7`
- truth_canonical: `main`
- branch_governance_template: `standard-feature`
- previous_version_baseline_branch: `main`
- next_version_feature_branch: `beacon/v0.0.7/<feature-slug>`
- worktree_mode: `dedicated`
- worktree_path: `.beacon/worktrees/v0.0.7`
- require_workspace_admission: `true`
- merge_direction_policy: `master->beacon/v0.0.7`
- merge_direction_policy: `beacon/v0.0.7->uat`
- forbidden_directions: `uat->beacon/v0.0.7`
- forbidden_directions: `uat_as_feature_base`

### v0.1.0
- participating_services: `agent-delivery-bus`
- branch_guard_mode: `strict`
- validation_state: `active`
- per_service_canonical_branch:
  - `agent-delivery-bus`: `beacon/v0.1.0`
- truth_canonical: `main`
- branch_governance_template: `standard-feature`
- previous_version_baseline_branch: `main`
- next_version_feature_branch: `beacon/v0.1.0/<feature-slug>`
- worktree_mode: `dedicated`
- worktree_path: `.beacon/worktrees/v0.1.0`
- require_workspace_admission: `true`
- merge_direction_policy: `master->beacon/v0.1.0`
- merge_direction_policy: `beacon/v0.1.0->uat`
- forbidden_directions: `uat->beacon/v0.1.0`
- forbidden_directions: `uat_as_feature_base`

### v0.1.1
- participating_services: `agent-delivery-bus`
- branch_guard_mode: `strict`
- validation_state: `active`
- per_service_canonical_branch:
  - `agent-delivery-bus`: `beacon/v0.1.1`
- truth_canonical: `main`
- branch_governance_template: `standard-feature`
- previous_version_baseline_branch: `main`
- next_version_feature_branch: `beacon/v0.1.1/<feature-slug>`
- worktree_mode: `dedicated`
- worktree_path: `.beacon/worktrees/v0.1.1`
- require_workspace_admission: `true`
- merge_direction_policy: `master->beacon/v0.1.1`
- merge_direction_policy: `beacon/v0.1.1->uat`
- forbidden_directions: `uat->beacon/v0.1.1`
- forbidden_directions: `uat_as_feature_base`

### v0.1.2
- participating_services: `agent-delivery-bus`
- branch_guard_mode: `strict`
- validation_state: `active`
- per_service_canonical_branch:
  - `agent-delivery-bus`: `beacon/v0.1.2`
- truth_canonical: `main`
- branch_governance_template: `standard-feature`
- previous_version_baseline_branch: `main`
- next_version_feature_branch: `beacon/v0.1.2/<feature-slug>`
- worktree_mode: `dedicated`
- worktree_path: `.beacon/worktrees/v0.1.2`
- require_workspace_admission: `true`
- merge_direction_policy: `master->beacon/v0.1.2`
- merge_direction_policy: `beacon/v0.1.2->uat`
- forbidden_directions: `uat->beacon/v0.1.2`
- forbidden_directions: `uat_as_feature_base`
