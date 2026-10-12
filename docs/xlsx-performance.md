# XLSX Performance Verification

## Purpose

The performance gate detects severe XLSX adapter regressions before release.
Its budgets are engineering guardrails, not promises of end-user latency. Real
workbooks vary by compression, shared strings, formulas, styles, drawings,
conditional rules, storage speed, and machine resources.

The benchmark creates a non-sensitive workbook in a temporary directory, opens
it through the real `xlsx.Session` boundary, reads the maximum supported bounded
range, commits one cell, and commits a 1,000-cell rectangular paste. It deletes
the generated workbook after the run.

## Profiles and Budgets

| Profile | Populated cells | Sheets and shape | Open | Read 10,000 cells | Commit one cell | Commit 1,000 cells | Peak observed Go heap |
| --- | ---: | --- | ---: | ---: | ---: | ---: | ---: |
| `ci` | 25,000 | 1 sheet, 250 by 100 | 3 s | 1 s | 5 s | 8 s | 512 MiB |
| `release` | 400,000 | 4 sheets, 1,000 by 100 | 10 s | 2 s | 15 s | 20 s | 1,024 MiB |

Each sheet also has one header row and formulas every 25 rows in its final
column. Workbook generation is measured and reported separately. It is excluded
from open and editing budgets because users supply an existing workbook.

The CI profile runs on every pull request and main push as
`Verify XLSX performance budget`. The release profile is intentionally not run
on every shared CI runner. Run it on the intended release machine and retain the
report before making performance claims.

## Commands

Run the required CI profile:

```bash
make xlsx-performance-verify
```

Retain a Markdown report outside the repository:

```bash
make xlsx-performance-verify REPORT=/absolute/path/to/xlsx-performance-ci.md
```

Run the larger release profile:

```bash
make xlsx-performance-verify XLSX_PERFORMANCE_PROFILE=release REPORT=/absolute/path/to/xlsx-performance-release.md
```

The report records the candidate commit, dirty state, timestamp, operating
system, architecture, CPU count, Go and Excelize versions, workbook shape, file
size, SHA-256 hash, action latency, allocated bytes, observed heap, budgets, and
status. XLSX ZIP metadata may make hashes differ between generations. The hash
identifies the exact input for one evidence record; it is not a golden hash.

## Memory Boundary

The sampler reads Go `HeapAlloc` every millisecond while each measured action is
running. It also records total bytes allocated during the action. This catches
large Go allocation regressions, but it is not the operating system's peak RSS
and does not include every native or mapped allocation. Record an external RSS
measurement separately if a release needs an operating-system memory claim.

## Failure Policy

A budget failure blocks the performance gate. First reproduce the same profile
on a comparable machine and inspect workbook size, action allocation, garbage
collection, storage, and runner contention. Do not increase a budget merely to
make CI green. A budget change requires an issue, before-and-after evidence, and
an explanation of the accepted user impact.

The benchmark does not replace the compatibility corpus. Run
`make compatibility-test` after XLSX behavior changes because a fast save that
loses unrelated workbook features is a failure.

## Known Limits

- The generated workload is regular tabular data with periodic formulas. It is
  not a large chart, image, style, or shared-string stress corpus.
- Browser rendering, scrolling frame rate, MCP transport latency, concurrent
  users, network storage, and hosted load are outside this benchmark.
- The 100 MiB input limit is a security ceiling, not a demonstrated performance
  target.
- A passing synthetic profile does not certify a private or customer workbook.
