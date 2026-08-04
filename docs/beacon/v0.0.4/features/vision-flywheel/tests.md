---
slug: vision-flywheel
version: v0.0.4
materials_status: stale
---

# Tests: vision-flywheel

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-FLY-001 | AC-FLY-001 | `python3 -m pytest -q tests/test_schedule.py -k register --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；register 成功写入调度注册表；重复 slug 幂等更新；未知引擎拒绝 |
| TC-FLY-002 | AC-FLY-002 | `python3 -m pytest -q tests/test_schedule.py -k list_show --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；list 含全部条目与 quota 状态；show 返回单条目 |
| TC-FLY-003 | AC-FLY-003 | `python3 -m pytest -q tests/test_schedule.py -k should_run --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；quota 充足→run；耗尽/不健康→blocked+reason_code；无 LLM 调用 |
| TC-FLY-004 | AC-FLY-004 | `python3 -m pytest -q tests/test_schedule.py -k quota --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；slot 按白名单来源记账；耗尽→throttled；无证据不计配额 |
| TC-FLY-005 | AC-FLY-005 | `python3 -m pytest -q tests/test_schedule.py -k ledger --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；心跳运行追加写事件流（entry_slug/status/evidence_refs/quota_spent） |
| TC-FLY-006 | AC-FLY-006 | `python3 -m pytest -q tests/test_schedule.py -k reconcile --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；缺证据保持 reconciling；证据齐→completed |
| TC-FLY-007 | AC-FLY-007 | `python3 -m pytest -q tests/test_schedule.py -k no_auto --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；心跳不自动 approve/dispatch |
| TC-FLY-ILL-001 | AC-FLY-007 | `python3 -m pytest -q tests/test_schedule.py -k illegal_dispatch --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；心跳→dispatch/approve 非法 |
| TC-FLY-ILL-002 | AC-FLY-006 | `python3 -m pytest -q tests/test_schedule.py -k skip_should_run --junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"` | exit_code==0；跳过 should-run 直达执行非法 |

## Domain FSM QA Matrix

| Walk ID | AC ID | Path type | Assertion |
|---------|-------|-----------|-----------|
| W-FLY-01 | AC-FLY-003 | legal walk idle→checking→running | covered by TC-FLY-003 should-run pass |
| W-FLY-02 | AC-FLY-005 | legal walk running→done | covered by TC-FLY-005 ledger heartbeat |
| W-FLY-03 | AC-FLY-004 | legal walk checking→blocked on quota | covered by TC-FLY-004 quota throttle |
| I-FLY-01 | AC-FLY-007 | illegal checking→dispatch / running→approve | covered by TC-FLY-ILL-001 |
| I-FLY-02 | AC-FLY-006 | illegal idle→running skip should-run | covered by TC-FLY-ILL-002 |

