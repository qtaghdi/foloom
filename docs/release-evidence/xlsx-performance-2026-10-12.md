# XLSX Performance Evidence

These budgets are regression guardrails, not end-user latency promises.
Generated workbooks are temporary and contain no user data.

## Environment

- Candidate commit: `54a08bce7fc59fda499fa74261738ce109262b72`
- Working tree dirty at start: `false`
- Recorded at: `2026-10-12T07:41:52Z`
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
  `0db4f0fe57b37e7e7352490a1f8c2d210dec85293d3f47d6ca2a1340e89dfa39`
- Generation time, excluded from latency budgets: `852.22 ms`

## Budgets

- Open: `10s`
- Read 10,000 cells: `2s`
- Commit one cell: `15s`
- Commit 1,000 cells: `20s`
- Peak observed Go heap for each measured action: `1024.00 MiB`

## Results

| Action | Duration | Limit | Allocated during action | Peak observed Go heap | Status | Details |
| --- | ---: | ---: | ---: | ---: | --- | --- |
| Open session | 757.79 ms | 10s | 922.32 MiB | 248.73 MiB | Pass | Within budget |
| Read 10,000 cells | 7.81 ms | 2s | 1.31 MiB | 159.84 MiB | Pass | Within budget |
| Commit one cell | 336.87 ms | 15s | 197.73 MiB | 311.26 MiB | Pass | Within budget |
| Commit 1,000 cells | 287.74 ms | 20s | 198.71 MiB | 119.69 MiB | Pass | Within budget |

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
