---
schema_version: "1.0"
project_root: "/Users/apple/Developer/Personal/products/agent-delivery-bus"
topology_kind: "single_repo"
default_execution_service: "agent-delivery-bus"
transaction_id: "tx-project-governance-ce49cf43c411fd00"
committed_at: "2026-07-27T13:21:21.619442+00:00"
source_hash: "ce49cf43c411fd007225224c45028bb8f460b1e26345ba3ac5cc8659a2ea14a5"
event_type: "update"
parser_contract: "beacon-project-governance-v1"
---

# Project Version Governance

- project_root: `/Users/apple/Developer/Personal/products/agent-delivery-bus`
- topology_kind: `single_repo`
- default_execution_service: `agent-delivery-bus`

## Service Bindings
- `agent-delivery-bus` repo=`/Users/apple/Developer/Personal/products/agent-delivery-bus` role=`control-plane` writable=`true` worktree_policy=`follow-repo-root`

## Version Contracts

### v0.0.1
- participating_services: `agent-delivery-bus`
- branch_guard_mode: `strict`
- validation_state: `active`
- per_service_canonical_branch:
  - `agent-delivery-bus`: `main`
- truth_canonical: `main`
- branch_governance_template: `standard-feature`
- worktree_mode: `dedicated`
- worktree_path: `.worktrees`
