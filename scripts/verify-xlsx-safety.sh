#!/bin/sh

set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

report_path=${REPORT:-}
summary_file=$(mktemp)
trap 'rm -f "$summary_file"' EXIT HUP INT TERM

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

append() {
  printf '%s\n' "$1" >> "$summary_file"
}

run_gate() {
  gate=$1
  description=$2
  shift 2

  printf '\n[%s] %s\n' "$gate" "$description"
  printf 'Command:'
  printf ' %s' "$@"
  printf '\n'
  "$@"
  append "| $gate | $description | Pass | Pass |"
}

candidate_commit=$(git rev-parse HEAD)
candidate_dirty=no
if test -n "$(git status --porcelain)"; then
  candidate_dirty=yes
fi

append "# XLSX Automated Safety Evidence"
append ""
append "This record covers the automated local safety boundary for X-01 through X-06."
append "It is not native Microsoft Excel, host, power-loss, or network filesystem certification."
append ""
append "## Environment"
append ""
append "- Candidate commit: \`$candidate_commit\`"
append "- Working tree dirty at start: \`$candidate_dirty\`"
append "- Recorded at: \`$(date -u '+%Y-%m-%dT%H:%M:%SZ')\`"
append "- Operating system: \`$(uname -srm)\`"
append "- Go: \`$(go version)\`"
append "- Node.js: \`$(node --version)\`"
append "- pnpm: \`$(pnpm --version)\`"
append "- Build mode: \`CI=true\`, focused Go tests use the race detector"
append "- pnpm lockfile SHA-256: \`$(checksum pnpm-lock.yaml)\`"
append ""
append "## Fixture hashes"
append ""

find testdata/xlsx/compatibility -maxdepth 1 -type f -name '*.xlsx' | sort | while IFS= read -r fixture; do
  append "- \`$fixture\`: \`$(checksum "$fixture")\`"
done

append ""
append "## Results"
append ""
append "| Gate | Focus | Expected result | Observed result |"
append "| --- | --- | --- | --- |"

run_gate X-01 "OOXML input and package boundaries" \
  go test -race ./internal/formats/xlsx \
  -run '^(TestOpenRejectsInvalidInputsWithoutMutation|TestWorkbookPackageBoundaries|TestOpenAcceptsGeneratedWorkbookAfterPackageValidation|TestSaveRejectsCandidateOutsidePackageLimitsWithoutMutation)$' \
  -count=1

run_gate X-02 "Atomic batches and concurrent same-revision writers" \
  go test -race ./internal/formats/xlsx \
  -run '^(TestMixedBatchRejectsAllChangesAcrossTransactionBoundaries|TestConcurrentSameRevisionWritesCommitExactlyOneTransaction)$' \
  -count=1

run_gate X-03 "Persistence failure rollback and pending history recovery" \
  go test -race ./internal/formats/xlsx \
  -run '^(TestPersistenceFailuresPreserveSessionAndDisk|TestPendingHistoryRecoveryAfterManifestFailure)$' \
  -count=1

run_gate X-04 "External replacement and cooperating writer conflicts" \
  go test -race ./internal/formats/xlsx \
  -run '^(TestExternalChangesBlockApplyAndRestore|TestLateExternalChangePreservesAuthoritativeMemory|TestWriterLockAndStaleSession|TestRestoredIdenticalBytesStillRejectStaleSession|TestExternalWorkbookReplacementResetsDurableHistory)$' \
  -count=1

run_gate X-05 "Replay gaps, slow subscribers, lost responses, and transport resync" \
  go test -race ./internal/formats/xlsx ./internal/transport/httpapi \
  -run '^(TestReplayGapsAndSlowSubscribersRequireResync|TestLostResponseCannotRepeatStructuralWrite|TestExternalReplacementReturnsHTTPConflict|TestSSESessionMismatchRequiresResync)$' \
  -count=1

run_gate X-05/X-06 "Browser recovery and history availability" \
  node --test \
  apps/web-editor/tests/recovery.test.mjs \
  apps/web-editor/tests/history-controls.test.mjs

run_gate X-06 "Bounded durable undo and redo history" \
  go test -race ./internal/formats/xlsx \
  -run '^(TestUndoAndRedoRestoreCommittedWorkbookRevisions|TestUndoAndRedoHistorySurvivesSessionRestart|TestHistoryEntryLimitSurvivesRestartAndKeepsRevisionsMonotonic|TestOversizedHistorySnapshotDoesNotBlockCommit)$' \
  -count=1

append ""
append "## Commands"
append ""
append "### X-01"
append ""
append '`go test -race ./internal/formats/xlsx -run '\''^(TestOpenRejectsInvalidInputsWithoutMutation|TestWorkbookPackageBoundaries|TestOpenAcceptsGeneratedWorkbookAfterPackageValidation|TestSaveRejectsCandidateOutsidePackageLimitsWithoutMutation)$'\'' -count=1`'
append ""
append "### X-02"
append ""
append '`go test -race ./internal/formats/xlsx -run '\''^(TestMixedBatchRejectsAllChangesAcrossTransactionBoundaries|TestConcurrentSameRevisionWritesCommitExactlyOneTransaction)$'\'' -count=1`'
append ""
append "### X-03"
append ""
append '`go test -race ./internal/formats/xlsx -run '\''^(TestPersistenceFailuresPreserveSessionAndDisk|TestPendingHistoryRecoveryAfterManifestFailure)$'\'' -count=1`'
append ""
append "### X-04"
append ""
append '`go test -race ./internal/formats/xlsx -run '\''^(TestExternalChangesBlockApplyAndRestore|TestLateExternalChangePreservesAuthoritativeMemory|TestWriterLockAndStaleSession|TestRestoredIdenticalBytesStillRejectStaleSession|TestExternalWorkbookReplacementResetsDurableHistory)$'\'' -count=1`'
append ""
append "### X-05"
append ""
append '`go test -race ./internal/formats/xlsx ./internal/transport/httpapi -run '\''^(TestReplayGapsAndSlowSubscribersRequireResync|TestLostResponseCannotRepeatStructuralWrite|TestExternalReplacementReturnsHTTPConflict|TestSSESessionMismatchRequiresResync)$'\'' -count=1`'
append ""
append "### X-05/X-06"
append ""
append '`node --test apps/web-editor/tests/recovery.test.mjs apps/web-editor/tests/history-controls.test.mjs`'
append ""
append "### X-06"
append ""
append '`go test -race ./internal/formats/xlsx -run '\''^(TestUndoAndRedoRestoreCommittedWorkbookRevisions|TestUndoAndRedoHistorySurvivesSessionRestart|TestHistoryEntryLimitSurvivesRestartAndKeepsRevisionsMonotonic|TestOversizedHistorySnapshotDoesNotBlockCommit)$'\'' -count=1`'

append ""
append "## Remaining boundaries"
append ""
append "- Native Microsoft Excel fixture provenance, reopen, and formula recalculation remain X-07 and X-08 gates."
append "- Claude and Codex host certification remains X-09."
append "- The source hash check cannot eliminate the final check-to-rename race with non-cooperating writers."
append "- Power-loss durability, network filesystems, and durable per-operation receipts are not certified."
append "- Rerun this command against the final release candidate and retain its CI log or report."

printf '\n'
cat "$summary_file"

if test -n "$report_path"; then
  cp "$summary_file" "$report_path"
  printf '\nEvidence report written to %s\n' "$report_path"
fi
