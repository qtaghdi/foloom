# XLSX 성능 증거

이 budget은 회귀 guardrail이며 사용자 latency 약속이 아닙니다. 생성 통합 문서는
임시 파일이며 사용자 데이터를 포함하지 않습니다.

## 환경

- Candidate commit: `88965f1d734dffe4a9161805a4ee56bcba5687f3`
- 시작 시 working tree dirty: `false`
- 기록 시각: `2026-10-13T05:33:55Z`
- 운영체제와 architecture: `darwin/arm64`
- Logical CPU: `14`
- Go: `go1.25.0`
- Excelize: `v2.11.0`

## Workload

- Profile: `release`
- 채운 셀: `4`개 sheet에 `400000`
- Sheet별 shape: `1000` data row x `100` column과 header 1줄
- 수식 간격: 마지막 column의 매 `25` row
- 제한 읽기: `10000`셀
- Batch commit: `1000`셀
- 생성 파일 크기: `1264123` bytes
- 생성 파일 SHA-256: `deacf85b26b0579e6eace7c7bd08cd8087f7343eb6924d274144c51a89070199`
- latency budget에서 제외한 생성 시간: `1003.66 ms`

## Budget

- 열기: `10s`
- 10,000셀 읽기: `2s`
- 1셀 commit: `15s`
- 1,000셀 commit: `20s`
- 작업별 최대 관찰 Go heap: `1024.00 MiB`

## 결과

| 작업 | 시간 | 제한 | 작업 중 allocation | 최대 관찰 Go heap | 상태 | 상세 |
| --- | ---: | ---: | ---: | ---: | --- | --- |
| Session 열기 | 846.91 ms | 10s | 922.33 MiB | 246.82 MiB | Pass | budget 이내 |
| 10,000셀 읽기 | 8.99 ms | 2s | 1.31 MiB | 159.84 MiB | Pass | budget 이내 |
| 1셀 commit | 329.38 ms | 15s | 197.73 MiB | 312.64 MiB | Pass | budget 이내 |
| 1,000셀 commit | 308.04 ms | 20s | 198.71 MiB | 119.19 MiB | Pass | budget 이내 |

전체 상태: **Pass**

## 해석 경계

- heap sampler는 1ms마다 Go `HeapAlloc`을 관찰하며 OS peak RSS가 아닙니다.
- 합성 파일은 모든 chart, image, style, 수식, shared-string 분포를 대표하지 않습니다.
- 생성 시간은 open 및 edit latency에 포함하지 않습니다.
- shared CI runner 편차가 있을 수 있습니다. 실패 시 증거 없이 budget을 올리지 않고
  원인을 조사합니다.
- 성능 주장 전 release machine에서 release profile을 실행하고 보고서를 보관합니다.
