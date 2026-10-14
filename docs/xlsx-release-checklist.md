# XLSX Stable Release Checklist

Status: release planning, not a release certification.
Baseline: implementation `4a6cfeb`, shared specification `e79e7e0`.
Requirements refer to the [shared specification](product-spec.md).
Capabilities remain bounded by the [format matrix](format-support.md).

## Release Scope

The first stable milestone supports one existing local XLSX workbook per
process, human and AI edits through the same revisioned session, and native
XLSX persistence. Include cells, formula storage, basic formatting, rectangular
paste, merges, row and column operations, existing image and chart placement,
resize and deletion, AI chart title updates, and bounded undo and redo.

Exclude blank creation, templates, format conversion, hosted collaboration,
staged proposal review, macro execution, encrypted editing, image insertion,
advanced chart authoring, and complete Excel formula calculation. Image
insertion and advanced chart editing remain planned follow-ups. This explicitly
narrows the stable milestone from the earlier object-editing backlog; it does
not remove those features from the roadmap.

Formula policy: preserve formula expressions for supported edits and distinguish
stored or cached values from browser-calculated results. Do not describe browser
results as authoritative Excel calculation. Before release, verify the behavior
of changed formulas, changed dependencies, unsupported functions, external
references, and stale caches, including reopening in Excel. The release must
document when recalculation is required and show a limitation where a result
cannot be established. No server calculation engine is selected by this spec.
The structured policy, cached-value labeling, browser warning, and native
recalculation request are implemented and documented in the
[XLSX formula policy](xlsx-formula-policy.md). Native Excel evidence remains a
release gate.

## Evidence Status

Recovery implementation now has [dedicated tests and an operator guide](xlsx-recovery.md).
X-03 through X-05 have implementation evidence for candidate rollback, failure
injection, source hash guards, cooperating writer locks, replay reset, and paused
browser queues. They remain candidate sign-off gates, with the external-writer
race and crash-durability limits documented in that guide.

Run `make xlsx-safety-verify` to produce the focused automated evidence for
X-01 through X-06. The command records the commit, environment, lockfile and
fixture hashes, exact focused checks, observed results, and remaining boundaries.
It runs in pull request and main-branch CI as `Verify XLSX safety gates`.
The first clean local baseline is recorded in
[XLSX automated safety evidence](release-evidence/xlsx-safety-2026-10-12.md).
Performance profiles, budgets, commands, and evidence boundaries are defined in
[XLSX performance verification](xlsx-performance.md). The manual fixture and
reopen workflow is defined in
[native Microsoft Excel verification](xlsx-native-excel-verification.md).

Run `make xlsx-pre-excel-verify` before native Excel testing to execute every
candidate check available without Excel or a visible desktop host session. Set
`REPORT=/absolute/path/to/report.md` and
`EVIDENCE_DIR=/absolute/path/to/evidence` to retain the aggregate record plus
the focused safety and release-performance reports. The aggregate command runs
the safety gates, generated and LibreOffice compatibility corpus, complete
frontend and Go verification, clean package verification, and the 400,000-cell
release performance profile. It leaves native Excel and visible-host results
explicitly manual.
The complete regression suite also runs on pull requests and main-branch pushes
as `Verify full XLSX regression suite`. Existing focused safety, performance,
and clean-package workflows remain separate so their failure domains stay clear.
The workflow installs the pinned Python dependencies in
`legacy/xlsx-python/requirements.txt` before running the preserved MVP
regression, so a clean runner does not depend on preinstalled Python packages.
The clean pre-Excel run for implementation commit `88965f1` is recorded in
[XLSX pre-Excel candidate evidence](release-evidence/xlsx-pre-excel-2026-10-13.md),
with linked focused safety and release-performance records. All automated steps
passed. Native Excel and visible-host boundaries remain open as listed there.

Existing evidence means a test or historical record exists, not that this
release candidate has passed it. Every checkbox remains open until evidence is
recorded against the candidate commit. A documentation-only change does not
require rerunning the application suite.

XLSX package preflight now rejects non-regular sources, invalid OOXML ZIP
containers, encrypted entries, unsafe or duplicate part names, more than 10,000
parts, parts larger than 256 MiB expanded, total expansion above 512 MiB, and
compressed sources above 100 MiB before the workbook parser runs.

Transaction tests now reject a mixed valid and invalid batch without changing
memory, source bytes, revision, history, or events. A synchronized concurrent
same-revision test proves that exactly one writer commits across those same
boundaries while the other receives a revision conflict.

History limit tests now retain only the newest ten snapshots, preserve that
boundary across restart, omit snapshots above the 32 MiB retention limit
without failing the committed edit, and keep revisions increasing through all
available undo and redo operations. Browser tests verify disabled controls for
unavailable actions and updated availability after local, remote AI, history,
reload, and recovery transitions.

| Gate | Requirements | Existing evidence | Remaining closure condition |
| --- | --- | --- | --- |
| X-01 Input boundaries | DOC-01 | Regular-file and OOXML package preflight; compressed, expanded, part-count, duplicate-name, unsafe-path, encryption, corrupt-input, and no-mutation tests | Candidate run and review of supported restricted or encrypted input messaging |
| X-02 Transactions | EDIT-01, EDIT-03, SAVE-01 | Mixed-batch atomicity and synchronized same-revision writer tests across memory, disk, history, revision, and events | Candidate run and review of the recorded invariants |
| X-03 Persistence failures | SAVE-01, REC-01 | Temporary creation, write, short-write, sync, close, rename, rollback, and pending-history failure tests for edits and history restores; no false committed event | Candidate run and review of the recorded invariants |
| X-04 External writers | REC-01 | Source checks before mutation and replacement, cooperating writer lock, external replacement, late-change, stale-session, and history-reset tests | Candidate run and acceptance of the documented single-writer boundary and final check-to-rename race |
| X-05 Disconnect and retry | REC-01 | Replay-gap, slow-subscriber, session-mismatch, HTTP conflict, lost-response, paused-queue, reconnect, resync, and history sequencing tests | Candidate run and review of the conservative no-blind-retry behavior |
| X-06 History | HIST-01 | Undo/redo, restart, replacement, snapshot size/count limit, monotonic revision, event availability, and disabled-control tests | Candidate run and review of the recorded invariants |
| X-07 Native fidelity | COMP-01 | Generated and LibreOffice corpus; object round-trip tests | Add provenance-recorded Excel fixture; verify native reopen and unrelated content across all supported edit families |
| X-08 Formula expectations | COMP-01 | Formula storage, value-status, structured-policy, warning, and native-recalculation property tests | Verify changed formulas, dependencies, unsupported functions, external references, and stale caches in native Excel |
| X-09 Host workflows | HOST-01, AUTH-01 | Historical tools and Codex browser fallback records | Candidate-level Claude and Codex tests with explicit mode, versions, permissions, human/AI edits, conflict, and native reopen |
| X-10 UI and limits | EDIT-02, COMP-01 | Keyboard edit, atomic paste, server history, live pending/success/failure/conflict/recovery states, focus and touch targets, reduced motion, safe areas, and desktop, laptop, and mobile shell viewport evidence | Visible-browser grid interaction repeat against the release candidate |
| X-11 Access and diagnostics | AUTH-01 | Negative browser and MCP credential matrix, cross-role rejection, foreign-origin no-mutation, fixed-length token comparison, response non-disclosure, security headers, loopback, and bounded single-use launch links | Candidate run and review of the recorded invariants |
| X-12 Packaging | HOST-01 | Clean-checkout CI with frozen dependency installation, browser and self-contained MCP App asset validation, compiled executable smoke test with a spaced absolute path, embedded delivery tests, version and workbook preflight commands, and unsigned macOS, Linux, and Windows archive generation with checksums | Run the candidate artifact matrix; select distribution terms; keep signing, notarization, installers, launcher, and updates explicit alpha limitations |

Source anchors: [session tests](../internal/formats/xlsx/session_test.go),
[corpus tests](../internal/formats/xlsx/compatibility_test.go),
[persistence](../internal/formats/xlsx/persistence.go), and
[transport tests](../internal/transport/httpapi/server_test.go).

## Safety Gates That Cannot Be Waived by a Warning

Silent source loss, partial commits, stale writes accepted as current, and false
persistence success block release. If a feature cannot meet a gate, disable its
write path or restrict the supported mode and update the scope and matrix.
A warning alone does not make unsafe overwrite acceptable.

For X-04, a file hash check alone does not eliminate the race between checking
and replacement. Record the concurrency strategy and test the promised boundary.
Do not claim coordination with Excel or another process without evidence.
For X-03, include the history manifest and rollback/reload failure paths. A
successful file rename does not alone prove crash-consistent session history.
For X-11, separate deliberate credential delivery at startup from routine logs;
record any remediation needed before publishing a release binary.

## Candidate Verification Procedure

1. Run `make xlsx-pre-excel-verify` to execute and record every automated
   pre-Excel candidate gate. Retain its aggregate, safety, and performance
   reports outside the repository while the candidate is under review.
2. Review the focused safety report, which records the commit, environment,
   lockfile and fixture hashes, build mode, and X-01 through X-06 results. The
   aggregate command also builds the browser bundles, runs the complete Go and
   frontend suites with race and vet checks, executes the Python regression,
   verifies a clean package, and runs the release performance profile.
3. Exercise missing failure scenarios in the gate table with reproducible
   tests. Record failures as open gates, not as expected passes.
4. On disposable corpus copies, perform human and AI edits and reopen the saved
   outputs in Excel and LibreOffice. Record application versions and differences.
5. Execute the [host procedure](host-compatibility.md) for each claimed mode.
   Native MCP App rendering and browser fallback are separate results. If a
   host only passes fallback, advertise fallback explicitly. Claude's UI path
   remains pending in the historical record; neither host has certified native
   embedded rendering there.
6. Capture UI evidence at desktop, small laptop, and mobile fallback sizes.
   Record actual viewport sizes, keyboard behavior, and reduced-motion behavior.
7. Reconcile README, format matrix, host matrix, release limitations, and dated
   changelog. Keep private workbooks, credentials, and temporary reports out of Git.

Known configured bounds are 100 MiB input size, 10,000 cells per bounded range,
1,000 operations per batch, 512 retained events, ten history snapshots, and
32 MiB per retained history snapshot. History tests enforce the configured
retention behavior, but these remain limits rather than performance promises.
Run `make xlsx-performance-verify` for the 25,000-cell CI regression profile.
Before a release, run the 400,000-cell release profile on the intended machine
and retain its report with the candidate evidence. The generated workload does
not replace feature-rich or producer-specific fixtures.

## Sign-Off Record

- [ ] X-01 through X-12 closed or explicitly restricted with a safe tested mode.
- [ ] Microsoft Excel fixture provenance and native reopen evidence recorded.
- [ ] No unexplained loss of unrelated workbook features.
- [ ] Claimed Claude and Codex modes certified against the candidate.
- [ ] Formula behavior and history/resource limits match product copy.
- [ ] No open source-loss, authorization, or false-success defect.
- [ ] Candidate checks pass and release documentation is consistent.

For each gate record: gate ID, candidate commit, date, reviewer, fixture hash,
command or manual procedure, expected result, observed result, evidence path,
and pass/fail/blocked status. A blocked native application check remains blocked;
a package parser reopen must not substitute for Excel verification.

## Implementation Order

1. Resolve X-03 through X-05 recovery and overwrite gaps with focused tests.
2. Complete Excel corpus coverage and formula policy checks.
3. Close candidate host, UI, access, and packaging gates.
4. Publish only the verified scope; schedule image insertion and advanced chart
   authoring as separate follow-ups with their own fidelity tests.
