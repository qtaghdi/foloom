# 폴룸 · Foloom

![폴룸](docs/brand/foloom-wordmark.svg)

**같은 문서에서, 함께.**

사람과 AI가 함께 일하는 문서 공간입니다. MCP 클라이언트와 브라우저에서 같은
문서를 확인하고 편집합니다. [브랜드 가이드와 로고 원본](docs/brand.md)을 제공합니다.

사람과 AI 에이전트가 같은 문서를 확인하고 수정할 수 있는 공유 편집 세션을
제공합니다. 리비전, 현재 위치 표시, 충돌 감지, 실행 취소, 안전한 저장을
지원합니다. XLSX가 첫 프로덕션 형식 어댑터이며, 편집 방식과 원본 보존 조건이
서로 다른 형식은 별도 어댑터로 개발합니다. DOCX는 생성과 본문·단순 표 텍스트
편집을 비공개 실험 기능으로 제공합니다. PPTX, HWPX, PDF는 계획 단계입니다.

상태: 비공개 로컬 XLSX 알파. Excel 실행 전 자동 안전성, 호환성, 패키징, 성능
검사를 통과합니다. 현재 소스와 바이너리를 공개 배포하지 않습니다. Microsoft
Excel 실사용 검증, 반복적인 외부 사용, 배포 조건, 소유자의 명시적 결정이 공개
릴리스 전 남은 조건입니다.

기존 document-workspace에서 전환하는 경우 [이름·설치 마이그레이션](docs/foloom-migration.md)을
확인하세요.

## 현재 지원 범위

현재 XLSX 어댑터는 다음을 제공합니다.

- Solid 기반 자체 앱 UI와 Univer OSS 브라우저 스프레드시트
- Streamable HTTP와 stdio MCP 전송
- 내장 MCP App과 인증된 loopback 대체 화면
- 통합 문서 메타데이터, 범위 읽기, 수식, 스타일, 셀 병합, 붙여넣기
- 행과 열 작업
- 순서가 보장되는 사람 및 AI 현재 위치 이벤트
- 리비전 충돌과 서버 기준의 영속적인 실행 취소 및 다시 실행
- 이미지와 차트 미리보기 이동, 크기 조절, 삭제, XLSX 반영
- 대용량 시트의 제한 기반 로딩
- 호환성 안내와 원자적 저장

앱 UI는 Solid + TypeScript + Vite로 구성하고 Go 서버에서 제공합니다. Univer 내부
React와 grid UI는 유지합니다. 프레임워크 선택과 경계는
[ADR 0003](docs/adr/0003-solid-editor-ui.md)을 참고하세요.

Univer는 오픈 소스 패키지만 사용하며, 빌드는 `@univerjs-pro/*` 의존성을
거부합니다.

DOCX는 새 문서 생성, 기존 본문·단순 표의 text run 편집, 원본 OOXML 보존과
인증된 브라우저 화면을 제공합니다. Word 페이지 배치와 구조 편집은 지원하지 않으며
native Word 검증은 남아 있습니다. [DOCX 사용 안내](docs/docx-editing.md)를 참고하세요.

## 저장소 구조

```text
apps/
  web-editor/                 TypeScript 브라우저 편집기
cmd/
  foloom/         Go 실행 파일
internal/
  formats/
    xlsx/                     XLSX 세션, 작업, 저장
    docx/                     DOCX 생성, 검사, 표적 텍스트 편집, 저장
  transport/
    httpapi/                  HTTP, SSE, MCP, 내장 UI 어댑터
testdata/
  xlsx/compatibility/         XLSX 검증 자료와 출처
legacy/
  xlsx-python/                보존된 Python 개념 증명
docs/                         아키텍처, 계획, 결정
```

형식 로직은 `internal/formats/<format>`에 둡니다. 전송 계층과 UI 패키지가 형식
저장 규칙을 소유하면 안 됩니다. 공통 문서 추상화는 두 번째 실제 어댑터가 공통
계약을 입증한 뒤에만 도입합니다.

## 비공개 XLSX 알파 실행

관리자는 macOS, Linux, Windows용 비공개 알파 압축 파일을 Intel 및 ARM64로
빌드할 수 있습니다. 각 결과물에는 실행 파일, 사용자 문서, SHA-256 체크섬
목록이 포함됩니다. 비공개 알파 단계에는 GitHub Release나 공개 다운로드를
제공하지 않습니다.

압축을 푼 뒤 서버를 시작하지 않고 복사본 통합 문서를 검증합니다.

```bash
./foloom -check /absolute/path/to/workbook.xlsx
```

그다음 로컬 브라우저 편집기를 시작합니다.

```bash
./foloom /absolute/path/to/workbook.xlsx
```

결과물 선택, 체크섬 검증, 서명되지 않은 앱 경고, 로컬 Claude Desktop 및 Codex
설정은 [비공개 XLSX 알파 안내](docs/getting-started.md)를 참고하세요.

macOS에서는 압축을 푼 실행 파일로 MCP와 `foloom` 스킬을 함께 설치할 수
있습니다.

```bash
./foloom mcp install
```

Claude Desktop과 Codex를 다시 시작한 뒤 `foloom` 스킬에 XLSX 또는
DOCX 절대 경로와 작업을 전달합니다. Codex에서는 `$foloom`로 호출할
수 있습니다. 기존 XLSX용 `document-xlsx` MCP 프롬프트도 사용할 수 있습니다.

## 소스에서 빌드

```bash
make web-install
make dev FILE=/absolute/path/to/workbook.xlsx
```

프로세스는 시작 시 브라우저와 MCP 인증 정보를 각각 출력합니다. 해당 터미널
출력은 민감 정보로 취급하고 표시된 브라우저 URL을 그대로 여세요. MCP
클라이언트는 별도로 출력된 bearer token으로 `http://127.0.0.1:8765/mcp`에
연결합니다. 검증된 경계와 호스팅 모드 제외 사항은
[로컬 보안 모델](docs/security.md)을 참고하세요.

stdio를 사용하는 로컬 데스크톱 호스트:

```bash
make build
./bin/foloom -transport stdio
```

기존처럼 `-file /absolute/path/to/workbook.xlsx`를 지정하는 방식도 유지됩니다.

호스트가 MCP App을 표시하지 못하면 `open_workbook`은 2분 동안 한 번만 사용할
수 있는 `browserUrl`을 반환합니다. `-stdio-browser=false`로 이 대체 동작을
끌 수 있습니다.

깨끗한 checkout에서 릴리스 방식으로 빌드하려면 `make package-verify`를
실행합니다. 고정된 pnpm lockfile을 설치하고 두 UI 번들을 빌드 및 검증한 뒤,
실행 파일을 컴파일하고 절대 경로 smoke test를 수행합니다. 결과물 경계는
[패키징 검증](docs/packaging.md)을 참고하세요.

현재 플랫폼은 `make release-package-test`, 전체 압축 파일 조합은
`make release-verify VERSION=0.1.0-alpha.local`로 검증할 수 있습니다.

플랫폼 전환 중에도 다음 통합 문서 도구 계약은 유지됩니다.

- `open_workbook`
- `get_workbook`
- `read_range`
- `apply_operations`
- `restore_history`
- `update_presence`

내장 앱은 앱 전용 이벤트, 객체, 사용자 작업, 이력, 현재 위치 도구도 사용합니다.

## 여러 문서 열기

브라우저의 작업 공간에서 XLSX와 DOCX 절대 경로를 입력하면 최대 8개 문서를
탭으로 오갈 수 있습니다. 여러 경로는 한 줄에 하나씩 입력합니다. 전환해도
각 문서의 초안, 선택 위치, 스크롤과 저장 상태를 유지합니다. 처음 선택한
탭의 편집기만 불러옵니다. 같은 파일은 기존 탭으로 이동합니다.

DOCX 초안이 있는 탭은 저장 후 닫기, 입력 버리기, 취소를 선택합니다.
저장 중인 파일은 작업이 끝난 뒤 닫습니다. 최근 10개 경로는 해당 브라우저에만
남으며 최근 파일 메뉴에서 지울 수 있습니다. 앱 종료 후 미저장 입력의 복구는
지원하지 않습니다.

MCP에서 여러 XLSX가 열리면 저장과 이력 복원에 `documentId`를 명시합니다.
`open_workbook` 또는 `get_workbook`의 `id`를 사용합니다. DOCX 쓰기는 기존
`sessionId`로 해당 열린 문서를 선택합니다. 브라우저 탭 전환은 AI의 기본 대상을
바꾸지 않습니다. 문서 간 비교와 일괄 저장은 포함하지 않습니다.

## 언어 설정

작업 공간에서 한국어, English, 日本語, 简体中文을 선택할 수 있습니다.
처음에는 브라우저의 지원 언어를 사용하며 지원하지 않는 언어는 영어로 표시합니다.
선택한 언어는 해당 브라우저에 저장됩니다. 언어 변경은 열린 문서의 초안이나
편집 위치를 바꾸지 않습니다. 편집 툴바, 문서 내용, 파일명, 원문 진단은 번역하지
않습니다. 알려진 오류와 경고는 선택한 언어로 안내하고 서버 진단은 필요할 때
펼쳐 볼 수 있습니다.

## 개발

```bash
make web-install
make compatibility-test
make verify
make build
```

JavaScript 의존성은 pnpm 11.9와 정확한 버전을 공유하는 Univer catalog를
사용합니다. 프론트엔드는 TypeScript, Vite, Tailwind CSS, Univer로 구성됩니다.
프로덕션 출력은 `internal/transport/httpapi/static`에서 내장됩니다. 생성된
assets와 독립 실행형 `app.html`은 커밋하지 않습니다.

업그레이드 뒤에도 로컬 실행 취소 이력을 유지하기 위해 XLSX 이력 저장소는 기존
`.xlsx-viewer-history` 디스크 namespace를 사용합니다. 이는 호환성을 위한 구현
세부 사항이며 제품 이름이 아닙니다.

## 문서

- [공통 제품 명세](docs/product-spec.md)
- [비공개 XLSX 알파 시작 안내](docs/getting-started.md)
- [형식별 지원표](docs/format-support.md)
- [XLSX 안정 릴리스 체크리스트](docs/xlsx-release-checklist.md)
- [XLSX 복구 절차](docs/xlsx-recovery.md)
- [XLSX 수식 정책](docs/xlsx-formula-policy.md)
- [DOCX 원본 보존 프로토타입](docs/docx-preservation-prototype.md)
- [제품 계획](docs/product-plan.md)
- [아키텍처](docs/architecture.md)
- [호스트 호환성](docs/host-compatibility.md)
- [스프레드시트 엔진 결정](docs/adr/0001-spreadsheet-engine.md)
- [기여 가이드](CONTRIBUTING.md)
- [변경 이력](CHANGELOG.md)
- [AI 엔지니어링 가이드](AGENTS.md)
