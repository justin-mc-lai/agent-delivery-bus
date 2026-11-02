---
slug: projects-import
version: v0.1.5
materials_status: draft
---

# Tests: projects-import

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-PI-001 | behavior | `python3 -m pytest -q tests/test_projects_import.py -k scan` | exit_code_0；只收有 features 的目录；found 计数正确 |
| AC-002 | TC-PI-002 | behavior | `python3 -m pytest -q tests/test_projects_import.py -k dry_run` | exit_code_0；dry-run 不修改文件 |
| AC-003 | TC-PI-003 | behavior | `python3 -m pytest -q tests/test_projects_import.py -k apply` | exit_code_0；写入后原字段仍在 |
| AC-004 | TC-PI-004 | behavior | `python3 -m pytest -q tests/test_projects_import.py -k existing` | exit_code_0；已存在跳过；--overwrite 才更新 |
| AC-005 | TC-PI-005 | behavior | `python3 -m pytest -q tests/test_projects_import.py -k kernel` | exit_code_0；内核失败不阻断 JSON，kernel_failed 计数正确 |
| AC-006 | TC-PI-006 | behavior | `python3 -m pytest -q tests/test_projects_import.py -k errors` | exit_code_0；三种 reason_code 正确 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-PI-EXEC-001 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0（含既有 255） |
| AC-001 | TC-PI-EXEC-002 | behavior | `cd cmd/adbd && go test ./...` | exit_code_0（Go 侧零回归） |
