---
change_id: chg_3fdbf2926e49
slug: worker-crud
status: pending
created_by: cli
created_at: 2026-11-02T14:20:33.19989Z
base_truth_sha256: 415e992e5646da949008786e4188184d8c9e7f982e71fa3d4e513f7409c61e86
base_package_fingerprint: 962da5b2017bb8027679cb8e528d4d83b631714f143c685f1c8dd9e83784863d
---

# Change Request: AC-004/005 补 foreign 保护：带 adbd.foreign 标签的 worker 必须 --force 才能改或归档；实测中规格漏写导致误改误归档真实 agent

AC-004/005 补 foreign 保护：带 adbd.foreign 标签的 worker 必须 --force 才能改或归档；实测中规格漏写导致误改误归档真实 agent

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `beacon truth refreeze`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.
