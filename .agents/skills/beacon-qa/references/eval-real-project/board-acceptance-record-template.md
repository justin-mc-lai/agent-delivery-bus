# Beacon Real Project Board Acceptance Record

- feature: `FEATURE_SLUG`
- requested_version: `VERSION_OR_AUTO`
- validation_mode: `read-only multi-project board acceptance`
- validation_scope: `specialized-board-acceptance-only`
- not_a_substitute_for: `current-version real-project release proof`
- executed_at: `YYYY-MM-DDTHH:MM:SSZ`
- executor: `NAME`
- output_dir: `OUTPUT_DIR`
- canonical_homepage_artifact: `portfolio-board.txt`

## 0. Boundary

- This record is only for specialized board / projection acceptance.
- It does not prove that the latest Beacon line is fully validated on a single real project.
- Companion current-version release-proof evidence should be recorded separately when latest-version rollout confidence is required.

## 1. Validation Scope

### Projects

1. `rolo` -> `<company-workspace>/rolo`
2. `slow_uni_bmtop` -> `<workspace>/slow_uni_bmtop`
3. `shopxo_canada` -> `<workspace>/shopxo_canada`
4. `beacon` -> `<workspace>/beacon`

### Truth / Projection Boundary

- truth plane: `docs/beacon + .beacon`
- projection plane: `SQLite + PocketBase`
- out of scope: `omc / omx / Beacon 外运行时`

## 2. Executed Commands

List the capture script path and any extra manual commands used during validation.

```bash
bash skills/beacon/beacon-eval-real-project/scripts/run_real_project_board_validation.sh \
  --version VERSION_OR_AUTO \
  --output-dir OUTPUT_DIR
```

Additional manual commands:

```bash
# add any follow-up read-only commands here
```

If current-version release proof was also run, link it separately instead of merging the records.

## 3. Homepage Summary Result

### Verdict

- [ ] pass
- [ ] blocked by project state
- [ ] blocked by Beacon defect
- [ ] not yet implemented

### Notes

- Did all four projects appear on the board?
- Did homepage stay lifecycle-first and low-noise?
- Could CEO distinguish the four projects in one scan?
- Use `portfolio-board.txt` / `portfolio-board.json` as the canonical homepage evidence, not the per-project detail boards.
- Do not use this section alone to claim `prototype` / `archive` / `gate check release` coverage for the latest Beacon line.

## 4. Project Detail Result

### rolo

- verdict:
- project_kind:
- lifecycle:
- debug_state:
- explanation_quality:
- notes:

### slow_uni_bmtop

- verdict:
- project_kind:
- lifecycle:
- debug_state:
- explanation_quality:
- notes:

### shopxo_canada

- verdict:
- project_kind:
- lifecycle:
- debug_state:
- explanation_quality:
- notes:

### beacon

- verdict:
- project_kind:
- lifecycle:
- debug_state:
- explanation_quality:
- notes:

## 5. Deterministic Projection Result

### Verdict

- [ ] pass
- [ ] blocked by project state
- [ ] blocked by Beacon defect
- [ ] not yet implemented

### Notes

- Was the same truth snapshot projected consistently?
- Were `source_hash / projection_seq` stable?
- Did preflight reject or mark conflicts instead of silently guessing?
- Use each project folder `projection-determinism.json` and `status.json` / `status-repeat.json`.

## 6. Debug Tri-State Result

Expected states:

- `rolo` -> `runtime_feed_only`
- `slow_uni_bmtop` -> `materialized_issue`
- `shopxo_canada` -> `no_debug_feed`

Observed states:

- `rolo`:
- `slow_uni_bmtop`:
- `shopxo_canada`:

Verdict:

- [ ] pass
- [ ] blocked by project state
- [ ] blocked by Beacon defect
- [ ] not yet implemented

## 7. Degradation / Rebuild Result

### Verdict

- [ ] pass
- [ ] blocked by project state
- [ ] blocked by Beacon defect
- [ ] not yet implemented

### Notes

- Could projects still be understood when SQLite / PocketBase was unavailable?
- Could minimum board state be rebuilt from local truth?
- Minimum evidence should include `status.json` truth snapshots plus the isolated ledger outputs in the capture pack.

## 8. Findings Classification

### Beacon Defects

1.
2.
3.

### Project-State Blockers

1.
2.
3.

### Not-Yet-Implemented Acceptance Gaps

1.
2.
3.

## 9. Final Recommendation

- [ ] safe to use on these projects
- [ ] safe for read-only use only
- [ ] blocked until Beacon defect is fixed
- [ ] blocked until feature implementation reaches acceptance line

## 10. Linked Artifacts

- capture directory:
- companion current-version release-proof record:
- doctor outputs:
- help/status/release outputs:
- board exports:
- deterministic projection reports:
- follow-up issue / PR links:
