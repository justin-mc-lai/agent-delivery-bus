---
slug: search-boundary-curation
version: v0.0.5
materials_status: stale
---

# Tests: search-boundary-curation

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-SBC-001 | AC-SBC-001 | `python3 -m pytest -q tests/test_boundary.py -k ingest_lands --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；ingest→awaiting_review；缺 topic 拒绝 |
| TC-SBC-002 | AC-SBC-002 | `python3 -m pytest -q tests/test_boundary.py -k list_awaiting_show --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；待审列表/show 正确 |
| TC-SBC-003 | AC-SBC-003 | `python3 -m pytest -q tests/test_boundary.py -k decide_approve --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；approve/reject + already_decided |
| TC-SBC-004 | AC-SBC-004 | `python3 -m pytest -q tests/test_boundary.py -k list_status --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；默认 active；可按 status 过滤 |
| TC-SBC-005 | AC-SBC-005 | `python3 -m pytest -q tests/test_boundary.py -k awaiting_view --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；kind 绑定 boundary 待审 |
| TC-SBC-006 | AC-SBC-006 | `python3 -m pytest -q tests/test_boundary.py -k schedule_tick --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；每日 5 条垂直选题 ingest-only；无 decide 执行 |
| TC-SBC-007 | AC-SBC-007 | `python3 -m pytest -q tests/test_boundary.py -k 'no_auto or illegal or skip_awaiting' --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；非法路径 fail-closed |
| TC-SBC-008 | AC-SBC-008 | `python3 -m pytest -q tests/test_boundary.py -k profile_refs_required --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；缺 project/account 画像 ref 拒绝 ingest |
| TC-SBC-009 | AC-SBC-009 | `python3 -m pytest -q tests/test_boundary.py -k vertical_profiles_auditable --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；Project/Account 画像可加载且库拾=oss-picks |
| TC-SBC-010 | AC-SBC-010 | `python3 -m pytest -q tests/test_boundary.py -k vertical_gate --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；表情包/情感漫等完整偏离被 VerticalGate 拦下 |
| TC-SBC-011 | AC-SBC-011 | `python3 -m pytest -q tests/test_boundary.py -k kushi_topics_in_vertical --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；日批 5 条均落在 GitHub 开源 AI / AI Spec；无表情包词 |
| TC-SBC-ILL-001 | AC-SBC-007 | `python3 -m pytest -q tests/test_boundary.py -k 'illegal or skip_awaiting' --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；ingest→active / 跳过待审 均 fail-closed |
| TC-SBC-ILL-002 | AC-SBC-010 | `python3 -m pytest -q tests/test_boundary.py -k vertical_gate --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit-search-boundary-curation.xml}"` | exit_code==0；离题入待审 fail-closed |

## Domain FSM QA Matrix

| Walk ID | AC ID | Path type | Assertion |
|---------|-------|-----------|-----------|
| W-SBC-01 | AC-SBC-001 | legal walk idle→awaiting_review | covered by TC-SBC-001 ingest with profiles+VerticalGate allow |
| W-SBC-02 | AC-SBC-003 | legal walk awaiting_review→approved→active | covered by TC-SBC-003 decide approve with actor |
| W-SBC-03 | AC-SBC-003 | legal walk awaiting_review→rejected | covered by TC-SBC-003 decide reject with actor |
| I-SBC-01 | AC-SBC-007 | illegal idle→active without awaiting_review | covered by TC-SBC-ILL-001 |
| I-SBC-02 | AC-SBC-007 | illegal awaiting_review→active without decide | covered by TC-SBC-007 no_auto |
| I-SBC-03 | AC-SBC-010 | illegal validating→awaiting_review on vertical_gate_reject | covered by TC-SBC-ILL-002 |
| I-SBC-04 | AC-SBC-008 | illegal validating→awaiting_review on missing_profile_ref | covered by TC-SBC-008 |
| I-SBC-05 | AC-SBC-011 | illegal idle→scheduled_tick with sticker_emoji_bank | covered by TC-SBC-011 |
