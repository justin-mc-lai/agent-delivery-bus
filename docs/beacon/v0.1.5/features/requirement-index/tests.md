---
slug: requirement-index
version: v0.1.5
materials_status: draft
---

# Tests: requirement-index

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-RI-001 | AC-001 | `python3 -m pytest -q tests/test_requirement_index.py -k schema --tb=short` | exit_code_0；每条记录 11 个字段齐全；`json.load` 成功；顶层含 `schema_version` / `generated_from` |
| TC-RI-002 | AC-002 | `python3 -m pytest -q tests/test_requirement_index.py -k idempotent --tb=short` | exit_code_0；连续生成两次的字节串相等；记录数不增长 |
| TC-RI-003 | AC-003 | `python3 -m pytest -q tests/test_requirement_index.py -k missing --tb=short` | exit_code_0；坏来源 → `source_status=unknown` + `missing[]` 有条目；记录未被删除；无默认值填充 |
| TC-RI-004 | AC-004 | `python3 -m pytest -q tests/test_requirement_index.py -k selfcontained --tb=short` | exit_code_0；把单文件复制到临时目录后 `json.load` 仍成功且字段完整 |
| TC-RI-005 | AC-005 | `python3 -m pytest -q tests/test_requirement_index.py -k write_authority --tb=short` | exit_code_0；写 `exec_state` 成功落库；写 `truth_stage` 返回 `truth_stage_readonly` 且文件 mtime/内容不变 |
| TC-RI-006 | AC-006 | `python3 -m pytest -q tests/test_requirement_index.py -k layering --tb=short` | exit_code_0；>10240B → 产生 `requirements/<project>.json` 分片；summary 保留 4 个关键字段；未超阈不产分片 |

## Illegal TC slots

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-RI-ILL-001 | AC-005 | `python3 -m pytest -q tests/test_requirement_index.py -k illegal_truth_stage --tb=short` | exit_code_0；直接调写入接口改 `truth_stage` → `truth_stage_readonly`，索引不变 |
| TC-RI-ILL-002 | AC-003 | `python3 -m pytest -q tests/test_requirement_index.py -k illegal_source --tb=short` | exit_code_0；来源 JSON 语法错误 → 记 `missing[]` 而非抛栈退出 |

## Execution slots

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-RI-EXEC-001 | AC-001..006 | `python3 -m pytest -q tests/test_requirement_index.py --tb=short` | exit_code_0 |
| TC-RI-EXEC-002 | AC-001..006 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0（全量回归，既有 211+） |
