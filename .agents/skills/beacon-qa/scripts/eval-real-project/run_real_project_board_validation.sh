#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
ROOT_DIR="$(cd "${SKILL_DIR}/../../.." && pwd)"
if command -v beacon >/dev/null 2>&1; then
  BEACON_RUNNER="$(command -v beacon)"
else
  BEACON_RUNNER="${ROOT_DIR}/skills/beacon/scripts/run_beacon.sh"
fi

REQUESTED_VERSION="auto"
TIMESTAMP="$(date +%Y%m%dT%H%M%S)"
OUTPUT_DIR="${ROOT_DIR}/.beacon/validation/real-project-board/${TIMESTAMP}"
LEDGER_ROOT=""
CHECKLIST_PATH=""
declare -a TARGETS=()
declare -a TARGET_ROWS=()

usage() {
  cat <<'EOF'
Usage:
  run_real_project_board_validation.sh [--version VERSION|auto] [--output-dir DIR]
                                      [--target "label|project_root|feature|version"]...

Description:
  Read-only capture pack for Beacon specialized multi-project board validation.
  It does not mutate target projects. Outputs are written under the Beacon repo,
  and an isolated runtime ledger is created inside the capture pack.
  This script does not replace current-version real-project release proof.

Examples:
  bash skills/beacon/beacon-eval-real-project/scripts/run_real_project_board_validation.sh

  bash skills/beacon/beacon-eval-real-project/scripts/run_real_project_board_validation.sh \
    --version v1.3.4 \
    --output-dir .beacon/validation/real-project-board/v1.3.4-sample

  bash skills/beacon/beacon-eval-real-project/scripts/run_real_project_board_validation.sh \
    --target "rolo|/Users/apple/Developer/Company/rolo||auto" \
    --target "beacon|/Users/apple/Developer/Personal/products/beacon|beacon-v1.3.4-global-runtime-projection-and-pocketbase-control-plane|v1.3.4"
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      REQUESTED_VERSION="$2"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --target)
      TARGETS+=("$2")
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ ${#TARGETS[@]} -eq 0 ]]; then
  TARGETS+=("rolo|/Users/apple/Developer/Company/rolo||auto")
  TARGETS+=("slow_uni_bmtop|/Users/apple/Developer/Personal/products/slow_uni_bmtop||auto")
  TARGETS+=("shopxo_canada|/Users/apple/Developer/Personal/products/shopxo_canada||auto")
  TARGETS+=("beacon|/Users/apple/Developer/Personal/products/beacon|beacon-v1.3.4-global-runtime-projection-and-pocketbase-control-plane|v1.3.4")
fi

mkdir -p "${OUTPUT_DIR}"
LEDGER_ROOT="${OUTPUT_DIR}/runtime-ledger"

if [[ "${REQUESTED_VERSION}" != "auto" ]]; then
  candidate_checklist="${ROOT_DIR}/docs/beacon/${REQUESTED_VERSION}/execution/real-project-validation-checklist-beacon-v1.3.4-global-runtime-projection-and-pocketbase-control-plane.md"
  if [[ -f "${candidate_checklist}" ]]; then
    CHECKLIST_PATH="docs/beacon/${REQUESTED_VERSION}/execution/real-project-validation-checklist-beacon-v1.3.4-global-runtime-projection-and-pocketbase-control-plane.md"
  fi
fi

run_capture() {
  local output_file="$1"
  shift
  local status_file="${output_file}.exit"

  (
    set +e
    if [[ "${1:-}" == "${BEACON_RUNNER}" ]]; then
      shift
      if [[ "${BEACON_RUNNER}" == *.sh ]]; then
        BEACON_RUNTIME_LEDGER_ROOT="${LEDGER_ROOT}" bash "${BEACON_RUNNER}" "$@" >"${output_file}" 2>&1
      else
        BEACON_RUNTIME_LEDGER_ROOT="${LEDGER_ROOT}" "${BEACON_RUNNER}" "$@" >"${output_file}" 2>&1
      fi
    else
      BEACON_RUNTIME_LEDGER_ROOT="${LEDGER_ROOT}" "$@" >"${output_file}" 2>&1
    fi
    printf '%s\n' "$?" >"${status_file}"
  )
}

run_shell_capture() {
  local output_file="$1"
  local command="$2"
  local status_file="${output_file}.exit"

  (
    set +e
    BEACON_RUNTIME_LEDGER_ROOT="${LEDGER_ROOT}" bash -lc "${command}" >"${output_file}" 2>&1
    printf '%s\n' "$?" >"${status_file}"
  )
}

detect_effective_version() {
  local project_root="$1"
  local requested="$2"
  local context_version=""
  local latest_version=""

  if [[ "${requested}" != "auto" && -d "${project_root}/docs/beacon/${requested}" ]]; then
    printf '%s\n' "${requested}"
    return 0
  fi

  if [[ -f "${project_root}/AGENTS.md" ]]; then
    context_version="$(grep -o 'BEACON:VERSION:[^ >]*' "${project_root}/AGENTS.md" | head -n 1 | cut -d: -f3 || true)"
    if [[ -n "${context_version}" && -d "${project_root}/docs/beacon/${context_version}" ]]; then
      printf '%s\n' "${context_version}"
      return 0
    fi
  fi

  if [[ -d "${project_root}/docs/beacon" ]]; then
    latest_version="$(
      find "${project_root}/docs/beacon" -mindepth 1 -maxdepth 1 -type d -name 'v*' -exec basename {} \; \
        | sort -V \
        | tail -n 1
    )"
  fi

  printf '%s\n' "${latest_version}"
}

infer_feature() {
  local project_root="$1"
  local version="$2"
  local docs_root="${project_root}/docs/beacon/${version}"
  local prd_dir="${docs_root}/prd"
  local candidate=""
  local base=""

  if [[ ! -d "${prd_dir}" ]]; then
    return 0
  fi

  while IFS= read -r candidate; do
    base="$(basename "${candidate}" .md)"
    if [[ -f "${docs_root}/user-story/${base}.md" && -f "${docs_root}/qa/test-cases/${base}.md" ]]; then
      printf '%s\n' "${base}"
      return 0
    fi
  done < <(find "${prd_dir}" -maxdepth 1 -type f -name '*.md' | sort)
}

capture_git_surface() {
  local project_root="$1"
  local output_file="$2"
  local status_file="${output_file}.exit"

  (
    set +e
    if git -C "${project_root}" rev-parse --show-toplevel >/dev/null 2>&1; then
      git -C "${project_root}" status --short
      printf 'git_surface=root-repo\n'
      printf '%s\n' "0" >"${status_file}"
      exit 0
    fi

    python3 - "${project_root}" <<'PY'
import json
import sys
from pathlib import Path

from beacon.utils.runtime_projection import discover_git_scopes

project_root = Path(sys.argv[1])
scopes = discover_git_scopes(project_root)
payload = {
    "git_surface": "workspace-child-repos",
    "scope_count": len(scopes),
    "scopes": scopes,
}
print(json.dumps(payload, ensure_ascii=False, indent=2))
PY
    printf '%s\n' "$?" >"${status_file}"
  ) >"${output_file}" 2>&1
}

write_projection_determinism_report() {
  local output_file="$1"
  local first_json="$2"
  local second_json="$3"
  python3 - "${first_json}" "${second_json}" <<'PY' >"${output_file}"
import json
import sys
from pathlib import Path

first = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
second = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))

first_projection = ((first.get("runtime_projection") or {}).get("projection") or {})
second_projection = ((second.get("runtime_projection") or {}).get("projection") or {})

report = {
    "first_source_hash": first_projection.get("source_hash", ""),
    "second_source_hash": second_projection.get("source_hash", ""),
    "first_projection_seq": first_projection.get("projection_seq", ""),
    "second_projection_seq": second_projection.get("projection_seq", ""),
    "preflight_status": first_projection.get("preflight_status", ""),
    "stable": (
        first_projection.get("source_hash", "") == second_projection.get("source_hash", "")
        and first_projection.get("projection_seq", "") == second_projection.get("projection_seq", "")
    ),
}

print(json.dumps(report, ensure_ascii=False, indent=2))
PY
}

write_acceptance_record() {
  local output_file="$1"
  local executed_at="$2"

  {
    printf '# Beacon Real Project Board Acceptance Record\n\n'
    printf -- '- feature: `%s`\n' 'beacon-v1.3.4-global-runtime-projection-and-pocketbase-control-plane'
    printf -- '- requested_version: `%s`\n' "${REQUESTED_VERSION}"
    printf -- '- validation_mode: `read-only multi-project board acceptance`\n'
    printf -- '- validation_scope: `specialized-board-acceptance-only`\n'
    printf -- '- not_a_substitute_for: `current-version real-project release proof`\n'
    printf -- '- executed_at: `%s`\n' "${executed_at}"
    printf -- '- executor: `capture-script`\n'
    printf -- '- output_dir: `%s`\n\n' "${OUTPUT_DIR}"
    printf '## 0. Boundary\n\n'
    printf -- '- This capture is specialized board acceptance only.\n'
    printf -- '- It does not prove the latest Beacon line is fully validated on a single real project.\n'
    printf -- '- Record companion current-version release-proof evidence separately when needed.\n\n'
    printf '## 1. Validation Scope\n\n'
    printf '### Projects\n\n'
    local row=""
    local index=1
    local label=""
    local project_root=""
    local feature=""
    local version=""
    for row in "${TARGET_ROWS[@]}"; do
      IFS='|' read -r label project_root feature version <<<"${row}"
      printf '%s. `%s` -> `%s` (version: `%s`' "${index}" "${label}" "${project_root}" "${version}"
      if [[ -n "${feature}" ]]; then
        printf ', feature: `%s`' "${feature}"
      fi
      printf ')\n'
      index=$((index + 1))
    done
    printf '\n### Truth / Projection Boundary\n\n'
    printf -- '- truth plane: `docs/beacon + .beacon`\n'
    printf -- '- projection plane: isolated `SQLite + PocketBase mirror pack`\n'
    printf -- '- out of scope: `omc / omx / Beacon 外运行时`\n\n'
    printf '## 2. Executed Commands\n\n'
    printf '```bash\n'
    printf 'bash skills/beacon/beacon-eval-real-project/scripts/run_real_project_board_validation.sh'
    printf ' --version %s' "${REQUESTED_VERSION}"
    printf ' --output-dir %s' "${OUTPUT_DIR}"
    printf '\n```\n\n'
    printf 'Top-level generated artifacts:\n\n'
    printf -- '- `README.md`\n'
    printf -- '- `portfolio-board.txt`\n'
    printf -- '- `portfolio-board.json`\n'
    printf -- '- `acceptance-record.md`\n'
    printf -- '- `runtime-ledger/`\n\n'
    printf 'Per-project generated artifacts include:\n\n'
    printf -- '- `doctor.json`\n'
    printf -- '- `help.json`\n'
    printf -- '- `status.json`\n'
    printf -- '- `status-repeat.json`\n'
    printf -- '- `status-board.txt`\n'
    printf -- '- `projection-determinism.json`\n'
    printf -- '- `qa-status.json`\n'
    printf -- '- `release-check.txt`\n'
    printf -- '- `release-scorecard.json`\n\n'
    printf '## 3. Homepage Summary Result\n\n'
    printf 'Use `portfolio-board.txt` and `portfolio-board.json` as the canonical multi-project homepage artifact.\n\n'
    printf '### Verdict\n\n'
    printf -- '- [ ] pass\n- [ ] blocked by project state\n- [ ] blocked by Beacon defect\n- [ ] not yet implemented\n\n'
    printf '### Notes\n\n'
    printf -- '- Did all projects appear on the board?\n'
    printf -- '- Did homepage stay lifecycle-first and low-noise?\n'
    printf -- '- Could CEO distinguish the project set in one scan?\n\n'
    printf '## 4. Project Detail Result\n\n'
    for row in "${TARGET_ROWS[@]}"; do
      IFS='|' read -r label project_root feature version <<<"${row}"
      printf '### %s\n\n' "${label}"
      printf -- '- version: `%s`\n' "${version}"
      if [[ -n "${feature}" ]]; then
        printf -- '- feature: `%s`\n' "${feature}"
      fi
      printf -- '- verdict:\n- project_kind:\n- lifecycle:\n- debug_state:\n- explanation_quality:\n- notes:\n\n'
    done
    printf '## 5. Deterministic Projection Result\n\n'
    printf 'Use each project folder `projection-determinism.json` plus `status.json` / `status-repeat.json`.\n\n'
    printf '### Verdict\n\n'
    printf -- '- [ ] pass\n- [ ] blocked by project state\n- [ ] blocked by Beacon defect\n- [ ] not yet implemented\n\n'
    printf '### Notes\n\n'
    printf -- '- Was the same truth snapshot projected consistently?\n'
    printf -- '- Were `source_hash / projection_seq` stable?\n'
    printf -- '- Did preflight mark conflicts instead of silently guessing?\n\n'
    printf '## 6. Debug Tri-State Result\n\n'
    printf 'Observed states:\n\n'
    for row in "${TARGET_ROWS[@]}"; do
      IFS='|' read -r label _project_root _feature _version <<<"${row}"
      printf -- '- `%s`:\n' "${label}"
    done
    printf '\nVerdict:\n\n'
    printf -- '- [ ] pass\n- [ ] blocked by project state\n- [ ] blocked by Beacon defect\n- [ ] not yet implemented\n\n'
    printf '## 7. Degradation / Rebuild Result\n\n'
    printf 'Use `status.json` as the truth-only projection baseline and `portfolio-board.json` as the post-sync board snapshot.\n\n'
    printf '### Verdict\n\n'
    printf -- '- [ ] pass\n- [ ] blocked by project state\n- [ ] blocked by Beacon defect\n- [ ] not yet implemented\n\n'
    printf '### Notes\n\n'
    printf -- '- Could projects still be understood from local truth when ledger output is ignored?\n'
    printf -- '- Could minimum board state be rebuilt from the captured truth snapshots?\n\n'
    printf '## 8. Findings Classification\n\n'
    printf '### Beacon Defects\n\n1.\n2.\n3.\n\n'
    printf '### Project-State Blockers\n\n1.\n2.\n3.\n\n'
    printf '### Not-Yet-Implemented Acceptance Gaps\n\n1.\n2.\n3.\n\n'
    printf '## 9. Final Recommendation\n\n'
    printf -- '- [ ] safe to use on these projects\n'
    printf -- '- [ ] safe for read-only use only\n'
    printf -- '- [ ] blocked until Beacon defect is fixed\n'
    printf -- '- [ ] blocked until feature implementation reaches acceptance line\n\n'
    printf '## 10. Linked Artifacts\n\n'
    printf -- '- capture directory: `%s`\n' "${OUTPUT_DIR}"
    printf -- '- ledger root: `%s`\n' "${LEDGER_ROOT}"
    if [[ -n "${CHECKLIST_PATH}" ]]; then
      printf -- '- checklist: `%s`\n' "${CHECKLIST_PATH}"
    fi
    printf -- '- portfolio board: `portfolio-board.txt`\n'
    printf -- '- portfolio board json: `portfolio-board.json`\n'
    printf -- '- follow-up issue / PR links:\n'
  } >"${output_file}"
}

printf 'requested_version=%s\n' "${REQUESTED_VERSION}" >"${OUTPUT_DIR}/capture.env"
printf 'validation_scope=%s\n' "specialized-board-acceptance-only" >>"${OUTPUT_DIR}/capture.env"
printf 'executed_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"${OUTPUT_DIR}/capture.env"
printf 'beacon_root=%s\n' "${ROOT_DIR}" >>"${OUTPUT_DIR}/capture.env"
printf 'ledger_root=%s\n' "${LEDGER_ROOT}" >>"${OUTPUT_DIR}/capture.env"

for target in "${TARGETS[@]}"; do
  IFS='|' read -r label project_root feature version_override <<<"${target}"
  if [[ -z "${label}" || -z "${project_root}" ]]; then
    echo "Invalid target spec: ${target}" >&2
    exit 1
  fi

  effective_version="$(detect_effective_version "${project_root}" "${version_override:-${REQUESTED_VERSION}}")"
  effective_feature="${feature}"
  if [[ -z "${effective_feature}" && -n "${effective_version}" ]]; then
    effective_feature="$(infer_feature "${project_root}" "${effective_version}")"
  fi
  TARGET_ROWS+=("${label}|${project_root}|${effective_feature}|${effective_version}")

  project_dir="${OUTPUT_DIR}/${label}"
  mkdir -p "${project_dir}"

  {
    printf 'label=%s\n' "${label}"
    printf 'project_root=%s\n' "${project_root}"
    printf 'requested_version=%s\n' "${REQUESTED_VERSION}"
    printf 'effective_version=%s\n' "${effective_version}"
    printf 'feature=%s\n' "${effective_feature}"
  } >"${project_dir}/target.env"

  if [[ ! -d "${project_root}" ]]; then
    printf 'missing project root: %s\n' "${project_root}" >"${project_dir}/missing.txt"
    printf '1\n' >"${project_dir}/missing.txt.exit"
    continue
  fi

  capture_git_surface "${project_root}" "${project_dir}/git-status.txt"
  run_shell_capture "${project_dir}/docs-version.txt" "test -n '${effective_version}' && test -d '${project_root}/docs/beacon/${effective_version}' && echo 'docs version exists' || { echo 'docs version missing'; exit 1; }"
  run_shell_capture "${project_dir}/docs-tree.txt" "find '${project_root}/docs/beacon/${effective_version}' -maxdepth 2 -type f 2>/dev/null | sort"
  run_shell_capture "${project_dir}/state-tree.txt" "find '${project_root}/.beacon' -maxdepth 3 -type f 2>/dev/null | sort | head -n 200"
  run_capture "${project_dir}/doctor.json" "${BEACON_RUNNER}" doctor --json --project-root "${project_root}"

  if [[ -n "${effective_feature}" ]]; then
    run_capture "${project_dir}/help.json" "${BEACON_RUNNER}" help --project-root "${project_root}" --version "${effective_version}" --feature "${effective_feature}" --json
    run_capture "${project_dir}/status.json" "${BEACON_RUNNER}" status --project-root "${project_root}" --version "${effective_version}" --feature "${effective_feature}"
    run_capture "${project_dir}/status-repeat.json" "${BEACON_RUNNER}" status --project-root "${project_root}" --version "${effective_version}" --feature "${effective_feature}"
    run_capture "${project_dir}/status-board.txt" "${BEACON_RUNNER}" status show-status --project-root "${project_root}" --version "${effective_version}" --feature "${effective_feature}" --board
    run_capture "${project_dir}/test-case-gate.txt" "${BEACON_RUNNER}" test-case gate "${effective_feature}" -p "${project_root}" -v "${effective_version}"
    run_capture "${project_dir}/implement-status.json" "${BEACON_RUNNER}" implement status "${effective_feature}" -p "${project_root}" -v "${effective_version}" --json
    run_capture "${project_dir}/qa-evolve.json" "${BEACON_RUNNER}" qa evolve "${effective_feature}" -p "${project_root}" -v "${effective_version}" --no-write-matrix --json
    run_capture "${project_dir}/qa-status.json" "${BEACON_RUNNER}" qa status -p "${project_root}" --json
  else
    run_capture "${project_dir}/help.json" "${BEACON_RUNNER}" help --project-root "${project_root}" --version "${effective_version}" --json
    run_capture "${project_dir}/status.json" "${BEACON_RUNNER}" status --project-root "${project_root}" --version "${effective_version}"
    run_capture "${project_dir}/status-repeat.json" "${BEACON_RUNNER}" status --project-root "${project_root}" --version "${effective_version}"
    run_capture "${project_dir}/status-board.txt" "${BEACON_RUNNER}" status show-status --project-root "${project_root}" --version "${effective_version}" --board
    run_capture "${project_dir}/qa-status.json" "${BEACON_RUNNER}" qa status -p "${project_root}" --json
  fi

  run_capture "${project_dir}/release-check.txt" "${BEACON_RUNNER}" release check "${effective_version}" -p "${project_root}"
  run_capture "${project_dir}/release-scorecard.json" "${BEACON_RUNNER}" release scorecard "${effective_version}" -p "${project_root}" --json
  run_capture "${project_dir}/prd-list.txt" "${BEACON_RUNNER}" prd list -p "${project_root}" -v "${effective_version}"
  run_capture "${project_dir}/user-story-list.txt" "${BEACON_RUNNER}" user-story list -p "${project_root}" -v "${effective_version}"
  run_capture "${project_dir}/test-case-list.txt" "${BEACON_RUNNER}" test-case list -p "${project_root}" -v "${effective_version}"

  if [[ -f "${project_dir}/status.json" && -f "${project_dir}/status-repeat.json" ]]; then
    write_projection_determinism_report "${project_dir}/projection-determinism.json" "${project_dir}/status.json" "${project_dir}/status-repeat.json"
  fi
done

BEACON_RUNTIME_LEDGER_ROOT="${LEDGER_ROOT}" python3 - "${OUTPUT_DIR}" <<'PY' >"${OUTPUT_DIR}/portfolio-board.json"
import json
import sys

from beacon.utils.runtime_projection_store import list_projects

output_dir = sys.argv[1]
payload = {
    "output_dir": output_dir,
    "portfolio_projects": list_projects(limit=50),
}
print(json.dumps(payload, ensure_ascii=False, indent=2))
PY

BEACON_RUNTIME_LEDGER_ROOT="${LEDGER_ROOT}" python3 - "${OUTPUT_DIR}/portfolio-board.json" <<'PY' >"${OUTPUT_DIR}/portfolio-board.txt"
import json
import sys
from pathlib import Path

payload = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
rows = payload.get("portfolio_projects", [])
print("Beacon Portfolio Board")
print(f"- project_count: {len(rows)}")
for index, row in enumerate(rows, start=1):
    print(
        f"- project.{index}: "
        f"{row.get('display_name') or row.get('project_key')} | "
        f"version={row.get('docs_version')} | "
        f"kind={row.get('project_kind')} | "
        f"stage={row.get('current_stage')} | "
        f"progress={row.get('progress_label')} | "
        f"debug={row.get('debug_state')} | "
        f"next={row.get('next_action')}"
    )
PY

cat >"${OUTPUT_DIR}/README.md" <<EOF
# Beacon Specialized Real Project Board Validation Capture

- requested_version: \`${REQUESTED_VERSION}\`
- validation_scope: \`specialized-board-acceptance-only\`
- executed_at: \`$(date -u +%Y-%m-%dT%H:%M:%SZ)\`
- output_dir: \`${OUTPUT_DIR}\`
- isolated_ledger_root: \`${LEDGER_ROOT}\`

## Targets

$(for row in "${TARGET_ROWS[@]}"; do IFS='|' read -r label project_root feature version <<<"${row}"; printf -- "- \`%s\` -> \`%s\` (version: \`%s\`" "${label}" "${project_root}" "${version}"; if [[ -n "${feature}" ]]; then printf -- ", feature: \`%s\`" "${feature}"; fi; printf ')\n'; done)

## Canonical Artifacts

- \`portfolio-board.txt\`
- \`portfolio-board.json\`
- \`acceptance-record.md\`
- \`runtime-ledger/\`

## Boundary

- This pack is only for specialized multi-project board acceptance.
- It does not replace current-version real-project release proof.

## Next Step

Use this capture pack with:

- \`skills/beacon/beacon-eval-real-project/references/board-acceptance-record-template.md\`
$(if [[ -n "${CHECKLIST_PATH}" ]]; then printf -- "- \`%s\`\n" "${CHECKLIST_PATH}"; fi)
EOF

write_acceptance_record "${OUTPUT_DIR}/acceptance-record.md" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"

printf '%s\n' "Capture complete: ${OUTPUT_DIR}"
