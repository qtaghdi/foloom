#!/bin/sh

set -u

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

report_path=${REPORT:-}
requested_evidence_dir=${EVIDENCE_DIR:-}
work_dir=$(mktemp -d)
results_file="$work_dir/results.md"
commands_file="$work_dir/commands.md"
if test -n "$requested_evidence_dir"; then
  evidence_dir=$requested_evidence_dir
  mkdir -p "$evidence_dir"
else
  evidence_dir=$work_dir
fi
safety_report="$evidence_dir/xlsx-safety.md"
performance_report="$evidence_dir/xlsx-performance-release.md"
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM

export CI=true
export GOTOOLCHAIN=${GO_TOOLCHAIN:-go1.25.0}
export GOCACHE=${GO_CACHE:-/tmp/foloom-go-cache}

checksum() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return
  fi
  shasum -a 256 "$1" | awk '{print $1}'
}

append_result() {
  printf '%s\n' "$1" >> "$results_file"
}

append_command() {
  printf '%s\n' "$1" >> "$commands_file"
}

run_check() {
  check_id=$1
  gates=$2
  description=$3
  shift 3
  log_file="$work_dir/$check_id.log"
  started_at=$(date +%s)

  printf '\n[%s] %s\n' "$check_id" "$description"
  printf 'Command:'
  printf ' %s' "$@"
  printf '\n'

  if "$@" >"$log_file" 2>&1; then
    status=Pass
  else
    status=Fail
    failed=yes
  fi
  cat "$log_file"
  finished_at=$(date +%s)
  duration=$((finished_at - started_at))
  append_result "| $check_id | $gates | $description | $status | ${duration}s |"
  append_command "- **$check_id:** \`$(printf '%s ' "$@" | sed 's/ $//')\`"
}

candidate_commit=$(git rev-parse HEAD)
candidate_dirty=no
if test -n "$(git status --porcelain)"; then
  candidate_dirty=yes
fi
failed=no

run_check safety "X-01 to X-06" "Focused safety and recovery gates" \
  make xlsx-safety-verify "REPORT=$safety_report"
run_check compatibility "X-07 automated" "Generated and LibreOffice corpus round trip" \
  make compatibility-test
run_check verification "X-01 to X-11 automated" "Frontend, Go, race, vet, and legacy regression suite" \
  make verify
run_check packaging "X-12 automated" "Frozen install, embedded assets, binary, and spaced-path smoke test" \
  make package-verify
run_check performance "Resource guardrail" "400,000-cell release performance profile" \
  make xlsx-performance-verify XLSX_PERFORMANCE_PROFILE=release "REPORT=$performance_report"

overall_status=Pass
if test "$failed" = yes; then
  overall_status=Fail
fi

summary_file="$work_dir/summary.md"
{
  printf '# XLSX Pre-Excel Candidate Evidence\n\n'
  printf 'This record covers every release-candidate check that can run without native Microsoft Excel or a visible desktop host session. It does not certify Microsoft Excel fidelity, Excel formula calculation, or native MCP App rendering.\n\n'
  printf '## Environment\n\n'
  printf -- '- Candidate commit: `%s`\n' "$candidate_commit"
  printf -- '- Working tree dirty at start: `%s`\n' "$candidate_dirty"
  printf -- '- Recorded at: `%s`\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  printf -- '- Operating system: `%s`\n' "$(uname -srm)"
  printf -- '- Go: `%s`\n' "$(go version)"
  printf -- '- Node.js: `%s`\n' "$(node --version)"
  printf -- '- pnpm: `%s`\n' "$(pnpm --version)"
  printf -- '- pnpm lockfile SHA-256: `%s`\n' "$(checksum pnpm-lock.yaml)"
  printf '\n## Fixture Hashes\n\n'
  find testdata/xlsx/compatibility -maxdepth 1 -type f -name '*.xlsx' | sort | while IFS= read -r fixture; do
    printf -- '- `%s`: `%s`\n' "$fixture" "$(checksum "$fixture")"
  done
  printf '\n## Results\n\n'
  printf '| Check | Gates | Focus | Result | Duration |\n'
  printf '| --- | --- | --- | --- | ---: |\n'
  cat "$results_file"
  printf '\nOverall status: **%s**\n' "$overall_status"
  printf '\n## Detailed Evidence\n\n'
  if test -f "$safety_report"; then
    printf -- '- Safety report SHA-256: `%s`\n' "$(checksum "$safety_report")"
  else
    printf -- '- Safety report: not produced\n'
  fi
  if test -f "$performance_report"; then
    printf -- '- Release performance report SHA-256: `%s`\n' "$(checksum "$performance_report")"
  else
    printf -- '- Release performance report: not produced\n'
  fi
  printf '\n## Commands\n\n'
  cat "$commands_file"
  printf '\n## Remaining Manual Boundaries\n\n'
  printf -- '- X-07 requires a provenance-recorded Microsoft Excel fixture and native reopen checks for every supported edit family.\n'
  printf -- '- X-08 requires observed Microsoft Excel cache and recalculation behavior for changed formulas, dependencies, unsupported functions, external references, and stale caches.\n'
  printf -- '- X-09 requires candidate-level visible Claude and Codex host runs for any newly claimed host mode. Browser fallback and embedded MCP App rendering remain separate claims.\n'
  printf -- '- X-10 requires one visible-browser grid interaction repeat against the final candidate. Automated event-boundary and responsive-shell evidence is included above.\n'
  printf -- '- X-12 still needs any platform artifact matrix claimed beyond the current operating system and architecture.\n'
  printf -- '- Power-loss durability, network filesystems, the final non-cooperating writer check-to-rename race, and durable per-operation receipts are not certified.\n'
} >"$summary_file"

printf '\n'
cat "$summary_file"

if test -n "$report_path"; then
  cp "$summary_file" "$report_path"
  printf '\nCandidate report written to %s\n' "$report_path"
fi
if test -n "$requested_evidence_dir"; then
  printf 'Detailed evidence written to %s\n' "$evidence_dir"
fi

if test "$failed" = yes; then
  exit 1
fi
