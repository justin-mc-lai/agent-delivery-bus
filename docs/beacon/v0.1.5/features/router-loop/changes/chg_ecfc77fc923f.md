---
change_id: chg_ecfc77fc923f
slug: router-loop
status: pending
created_by: cli
created_at: 2026-11-03T14:24:47.659966Z
base_truth_sha256: b978f0ac00930907f9f4f756465aac13aaaccf0e03b935f0a8cd37bef39f241b
base_package_fingerprint: fcd91da28248bfab3a45acce4006cd1b0b8d1c835091853b708c643178aca618
---

# Change Request: AC-007 按来源分级信任：github/skill 为可信来源（高危词不强制升级、派活不带 UNTRUSTED_INPUT 前缀），feishu 等保持不可信；新增 AC-010 信任配置与 AC-011 escalate 进 inbox

AC-007 按来源分级信任：github/skill 为可信来源（高危词不强制升级、派活不带 UNTRUSTED_INPUT 前缀），feishu 等保持不可信；新增 AC-010 信任配置与 AC-011 escalate 进 inbox

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `beacon truth refreeze`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.
