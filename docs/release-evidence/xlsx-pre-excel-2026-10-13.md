# XLSX Excel 실행 전 Candidate 증거

이 기록은 실제 Microsoft Excel이나 보이는 desktop host session 없이 실행할 수
있는 모든 release-candidate 검사를 포함합니다. Microsoft Excel fidelity, Excel
수식 계산, native MCP App rendering을 인증하지 않습니다.

## 환경

- Candidate commit: `88965f1d734dffe4a9161805a4ee56bcba5687f3`
- 시작 시 working tree dirty: `no`
- 기록 시각: `2026-10-13T05:33:57Z`
- 운영체제: `Darwin 25.5.0 arm64`
- Go: `go version go1.25.0 darwin/arm64`
- Node.js: `v24.14.0`
- pnpm: `11.9.0`
- pnpm lockfile SHA-256: `c2d194cbbb9215f029c1ef53191ed64762fe64006b74d02b2749b663cb1c45a0`

## Fixture hash

- `feature-rich.xlsx`: `fb8576593415b76eeea6ff4fd33c9b0739cf9a79c8448ef27b2d54033d951e45`
- `libreoffice.xlsx`: `2b253b8197b7ba53832a141dd4946aef24130e43f8fc05589365e13478cada32`

## 결과

| 검사 | Gate | 초점 | 결과 | 시간 |
| --- | --- | --- | --- | ---: |
| safety | X-01부터 X-06 | 집중 safety와 recovery | Pass | 15s |
| compatibility | X-07 자동 | 생성 및 LibreOffice corpus round-trip | Pass | 1s |
| verification | X-01부터 X-11 자동 | Frontend, Go, race, vet, legacy 회귀 | Pass | 53s |
| packaging | X-12 자동 | Frozen 설치, embedded asset, binary, 공백 경로 smoke test | Pass | 76s |
| performance | resource guardrail | 400,000셀 release profile | Pass | 2s |

전체 상태: **Pass**

## 상세 증거

- [집중 safety 증거](xlsx-safety-2026-10-13.md), SHA-256:
  `657defbbbb50fefba8b8a228c9d489f6ab79ca450f54cad1b0b3f32b84677dd9`
- [release 성능 증거](xlsx-performance-2026-10-13.md), SHA-256:
  `e31ec4c1bbe7d68c6ffda5173f5b249115d2b4214382a33b88ed6cce47b7e769`

## 명령

- **safety:** `make xlsx-safety-verify REPORT=<temporary-evidence-dir>/xlsx-safety.md`
- **compatibility:** `make compatibility-test`
- **verification:** `make verify`
- **packaging:** `make package-verify`
- **performance:** `make xlsx-performance-verify XLSX_PERFORMANCE_PROFILE=release REPORT=<temporary-evidence-dir>/xlsx-performance-release.md`

## 남은 수동 경계

- X-07: 출처가 기록된 Microsoft Excel fixture와 모든 지원 편집의 native reopen
- X-08: 변경 수식, 의존성, 미지원 함수, 외부 reference, stale cache의 실제 Excel
  cache 및 재계산 동작
- X-09: 새로 주장할 모드의 보이는 Claude 및 Codex candidate 실행. browser
  fallback과 embedded MCP App은 별도 주장
- X-10: 최종 candidate의 보이는 browser grid 상호작용 1회 반복
- X-12: Darwin arm64와 연결 PR의 Ubuntu amd64 CI 밖에서 주장할 platform matrix
- 미인증: 전원 손실 내구성, network filesystem, 비협조 writer의 마지막 race,
  프로세스를 넘는 작업별 영속 receipt
