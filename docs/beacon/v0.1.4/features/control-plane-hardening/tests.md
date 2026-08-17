---
slug: control-plane-hardening
version: v0.1.4
materials_status: current
---

# Tests: control-plane-hardening

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-CPH-001 | AC-CPH-001 | `python3 -m pytest -q tests/test_version_alignment.py --tb=short` | exit_code_0；catalog 解析 + 四表面比对通过 |
| TC-CPH-002 | AC-CPH-002 | `python3 -m pytest -q tests/test_version_alignment.py -k 'tag' --tb=short` | exit_code_0；--check-tag 有/无 tag 均按契约 |
| TC-CPH-003 | AC-CPH-003 | `python3 -m pytest -q tests/test_storage_migration.py -k 'fresh' --tb=short` | exit_code_0；user_version==SCHEMA_VERSION；审计行存在 |
| TC-CPH-004 | AC-CPH-004 | `python3 -m pytest -q tests/test_storage_migration.py -k 'legacy' --tb=short` | exit_code_0；旧列迁移 + 结构一致 + 幂等 |
| TC-CPH-005 | AC-CPH-005 | `python3 -m pytest -q tests/test_backup.py --tb=short` | exit_code_0；manifest + 账本可打开 + 缺失源报错 |
| TC-CPH-006 | AC-CPH-006 | `python3 -m pytest -q tests/test_adapter_capabilities.py --tb=short` | exit_code_0；按能力传参 + resolver 签名协商 |
| TC-CPH-007 | AC-CPH-007 | `python3 -c "import yaml, pathlib; p=pathlib.Path('.github/workflows/ci.yml'); assert 'release' not in p.read_text().lower()"` | output_ok；CI 无 release 步骤 |
| TC-CPH-008 | AC-CPH-008 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0；全量回归（211 + 新增） |
| TC-EXEC-001 | AC-CPH-001..008 | `python3 -c "import agent_delivery_bus.version_truth, agent_delivery_bus.storage; print('ok')"` | output_ok |
| TC-EXEC-002 | AC-CPH-001..008 | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |

## Domain FSM QA Matrix

| Walk ID | AC ID | Path type | Assertion |
|---------|-------|-----------|-----------|
| W-CPH-01 | AC-CPH-003 | legal fresh → up_to_date | covered by TC-CPH-003 |
| W-CPH-02 | AC-CPH-004 | legal legacy → migrated → 幂等重跑 | covered by TC-CPH-004 |
| W-CPH-03 | AC-CPH-001 | legal catalog → 四表面一致 | covered by TC-CPH-001 |
| I-CPH-01 | AC-CPH-001 | illegal 跳过 catalog 手改表面 | covered by TC-CPH-ILL-001 |
| I-CPH-02 | AC-CPH-003 | illegal 跳版本迁移 | covered by TC-CPH-ILL-002 |
| I-CPH-03 | AC-CPH-006 | illegal 能力未声明被注入 | covered by TC-CPH-ILL-003 |
| I-CPH-04 | AC-CPH-007 | illegal CI 含 release | covered by TC-CPH-ILL-004 |

## Illegal TC slots

| TC ID | AC ID | Command | Assertion |
|-------|-------|---------|-----------|
| TC-CPH-ILL-001 | AC-CPH-001 | `python3 -m pytest -q tests/test_version_alignment.py -k 'drift' --tb=short` | exit_code_0；手改表面 → 校验失败列出差异 |
| TC-CPH-ILL-002 | AC-CPH-003 | `python3 -m pytest -q tests/test_storage_migration.py -k 'gap' --tb=short` | exit_code_0；跳版本迁移 blocked |
| TC-CPH-ILL-003 | AC-CPH-006 | `python3 -m pytest -q tests/test_adapter_capabilities.py -k 'missing_cap' --tb=short` | exit_code_0；缺能力 → fail-closed 或跳过不注入 |
| TC-CPH-ILL-004 | AC-CPH-007 | `python3 scripts/verify-version-alignment.py` + CI 文件检查 | ci.yml 无 release/publish/deploy |
