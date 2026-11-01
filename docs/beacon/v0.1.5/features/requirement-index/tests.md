---
slug: requirement-index
version: v0.1.5
materials_status: current
---

# Tests: requirement-index

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-RI-001 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k schema --tb=short` | exit_code_0；每条记录 11 个字段齐全；`json.load` 成功；顶层含 `schema_version` 与 `generated_from` |
| AC-002 | TC-RI-002 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k idempotent --tb=short` | exit_code_0；连续生成两次的字节串相等；记录数不增长 |
| AC-003 | TC-RI-003 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k missing --tb=short` | exit_code_0；坏来源写入 `missing[]` 且记录 `source_status=unknown`；记录未被删除；无默认值填充 |
| AC-004 | TC-RI-004 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k selfcontained --tb=short` | exit_code_0；单文件复制到临时目录后 `json.load` 仍成功且字段完整 |
| AC-005 | TC-RI-005 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k write_authority --tb=short` | exit_code_0；写 `exec_state` 落库成功；写 `truth_stage` 返回 `truth_stage_readonly` 且文件内容哈希不变 |
| AC-006 | TC-RI-006 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k layering --tb=short` | exit_code_0；超过 10240 字节产生分片；summary 保留 req_id/truth_stage/exec_state/truth_ref；未超阈值不产分片 |
| AC-007 | TC-RI-007 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k enumerator --tb=short` | exit_code_0；两个假项目树中只收 frozen 的包；未冻结/无 frontmatter 进 `skipped[]`；重复 root 只枚举一次；req_id 形态为 `req:<project>:<feature>` |
| AC-009 | TC-RI-009 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k lifecycle_source --tb=short` | exit_code_0；注入 fake provider 时 `truth_stage` 取 provider 值；provider 无该 slug 时 `unknown` + `missing[]` reason=`lifecycle_source_missing`；frontmatter 的 `status: frozen` 不改变 `truth_stage` |
| AC-008 | TC-RI-008 | integration | `python3 -m pytest -q tests/test_requirement_index.py -k cli --tb=short` | exit_code_0；`adb index generate` 写文件并 exit 0；`adb index status` 输出条数；文件缺失 exit 2 且 reason_code=index_file_missing；固定 `ADB_REQUIREMENT_INDEX_NOW` 后两次生成字节一致 |

## Illegal TC slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-005 | TC-RI-ILL-001 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k illegal_truth_stage --tb=short` | exit_code_0；直接调写入接口改 `truth_stage` 得到 `truth_stage_readonly`，索引内容不变 |
| AC-003 | TC-RI-ILL-002 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k illegal_source --tb=short` | exit_code_0；来源 JSON 语法错误写入 `missing[]`，进程正常退出 |
| AC-009 | TC-RI-ILL-004 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k lifecycle_not_guessed --tb=short` | exit_code_0；只改 frontmatter `status` 不会让 `truth_stage` 从 `unknown` 变成任何阶段 |
| AC-007 | TC-RI-ILL-003 | unit | `python3 -m pytest -q tests/test_requirement_index.py -k enumerator_missing_root --tb=short` | exit_code_0；传入不存在的 project root → 记入 `skipped[]`，不抛栈 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-RI-EXEC-001 | integration | `python3 -m pytest -q tests/test_requirement_index.py --tb=short` | exit_code_0 |
| AC-001 | TC-RI-EXEC-002 | integration | `python3 -m pytest -q tests/ --tb=short` | exit_code_0（全量回归） |
