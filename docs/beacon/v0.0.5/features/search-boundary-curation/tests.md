---
slug: search-boundary-curation
version: v0.0.5
materials_status: current
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

| AC ID | TC ID | Path type | Assertion |
|-------|-------|-----------|-----------|
| AC-SBC-001 | TC-SBC-001 | legal walk idle→awaiting_review | ingest with profiles+VerticalGate allow lands awaiting_review |
| AC-SBC-003 | TC-SBC-003 | legal walk awaiting_review→approved→active | decide approve with actor activates boundary |
| AC-SBC-003 | TC-SBC-003 | legal walk awaiting_review→rejected | decide reject with actor records rejection |
| AC-SBC-007 | TC-SBC-ILL-001 | illegal idle→active without awaiting_review | skip awaiting_review fail-closed |
| AC-SBC-007 | TC-SBC-007 | illegal awaiting_review→active without decide | no auto-approve / no_auto fail-closed |
| AC-SBC-010 | TC-SBC-ILL-002 | illegal validating→awaiting_review on vertical_gate_reject | complete drift blocked before awaiting_review |
| AC-SBC-008 | TC-SBC-008 | illegal validating→awaiting_review on missing_profile_ref | missing profile refs blocked |
| AC-SBC-011 | TC-SBC-011 | illegal idle→scheduled_tick with sticker_emoji_bank | daily batch stays on oss-picks vertical |
