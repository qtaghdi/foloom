# XLSX Pre-Excel Candidate Evidence

This record covers every release-candidate check that can run without native
Microsoft Excel or a visible desktop host session. It does not certify
Microsoft Excel fidelity, Excel formula calculation, or native MCP App
rendering.

## Environment

- Candidate commit: `88965f1d734dffe4a9161805a4ee56bcba5687f3`
- Working tree dirty at start: `no`
- Recorded at: `2026-10-13T05:33:57Z`
- Operating system: `Darwin 25.5.0 arm64`
- Go: `go version go1.25.0 darwin/arm64`
- Node.js: `v24.14.0`
- pnpm: `11.9.0`
- pnpm lockfile SHA-256:
  `c2d194cbbb9215f029c1ef53191ed64762fe64006b74d02b2749b663cb1c45a0`

## Fixture Hashes

- `testdata/xlsx/compatibility/feature-rich.xlsx`:
  `fb8576593415b76eeea6ff4fd33c9b0739cf9a79c8448ef27b2d54033d951e45`
- `testdata/xlsx/compatibility/libreoffice.xlsx`:
  `2b253b8197b7ba53832a141dd4946aef24130e43f8fc05589365e13478cada32`

## Results

| Check | Gates | Focus | Result | Duration |
| --- | --- | --- | --- | ---: |
| safety | X-01 to X-06 | Focused safety and recovery gates | Pass | 15s |
| compatibility | X-07 automated | Generated and LibreOffice corpus round trip | Pass | 1s |
| verification | X-01 to X-11 automated | Frontend, Go, race, vet, and legacy regression suite | Pass | 53s |
| packaging | X-12 automated | Frozen install, embedded assets, binary, and spaced-path smoke test | Pass | 76s |
| performance | Resource guardrail | 400,000-cell release performance profile | Pass | 2s |

Overall status: **Pass**

## Detailed Evidence

- [Focused safety evidence](xlsx-safety-2026-10-13.md), SHA-256:
  `657defbbbb50fefba8b8a228c9d489f6ab79ca450f54cad1b0b3f32b84677dd9`
- [Release performance evidence](xlsx-performance-2026-10-13.md), SHA-256:
  `e31ec4c1bbe7d68c6ffda5173f5b249115d2b4214382a33b88ed6cce47b7e769`

## Commands

- **safety:** `make xlsx-safety-verify REPORT=<temporary-evidence-dir>/xlsx-safety.md`
- **compatibility:** `make compatibility-test`
- **verification:** `make verify`
- **packaging:** `make package-verify`
- **performance:** `make xlsx-performance-verify XLSX_PERFORMANCE_PROFILE=release REPORT=<temporary-evidence-dir>/xlsx-performance-release.md`

## Remaining Manual Boundaries

- X-07 requires a provenance-recorded Microsoft Excel fixture and native reopen
  checks for every supported edit family.
- X-08 requires observed Microsoft Excel cache and recalculation behavior for
  changed formulas, dependencies, unsupported functions, external references,
  and stale caches.
- X-09 requires candidate-level visible Claude and Codex host runs for any newly
  claimed host mode. Browser fallback and embedded MCP App rendering remain
  separate claims.
- X-10 requires one visible-browser grid interaction repeat against the final
  candidate. Automated event-boundary and responsive-shell evidence passed.
- X-12 still needs any platform artifact matrix claimed beyond Darwin arm64 and
  the Ubuntu amd64 CI environment used by the linked pull request.
- Power-loss durability, network filesystems, the final non-cooperating writer
  check-to-rename race, and durable per-operation receipts are not certified.
