# Release Checklist: v0.1.4

**Version**: v0.1.4
**Created**: 2026-08-17
**Status**: Pending (human gate required)

## Pre-Release Checks

- [ ] All tests passing（211 + 新增）
- [ ] 版本校验：`python3 scripts/verify-version-alignment.py --check-tag`
- [ ] No critical bugs open
- [ ] Documentation updated

## Build

- [ ] Build successful（`python3 -m pip install -e '.[test]'`）

## Deployment

- [ ] Smoke tests passed（backup + migration + capabilities）

## Release

- [ ] Production deployment (main merged, release candidate)
- [ ] git tag `v0.1.4` 与 catalog 对齐

## Change Documents

- [ ] Version change document generated
- [ ] Release reviewer confirmed included feature packages match intended scope
