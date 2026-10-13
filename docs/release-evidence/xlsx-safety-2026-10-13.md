# XLSX Automated Safety Evidence

This record covers the automated local safety boundary for X-01 through X-06.
It is not native Microsoft Excel, host, power-loss, or network filesystem
certification.

## Environment

- Candidate commit: `88965f1d734dffe4a9161805a4ee56bcba5687f3`
- Working tree dirty at start: `no`
- Recorded at: `2026-10-13T05:31:30Z`
- Operating system: `Darwin 25.5.0 arm64`
- Go: `go version go1.25.0 darwin/arm64`
- Node.js: `v24.14.0`
- pnpm: `11.9.0`
- Build mode: `CI=true`, focused Go tests use the race detector
- pnpm lockfile SHA-256:
  `c2d194cbbb9215f029c1ef53191ed64762fe64006b74d02b2749b663cb1c45a0`

## Fixture Hashes

- `testdata/xlsx/compatibility/feature-rich.xlsx`:
  `fb8576593415b76eeea6ff4fd33c9b0739cf9a79c8448ef27b2d54033d951e45`
- `testdata/xlsx/compatibility/libreoffice.xlsx`:
  `2b253b8197b7ba53832a141dd4946aef24130e43f8fc05589365e13478cada32`

## Results

| Gate | Focus | Expected result | Observed result |
| --- | --- | --- | --- |
| X-01 | OOXML input and package boundaries | Pass | Pass |
| X-02 | Atomic batches and concurrent same-revision writers | Pass | Pass |
| X-03 | Persistence failure rollback and pending history recovery | Pass | Pass |
| X-04 | External replacement and cooperating writer conflicts | Pass | Pass |
| X-05 | Replay gaps, slow subscribers, lost responses, and transport resync | Pass | Pass |
| X-05/X-06 | Browser recovery and history availability | Pass | Pass |
| X-06 | Bounded durable undo and redo history | Pass | Pass |

## Commands

### X-01

`go test -race ./internal/formats/xlsx -run '^(TestOpenRejectsInvalidInputsWithoutMutation|TestWorkbookPackageBoundaries|TestOpenAcceptsGeneratedWorkbookAfterPackageValidation|TestSaveRejectsCandidateOutsidePackageLimitsWithoutMutation)$' -count=1`

### X-02

`go test -race ./internal/formats/xlsx -run '^(TestMixedBatchRejectsAllChangesAcrossTransactionBoundaries|TestConcurrentSameRevisionWritesCommitExactlyOneTransaction)$' -count=1`

### X-03

`go test -race ./internal/formats/xlsx -run '^(TestPersistenceFailuresPreserveSessionAndDisk|TestPendingHistoryRecoveryAfterManifestFailure)$' -count=1`

### X-04

`go test -race ./internal/formats/xlsx -run '^(TestExternalChangesBlockApplyAndRestore|TestLateExternalChangePreservesAuthoritativeMemory|TestWriterLockAndStaleSession|TestRestoredIdenticalBytesStillRejectStaleSession|TestExternalWorkbookReplacementResetsDurableHistory)$' -count=1`

### X-05

`go test -race ./internal/formats/xlsx ./internal/transport/httpapi -run '^(TestReplayGapsAndSlowSubscribersRequireResync|TestLostResponseCannotRepeatStructuralWrite|TestExternalReplacementReturnsHTTPConflict|TestSSESessionMismatchRequiresResync)$' -count=1`

### X-05/X-06

`node --test apps/web-editor/tests/recovery.test.mjs apps/web-editor/tests/history-controls.test.mjs`

### X-06

`go test -race ./internal/formats/xlsx -run '^(TestUndoAndRedoRestoreCommittedWorkbookRevisions|TestUndoAndRedoHistorySurvivesSessionRestart|TestHistoryEntryLimitSurvivesRestartAndKeepsRevisionsMonotonic|TestOversizedHistorySnapshotDoesNotBlockCommit)$' -count=1`

## Remaining Boundaries

- Native Microsoft Excel fixture provenance, reopen, and formula recalculation
  remain X-07 and X-08 gates.
- Candidate-level visible Claude and Codex host certification remains X-09.
- The source hash check cannot eliminate the final check-to-rename race with
  non-cooperating writers.
- Power-loss durability, network filesystems, and durable per-operation receipts
  are not certified.
