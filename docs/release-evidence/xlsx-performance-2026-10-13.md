# XLSX Performance Evidence

These budgets are regression guardrails, not end-user latency promises.
Generated workbooks are temporary and contain no user data.

## Environment

- Candidate commit: `88965f1d734dffe4a9161805a4ee56bcba5687f3`
- Working tree dirty at start: `false`
- Recorded at: `2026-10-13T05:33:55Z`
- Operating system and architecture: `darwin/arm64`
- Logical CPUs: `14`
- Go: `go1.25.0`
- Excelize: `v2.11.0`

## Workload

- Profile: `release`
- Populated cells: `400000` across `4` sheets
- Shape per sheet: `1000` data rows by `100` columns, plus one header row
- Formula interval: every `25` rows in the final column
- Bounded read: `10000` cells
- Batch commit: `1000` cells
- Generated workbook size: `1264123` bytes
- Generated workbook SHA-256:
  `deacf85b26b0579e6eace7c7bd08cd8087f7343eb6924d274144c51a89070199`
- Generation time, excluded from latency budgets: `1003.66 ms`

## Budgets

- Open: `10s`
- Read 10,000 cells: `2s`
- Commit one cell: `15s`
- Commit 1,000 cells: `20s`
- Peak observed Go heap for each measured action: `1024.00 MiB`

## Results

| Action | Duration | Limit | Allocated during action | Peak observed Go heap | Status | Details |
| --- | ---: | ---: | ---: | ---: | --- | --- |
| Open session | 846.91 ms | 10s | 922.33 MiB | 246.82 MiB | Pass | Within budget |
| Read 10,000 cells | 8.99 ms | 2s | 1.31 MiB | 159.84 MiB | Pass | Within budget |
| Commit one cell | 329.38 ms | 15s | 197.73 MiB | 312.64 MiB | Pass | Within budget |
| Commit 1,000 cells | 308.04 ms | 20s | 198.71 MiB | 119.19 MiB | Pass | Within budget |

Overall status: **Pass**

## Interpretation Boundaries

- The heap sampler observes Go `HeapAlloc` at one-millisecond intervals. It is
  not operating-system peak RSS.
- The workbook is synthetic and does not model every chart, image, style,
  formula, or shared-string distribution.
- Generation time is reported separately and is not part of open or edit
  latency.
- Shared CI runner variance is expected. A failure blocks the gate and requires
  investigation, not a budget increase without evidence.
- Run the release profile on the release machine and retain its report before
  making performance claims.
