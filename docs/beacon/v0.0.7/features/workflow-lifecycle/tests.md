---
slug: workflow-lifecycle
version: v0.0.7
materials_status: current
---

# Tests: workflow-lifecycle

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-WF-001 | AC-WF-001 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k 'beacon_stages or force_load or missing_skill' --tb=short` | exit_code_0；六阶段 force-load；缺 skill fail-closed |
| TC-WF-002 | AC-WF-002 | `python3 -m pytest -q tests/test_keywords.py --tb=short` | exit_code_0；三渠道同 envelope；`adb intent keywords --json` 可输出 |
| TC-WF-003 | AC-WF-003 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k presets --tb=short` | exit_code_0；superpowers/openspec 可装可绑 |
| TC-WF-004 | AC-WF-004 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k 'ingest or host_fill or draft_confirm' --tb=short` | exit_code_0；ingest→request→回填→校验→draft→confirm→install |
| TC-WF-005 | AC-WF-005 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k 'trace or debug or replay' --tb=short` | exit_code_0；JSONL trace 事件齐全；trace/debug/replay 可用 |
| TC-WF-006 | AC-WF-006 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k verify --tb=short` | exit_code_0；verify 探针全过 |
| TC-WF-007 | AC-WF-007 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k 'dispatch_reconcile or bad_repo' --tb=short` | exit_code_0；闭环完成；坏库 fail-closed |
| TC-WF-008 | AC-WF-008 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k 'illegal or dangerous' --tb=short` | exit_code_0；无确认/危险命令/无证据/伪造 trace 全拒 |
| TC-EXEC-001 | AC-WF-001..008 | `python3 -c "import agent_delivery_bus, beacon; print('ok')"` | output_ok |
| TC-EXEC-002 | AC-WF-001..008 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |

## Domain FSM QA Matrix

| Walk ID | AC ID | Path type | Assertion |
|---------|-------|-----------|-----------|
| W-WF-01 | AC-WF-004 | legal requested→…→reconciled | covered by TC-WF-004/TC-WF-007 |
| W-WF-02 | AC-WF-001 | legal 六阶段派发 force-load | covered by TC-WF-001 |
| W-WF-03 | AC-WF-002 | legal 三渠道同 envelope | covered by TC-WF-002 |
| I-WF-01 | AC-WF-008 | illegal 无证据直接确认 | covered by TC-WF-ILL-001 |
| I-WF-02 | AC-WF-008 | illegal 未确认安装 | covered by TC-WF-ILL-002 |
| I-WF-03 | AC-WF-008 | illegal 危险命令安装 | covered by TC-WF-ILL-003 |
| I-WF-04 | AC-WF-008 | illegal 未 verify 直接派发 | covered by TC-WF-ILL-004 |
| I-WF-05 | AC-WF-008 | illegal 伪造/缺失 trace | covered by TC-WF-ILL-005 |

## Illegal TC slots

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-WF-ILL-001 | AC-WF-008 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k 'no_evidence_confirm' --tb=short` | exit_code_0；无证据字段拒绝确认 |
| TC-WF-ILL-002 | AC-WF-008 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k 'install_without_confirm' --tb=short` | exit_code_0；未确认不安装 |
| TC-WF-ILL-003 | AC-WF-008 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k dangerous_command --tb=short` | exit_code_0；危险命令拒绝 |
| TC-WF-ILL-004 | AC-WF-008 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k 'dispatch_without_verify' --tb=short` | exit_code_0；未 verify 不可正式派发 |
| TC-WF-ILL-005 | AC-WF-008 | `python3 -m pytest -q tests/test_workflow_lifecycle.py -k 'fake_trace' --tb=short` | exit_code_0；伪造 trace 拒绝 |
