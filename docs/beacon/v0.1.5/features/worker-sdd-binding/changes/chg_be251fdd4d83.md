---
change_id: chg_be251fdd4d83
slug: worker-sdd-binding
status: pending
created_by: opencode
created_at: 2026-11-03T17:13:17.821788Z
base_truth_sha256: 1e31cf8d9b9ff009e0a6310b1a6785d2b201889540c767cb1c0c932abade6109
base_package_fingerprint: 7262dada40312e2350eae9adb929db4b8744873b9c3fa5757d2c9d922b506ae1
---

# Change Request: TC 命令改为从仓库根可直接执行（autotest 不切 cwd、不执行 shell）

TC 命令改为从仓库根可直接执行（autotest 不切 cwd、不执行 shell）

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `beacon truth refreeze`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.
