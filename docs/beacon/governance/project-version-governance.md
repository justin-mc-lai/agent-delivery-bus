---
schema_version: "1.0"
project_root: "."
topology_kind: "single_repo"
default_execution_service: "agent-delivery-bus"
transaction_id: "tx-project-governance-adc60d706fc33338"
committed_at: "2026-08-04T01:43:24.844695+00:00"
source_hash: "adc60d706fc333384d6c808fe35e89d1d851e9643adda7a38e66b42f2f675d28"
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
