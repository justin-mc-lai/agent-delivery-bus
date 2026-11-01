---
change_id: chg_67d79aa8af42
slug: requirement-index
status: pending
created_by: cli
created_at: 2026-11-01T14:17:19.807062Z
base_truth_sha256: 7c3c09c9a79ce55c67d79ed4ac517cbfd25e0733019ec8e2458e0039cbed1892
base_package_fingerprint: 930e2a700830003e9ea3b8ff9088857d6622f1b34e2a896b680855856e4821c4
---

# Change Request: M0.5: truth_stage 必须来自内核生命周期（beacon-go truth list --json），不得由 frontmatter 猜测；无记录时标 unknown 并计入 missing

M0.5: truth_stage 必须来自内核生命周期（beacon-go truth list --json），不得由 frontmatter 猜测；无记录时标 unknown 并计入 missing

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `beacon truth refreeze`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.
