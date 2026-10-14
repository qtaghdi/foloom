# XLSX 자동 안전성 증거

이 기록은 X-01부터 X-06까지 자동화된 로컬 안전 경계를 다룹니다. 실제 Microsoft
Excel, host, 전원 손실, network filesystem 인증이 아닙니다.

## 환경

- Candidate commit: `88965f1d734dffe4a9161805a4ee56bcba5687f3`
- 시작 시 working tree dirty: `no`
- 기록 시각: `2026-10-13T05:31:30Z`
- 운영체제: `Darwin 25.5.0 arm64`
- Go: `go version go1.25.0 darwin/arm64`
- Node.js: `v24.14.0`
- pnpm: `11.9.0`
- Build mode: `CI=true`, 집중 Go test는 race detector 사용
- pnpm lockfile SHA-256: `c2d194cbbb9215f029c1ef53191ed64762fe64006b74d02b2749b663cb1c45a0`

## Fixture hash

- `feature-rich.xlsx`: `fb8576593415b76eeea6ff4fd33c9b0739cf9a79c8448ef27b2d54033d951e45`
- `libreoffice.xlsx`: `2b253b8197b7ba53832a141dd4946aef24130e43f8fc05589365e13478cada32`

## 결과

| Gate | 초점 | 예상 | 관찰 |
| --- | --- | --- | --- |
| X-01 | OOXML 입력 및 package 경계 | Pass | Pass |
| X-02 | 원자적 batch와 같은 리비전의 동시 writer | Pass | Pass |
| X-03 | 저장 실패 rollback과 pending history 복구 | Pass | Pass |
| X-04 | 외부 교체와 협조 writer conflict | Pass | Pass |
| X-05 | replay gap, slow subscriber, lost response, transport resync | Pass | Pass |
| X-05/X-06 | browser recovery와 history availability | Pass | Pass |
| X-06 | 제한된 영속 undo와 redo | Pass | Pass |

## 명령

- X-01: `go test -race ./internal/formats/xlsx -run '^(TestOpenRejectsInvalidInputsWithoutMutation|TestWorkbookPackageBoundaries|TestOpenAcceptsGeneratedWorkbookAfterPackageValidation|TestSaveRejectsCandidateOutsidePackageLimitsWithoutMutation)$' -count=1`
- X-02: `go test -race ./internal/formats/xlsx -run '^(TestMixedBatchRejectsAllChangesAcrossTransactionBoundaries|TestConcurrentSameRevisionWritesCommitExactlyOneTransaction)$' -count=1`
- X-03: `go test -race ./internal/formats/xlsx -run '^(TestPersistenceFailuresPreserveSessionAndDisk|TestPendingHistoryRecoveryAfterManifestFailure)$' -count=1`
- X-04: `go test -race ./internal/formats/xlsx -run '^(TestExternalChangesBlockApplyAndRestore|TestLateExternalChangePreservesAuthoritativeMemory|TestWriterLockAndStaleSession|TestRestoredIdenticalBytesStillRejectStaleSession|TestExternalWorkbookReplacementResetsDurableHistory)$' -count=1`
- X-05: `go test -race ./internal/formats/xlsx ./internal/transport/httpapi -run '^(TestReplayGapsAndSlowSubscribersRequireResync|TestLostResponseCannotRepeatStructuralWrite|TestExternalReplacementReturnsHTTPConflict|TestSSESessionMismatchRequiresResync)$' -count=1`
- X-05/X-06: `node --test apps/web-editor/tests/recovery.test.mjs apps/web-editor/tests/history-controls.test.mjs`
- X-06: `go test -race ./internal/formats/xlsx -run '^(TestUndoAndRedoRestoreCommittedWorkbookRevisions|TestUndoAndRedoHistorySurvivesSessionRestart|TestHistoryEntryLimitSurvivesRestartAndKeepsRevisionsMonotonic|TestOversizedHistorySnapshotDoesNotBlockCommit)$' -count=1`

## 남은 경계

- Microsoft Excel fixture 출처, reopen, 수식 재계산은 X-07과 X-08입니다.
- Candidate 수준의 보이는 Claude 및 Codex host 인증은 X-09입니다.
- source hash 검사는 비협조 writer와 마지막 check-to-rename race를 없애지 못합니다.
- 전원 손실, network filesystem, 작업별 영속 receipt는 인증하지 않았습니다.
