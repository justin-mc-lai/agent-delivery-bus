# Release Checklist: v0.1.4

**Version**: v0.1.4
**Created**: 2026-08-17
**Status**: QA passed — awaiting human release gate

## Pre-Release Checks

- [x] All tests passing（232 passed）
- [x] 版本校验：`python3 scripts/verify-version-alignment.py`（--check-tag 随 release 通过）
- [x] No critical bugs open
- [x] Documentation updated

## Build

- [x] Build successful（`python3 -m pip install -e '.[test]'`）

## Deployment

- [x] Smoke tests passed（backup + legacy migration + capabilities + CI YAML）

## Release

- [ ] Production deployment (main merged, release candidate)
- [ ] git tag `v0.1.4` 与 catalog 对齐（打标即对齐）

## Change Documents

- [x] Version change document generated
- [x] Release reviewer confirmed included feature packages match intended scope
