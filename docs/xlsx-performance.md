# XLSX 성능 검증

## 목적

성능 gate는 release 전 심각한 XLSX adapter 회귀를 찾습니다. budget은 엔지니어링
guardrail이며 사용자 latency 약속이 아닙니다. 실제 통합 문서는 압축, shared
string, 수식, style, drawing, conditional rule, storage, machine에 따라 다릅니다.

benchmark는 임시 디렉터리에 비민감 통합 문서를 만들고 실제 `xlsx.Session`으로
열어 최대 지원 범위 10,000셀을 읽습니다. 셀 하나와 1,000셀 사각 범위를 각각
commit한 뒤 생성 파일을 삭제합니다.

## Profile과 budget

| Profile | 채운 셀 | 구조 | 열기 | 10,000셀 읽기 | 1셀 commit | 1,000셀 commit | 최대 Go heap |
| --- | ---: | --- | ---: | ---: | ---: | ---: | ---: |
| `ci` | 25,000 | 1 sheet, 250 x 100 | 3초 | 1초 | 5초 | 8초 | 512 MiB |
| `release` | 400,000 | 4 sheets, 1,000 x 100 | 10초 | 2초 | 15초 | 20초 | 1,024 MiB |

각 sheet에는 header 한 줄과 마지막 열의 25행마다 수식이 있습니다. 통합 문서
생성 시간은 별도로 보고하고 사용자는 기존 파일을 제공하므로 open 및 edit
budget에서는 제외합니다.

`ci`는 모든 Pull Request와 `main` push에서 `Verify XLSX performance budget`으로
실행합니다. `release`는 intended release machine에서 수동 실행하고 성능 주장을
하기 전에 보고서를 보관합니다. 첫 clean baseline은
[성능 증거](release-evidence/xlsx-performance-2026-10-12.md)에 있습니다.

## 명령

```bash
make xlsx-performance-verify
make xlsx-performance-verify REPORT=/absolute/path/to/xlsx-performance-ci.md
make xlsx-performance-verify XLSX_PERFORMANCE_PROFILE=release REPORT=/absolute/path/to/xlsx-performance-release.md
```

보고서는 candidate commit, dirty 상태, 시간, OS, architecture, CPU 수, Go와
Excelize 버전, workbook shape, file size, SHA-256, latency, allocated bytes,
observed heap, budget, 상태를 기록합니다. ZIP metadata 때문에 생성할 때마다 hash가
달라질 수 있으며 hash는 그 증거의 정확한 입력만 식별합니다.

## Memory 경계

sampler는 각 작업 중 매 millisecond마다 Go `HeapAlloc`을 읽고 total allocated
bytes도 기록합니다. 큰 Go allocation 회귀는 잡지만 OS peak RSS나 모든 native 및
mapped allocation을 포함하지 않습니다. OS memory 주장이 필요하면 RSS를 따로
측정합니다.

## 실패 정책

budget 실패는 gate를 차단합니다. 비슷한 machine에서 같은 profile을 재현하고 file
size, allocation, garbage collection, storage, runner contention을 확인합니다. CI를
통과시키기 위해 budget만 올리지 않습니다. 변경에는 이슈, 전후 증거, 사용자 영향
설명이 필요합니다.

성능 benchmark는 compatibility corpus를 대신하지 않습니다. 빠르지만 관련 없는
기능을 잃는 저장도 실패입니다.

## 알려진 한계

- workload는 주기적 수식이 있는 규칙적인 table이며 chart, image, 많은 style,
  shared string stress corpus가 아닙니다.
- browser rendering, scroll frame rate, MCP latency, 동시 사용자, network storage,
  hosted load는 제외합니다.
- 입력 100 MiB는 보안 상한이며 입증된 성능 목표가 아닙니다.
- 합성 profile 통과가 비공개 또는 고객 통합 문서를 인증하지 않습니다.
