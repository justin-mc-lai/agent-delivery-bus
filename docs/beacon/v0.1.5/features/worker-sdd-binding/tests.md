---
slug: worker-sdd-binding
version: v0.1.5
materials_status: current
---

# Tests: worker-sdd-binding

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-SB-001 | behavior | go -C cmd/adbd test -run TestSddDetectsBeaconProject | exit_code_0；有 features 目录判为 beacon；无则 not_applicable |
| AC-002 | TC-SB-002 | behavior | go -C cmd/adbd test -run TestSddRefusesUnfrozenTruth | exit_code_0；truth 非 frozen 得 truth_not_frozen，零 paseo 调用，条目留队列 |
| AC-003 | TC-SB-003 | behavior | go -C cmd/adbd test -run TestSddRefusesUnknownFeature | exit_code_0；truth.md 不存在得 feature_not_registered，零 paseo 调用 |
| AC-004 | TC-SB-004 | behavior | go -C cmd/adbd test -run TestSddRefusalKeepsState | exit_code_0；拒绝后绑定未变、队列条数不变、幂等去重生效 |
| AC-005 | TC-SB-005 | behavior | go -C cmd/adbd test -run TestSddGuardInDispatchedText | exit_code_0；派活文本与 bootstrap 都含 spine show 与没有冻结 AC 两句 |
| AC-006 | TC-SB-006 | behavior | go -C cmd/adbd test -run TestSddNonBeaconUnaffected | exit_code_0；非 beacon 项目照常派活，trace 记 not_applicable |
| AC-007 | TC-SB-007 | behavior | go -C cmd/adbd test -run TestSddConfigAndTraces | exit_code_0；gate=off 不拒绝且记 gated_off；--json 能看到 sdd 与配置 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-SB-EXEC-001 | behavior | go -C cmd/adbd test ./... | exit_code_0（含此前 126 个用例） |
| AC-001 | TC-SB-EXEC-002 | behavior | python3 -m pytest -q tests/ | exit_code_0 |
