# Real Project Command Matrix

Use these placeholders:

- `ROOT` = real project root
- `VERSION` = docs line such as `v1.0.0`
- `FEATURE` = one existing feature such as `friend-match`

Default validation goal:

- current-version real-project release proof

Specialized validation goal:

- multi-project board acceptance

Do not use the specialized board path as a substitute for the default current-version release proof.

## Version Overlay Sources

Before treating the matrix as complete, inspect the current version sources:

- `ROOT/AGENTS.md`
- `ROOT/docs/beacon/VERSION/SUMMARY.md`
- `ROOT/docs/beacon/VERSION/execution/index.md`
- `ROOT/docs/beacon/VERSION/release/VERSION.md`
- `skills/beacon/SKILL.md`

Use those files to derive which shipped features must be proven on the real project for the requested Beacon line.

## Preflight

```bash
git -C ROOT status --short
beacon doctor --json --project-root ROOT
beacon help --project-root ROOT --version VERSION --feature FEATURE --json
```

## Preferred Shortcut

```bash
beacon real-project-validation --project-root ROOT --version VERSION --feature FEATURE --json
```

The shortcut must still cover current-version public release proof. It cannot replace the underlying command matrix by silently skipping `prototype`, `archive`, `gate`, acceptance transaction, prompt-boundary, or latest-version acceptance overlays.

## Modern Runtime Prechecks

```bash
beacon doctor verify-subagent-runtime --project-root ROOT --host-runtime codex --json
beacon doctor verify-subagent-runtime --project-root ROOT --host-runtime claude --json
beacon doctor verify-subagent-delegation FEATURE --project-root ROOT --version VERSION --json
```

## Read-Only Minimal Surface

```bash
beacon status show-status --project-root ROOT --version VERSION --feature FEATURE --board
beacon prd list -p ROOT -v VERSION
beacon user-story list -p ROOT -v VERSION
beacon test-case list -p ROOT -v VERSION
beacon test-case gate FEATURE -p ROOT -v VERSION
beacon prototype list -p ROOT -v VERSION
beacon prototype status FEATURE -p ROOT -v VERSION --json
beacon implement status FEATURE -p ROOT -v VERSION --json
beacon qa status -p ROOT --json
beacon qa evolve FEATURE -p ROOT -v VERSION --no-write-matrix --json
beacon release check VERSION --project-root ROOT --json
beacon release scorecard VERSION -p ROOT --json
beacon archive status -p ROOT -v VERSION --feature FEATURE --json
beacon archive explain -p ROOT -v VERSION --json
beacon archive plan -p ROOT -v VERSION --feature FEATURE --json
beacon gate check prototype --project-root ROOT --version VERSION --json
beacon gate check release --project-root ROOT --version VERSION --json
```

For current Beacon lines, this baseline is the minimum public-surface proof. Missing `prototype`, `archive`, or `gate check release` means the latest human surface is not fully covered.

## Current-Version Feature Overlay

For Beacon `v1.3.8`, the real-project proof should explicitly classify whether the public surface proves:

- diagram truth admission
- post-freeze host prompt trace
- prompt-boundary gate / release convergence
- archive governance support command behavior
- canonical delivery baseline convergence across implement / qa / release

If a capability is not applicable to the target feature, classify it as `not_applicable` rather than silently skipping it.

## Team / Ralph Dry-Run Surface

Run these only when the Beacon version under validation already supports `team` / `ralph` on the public `implement` surface:

```bash
beacon implement run FEATURE --project ROOT --version VERSION --mode team --runner codex-subagents --host-runtime codex --dry-run --json
beacon implement run FEATURE --project ROOT --version VERSION --mode team --runner gstack --host-runtime claude --dry-run --json
beacon implement run FEATURE --project ROOT --version VERSION --mode ralph --runner codex-subagents --host-runtime codex --dry-run --json
beacon implement run FEATURE --project ROOT --version VERSION --mode ralph --runner gstack --host-runtime claude --dry-run --json
beacon implement ralph-status FEATURE --project-root ROOT --json
```

## Browser QA / Session Surface

Use these when validating Beacon versions that include formal browser QA and session/profile support:

```bash
beacon qa session list -p ROOT --json
beacon qa status -p ROOT --json
beacon qa run FEATURE -p ROOT -v VERSION --json
beacon qa team-run FEATURE -p ROOT -v VERSION --host-runtime codex --json
beacon qa team-run FEATURE -p ROOT -v VERSION --host-runtime claude --json
```

Interpret carefully:

- If the feature's formal QA bundle only declares `unit`, then no browser execution should be expected.
- If the formal QA bundle declares browser-capable layers, verify that Beacon produces browser-task / browser evidence artifacts rather than silently falling back to unrelated default layers.

## How To Interpret Results

- Count the surface as operational when it returns structured output or a clear human-readable status.
- Do not count `qa blocked`, `release_ready=false`, or failed release checks as Beacon defects by themselves.
- Do not count version-line package closure gaps as Beacon defects when current-feature closure is already ready.
- Do not count structured `prototype`, `archive`, or `gate` warnings as Beacon defects by themselves.
- Count the surface as defective when it crashes, routes to the wrong version, selects the wrong run, or mutates tracked docs during read-only validation.
- Count the surface as defective when current-version published features cannot be proven on the public surface, even though release docs claim they shipped.
- Count it as a classification error when a specialized board-acceptance pack is used as if it were full current-version release proof.

## Write-Path Validation In Isolation Only

Run these only in an isolated copy or worktree:

```bash
beacon prd create FEATURE --project ROOT --version VERSION
beacon user-story create FEATURE --project ROOT --version VERSION
beacon test-case create FEATURE --project ROOT --version VERSION
beacon implement plan FEATURE --project ROOT --version VERSION --json
```

For modern Beacon lines, isolated write-path validation may also include:

```bash
beacon doctor setup-context --project-root ROOT
beacon freeze FEATURE --project-root ROOT --version VERSION
beacon doctor sync-technical-review-contract FEATURE --project-root ROOT --version VERSION --revision-id R1 --review-status approved --approved-by validator --reviewed-at 2026-01-01T00:00:00Z --json
beacon qa session save qa-local -p ROOT --origin http://127.0.0.1:3000 --runner-type agent-browser --scope http://127.0.0.1:3000 --json
beacon qa session use qa-local -p ROOT --json
beacon qa team-run FEATURE -p ROOT -v VERSION --host-runtime codex --json
```

## Multi-Project Board Acceptance Pack

Use this path only when validating a lifecycle-first board / projection feature across multiple real projects.

This pack is specialized acceptance only. It does not replace current-version release proof.

### Capture Script

```bash
bash skills/beacon/beacon-eval-real-project/scripts/run_real_project_board_validation.sh \
  --version v1.3.4 \
  --output-dir .beacon/validation/real-project-board/v1.3.4-sample
```

The capture pack now writes an isolated ledger under the output directory and emits a canonical multi-project homepage artifact:

- `portfolio-board.txt`
- `portfolio-board.json`
- `acceptance-record.md`

Default targets:

- `/Users/apple/Developer/Company/rolo`
- `/Users/apple/Developer/Personal/products/slow_uni_bmtop`
- `/Users/apple/Developer/Personal/products/shopxo_canada`
- `/Users/apple/Developer/Personal/products/beacon`

### Optional Explicit Targets

```bash
bash skills/beacon/beacon-eval-real-project/scripts/run_real_project_board_validation.sh \
  --target "rolo|/Users/apple/Developer/Company/rolo||auto" \
  --target "slow_uni_bmtop|/Users/apple/Developer/Personal/products/slow_uni_bmtop||auto" \
  --target "shopxo_canada|/Users/apple/Developer/Personal/products/shopxo_canada||auto" \
  --target "beacon|/Users/apple/Developer/Personal/products/beacon|beacon-v1.3.4-global-runtime-projection-and-pocketbase-control-plane|v1.3.4"
```

Target format:

- `label|project_root|feature|version`
- `feature` can be empty when the validation step is project-level only
- `version` can be `auto`; when omitted or missing, the script resolves an effective docs line from `AGENTS.md` or the latest local docs line

### Record Template

After capture, review and complete:

- `skills/beacon/beacon-eval-real-project/references/board-acceptance-record-template.md`
- generated `acceptance-record.md`

using:

- generated output directory
- per-project `doctor` / `help` / `status` / `release` outputs
- per-project `projection-determinism.json`
- top-level `portfolio-board.txt`
- homepage / detail / deterministic projection / debug tri-state judgments
- explicit note that this pack is a specialized board acceptance artifact, not the default latest-version release-proof record
