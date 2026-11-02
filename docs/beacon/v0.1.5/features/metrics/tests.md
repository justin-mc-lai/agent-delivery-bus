---
slug: metrics
version: v0.1.5
materials_status: current
---

# Tests: metrics

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-MT-001 | behavior | `go test ./... -run MetricsReadsExistingStreams` | exit_code_0；四个指标键齐全；--since 过滤生效 |
| AC-002 | TC-MT-002 | behavior | `go test ./... -run MetricsTimeToFirstWorker` | exit_code_0；p50/p95 由 recorded_at 与 decided_at 差值算出；样本<5 时 p95 缺省且标 insufficient |
| AC-003 | TC-MT-003 | behavior | `go test ./... -run MetricsTaskCompletion` | exit_code_0；无回执 count=0 且无分位数字 |
| AC-004 | TC-MT-004 | behavior | `go test ./... -run MetricsPerThreadAndFastShare` | exit_code_0；decisions_per_thread 与 fastpath_share 与手算一致 |
| AC-005 | TC-MT-005 | behavior | `go test ./... -run MetricsEmptyIsExplicit` | exit_code_0；文件缺失/空时 status=insufficient_data、exit 0、无编造数字 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-MT-EXEC-001 | behavior | `go test ./...` in cmd/adbd | exit_code_0（含此前 76 个用例） |
| AC-001 | TC-MT-EXEC-002 | behavior | `python3 -m pytest -q tests/ --tb=short` | exit_code_0 |
