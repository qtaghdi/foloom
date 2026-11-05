# 아키텍처

## 목표

document-workspace는 사람과 AI가 같은 사무 문서를 안전하게 편집하는 로컬 우선
플랫폼입니다. 형식별 fidelity는 adapter가 소유하고, 전송 계층은 형식 규칙을
모르며, browser는 표시와 초안만 담당합니다. 서버의 리비전과 저장 상태가 항상
기준입니다.

## 저장소 구조

```text
apps/web-editor/                 TypeScript 브라우저 편집기
cmd/document-workspace/         Go 실행 파일
internal/formats/xlsx/          XLSX 세션, 작업, 저장, 이력
internal/formats/docx/          실험 DOCX 생성, 표적 편집, 저장
internal/transport/httpapi/     HTTP, SSE, MCP, 내장 UI
testdata/xlsx/compatibility/    출처가 기록된 호환성 corpus
legacy/xlsx-python/             보존된 Python 개념 증명
docs/                           제품 및 운영 문서
```

형식 로직은 `internal/formats/<format>`에 둡니다. 전송이나 UI가 OOXML 저장 규칙을
소유하면 안 됩니다. 두 번째 실제 adapter가 공통 계약을 입증하기 전에는 추상적인
범용 문서 runtime을 만들지 않습니다.

## 시스템 흐름

```text
Claude/Codex MCP host ─┐
                      ├─ HTTP 또는 stdio MCP ─┐
브라우저/MCP App ─────┘                       │
                                              ▼
                                  Go 전송 및 인증 계층
                                              │
                                  typed 작업, 리비전, event
                                              │
                                              ▼
                                      XLSX format session
                                              │
                             candidate 검증, 원자적 교체, history
                                              │
                                              ▼
                                         로컬 .xlsx
```

### Go 실행 파일

`cmd/document-workspace`는 입력 파일을 열고 HTTP 또는 stdio 전송을 시작합니다.
stdio는 파일 없이 시작한 뒤 `open_workbook`으로 활성 XLSX를 정할 수 있습니다.
thread-safe session holder가 활성 세션의 교체와 종료를 직렬화하며 진행 중인 도구
호출이 닫힌 세션을 사용하지 않게 합니다. HTTP는 시작 시 XLSX 또는 DOCX 파일이 필요하며 CLI는 확장자로 adapter를 선택합니다.
`-check`는 같은 adapter 경계에서 사전 검사만 수행합니다. `mcp install`, `status`,
`uninstall`은 macOS host 설정과 사용자 skill을 관리합니다. 릴리스 정보는 build 시
version, commit, date로 주입합니다.

### 형식 adapter

현재 `internal/formats/xlsx`가 유일한 production adapter입니다. Excelize package,
session, typed operation, 리비전, event, persistence, history, compatibility notice를
소유합니다. `internal/formats/docx`는 비공개 실험 adapter로 paragraph·단순 table 생성과
일반 run text 교체와 제한된 문단 구조 계획을 제공합니다. namespace-aware 검사와 원본 byte offset 표적
수정으로 주변 XML과 다른 package part를 보존합니다. DOCX는 별도 구체 session을
사용하고 shared runtime은 추출하지 않습니다. PPTX, HWPX, PDF는 계획입니다.

### 전송 계층

`internal/transport/httpapi`는 인증, request 제한, HTTP API, SSE, MCP 도구,
MCP App resource, loopback 브라우저 대체 화면을 제공합니다. workbook 규칙을 직접
구현하지 않습니다.

현재 안정 MCP 도구:

- `open_workbook`
- `get_workbook`
- `search_workbook`
- `read_range`
- `get_sheet_objects`
- `apply_operations`
- `restore_history`
- `update_presence`

`open_workbook`은 선택적인 절대 `.xlsx` 경로를 받아 활성 세션을 만들거나
전환하고, 사람이 직접 편집할 인증된 browser URL을 반환합니다. MCP host는 작은
inline preview 대신 이 URL을 browser surface에 엽니다. 다른 도구는 문서가 열리기
전 명확한 오류를 반환합니다. MCP `document-xlsx` prompt와 host별 skill은 전체
workbook 대신 metadata와 필요한 범위만 읽는 workflow를 제공합니다.

DOCX는 `create_docx`, `open_docx`, `get_docx`, `apply_docx_operations` 전용 도구,
`/api/docx`와 `/api/docx/operations`, `/api/docx/events`, `/docx` 브라우저를
사용합니다. DOCX event는 별도 bounded replay와 sequence·revision을 가지며
SSE 전송은 1초 간격으로 새로운 commit을 확인합니다. 문서 교체나 gap은 reload를
요청하고 local draft가 있으면 복구 전 저장을 차단합니다.

다른 형식은 공통 계약이 증명되기 전까지 형식별 typed 도구를 가질 수 있습니다.
MCP App resource는 호환성을 위해 `ui://document-workspace/xlsx/v1.html`로
유지하지만 `open_workbook`의 자동 output template으로 연결하지 않습니다. HTTP와
stdio가 같은 서버를 사용하며, stdio는 일회용 실행 token의 인증된 loopback
browser editor를 기본 상호작용 surface로 제공합니다.

### 브라우저 편집기

`apps/web-editor`는 Solid, TypeScript, Vite, Tailwind CSS, Univer OSS를 사용합니다.
Solid 컴포넌트가 앱 소유 shell·상태·panel과 DOCX 입력을 렌더링하고 view adapter가
controller의 표시 상태를 signal로 전달합니다. grid container 내부는 Univer가
소유하며 상태 변경으로 engine을 다시 만들지 않습니다. root 해제 시 engine·구독·
window listener를 정리하고 지연된 초기 읽기가 해제된 engine을 만들지 못하게 합니다.
선택과 경계는 [ADR 0003](adr/0003-solid-editor-ui.md)을 따릅니다.
`SpreadsheetEngine`이 Univer type을 격리하고 `WorkbookController`가 리비전,
직렬화된 쓰기, presence throttle, conflict, 구독 lifecycle을 소유합니다. HTTP와
MCP App client는 같은 `WorkbookClient` 계약을 구현하고 Zod가 입력 payload를
검증합니다.

Univer는 grid keyboard와 clipboard를 담당합니다. controller는 확정된 셀 편집,
사각 붙여넣기, undo 및 redo를 서버 기준 작업으로 바꿉니다. UI는 대기, 저장,
실패, 충돌, 복구를 live region에 표시하고 focus, reduced motion, safe area, 작은
화면 대체 layout을 유지합니다. compatibility notice와 일반 오류는 접거나 닫을 수
있고, 편집이 중단된 복구 Alert는 reload action을 유지한 채 접을 수 있습니다.

### 호환성 corpus

`testdata/<format>`에는 공개, 생성, 또는 출처가 기록된 fixture만 둡니다. XLSX
테스트는 복사본을 열고 관련 없는 기능을 inventory한 뒤 실제 `xlsx.Session`으로
편집, 저장, reopen하여 보존을 비교합니다. 현재 생성기와 LibreOffice fixture가
있고 Microsoft Excel fixture는 남은 release gate입니다.

`legacy/xlsx-python`은 Go 테스트가 동작을 완전히 대체할 때까지 초기 Python
prototype과 회귀 테스트를 보존합니다.

## XLSX 세션 모델

세션은 안정 ID, 현재 리비전, 순서 있는 event sequence, subscriber, 메모리의
Excelize file을 가집니다. 쓰기는 기준 리비전을 포함합니다. 전체 batch를 먼저
검증하고 candidate에 적용하여 원자적으로 저장한 뒤 리비전을 한 번 증가시키고
event를 보냅니다.

최근 리비전 snapshot 10개를 유지합니다. 32 MiB를 넘는 snapshot은 보관하지
않지만 문서 commit은 성공할 수 있습니다. 이력 event는 undo와 redo 가능 상태를
포함합니다. `.xlsx-viewer-history` 이름은 업그레이드 호환성을 위해 유지합니다.

큰 sheet는 정렬된 제한 범위 tile로 읽습니다. 쓰기는 candidate workbook에서
수행하고 저장이 성공할 때까지 기준 메모리 상태를 유지합니다. 준비, write, sync,
close, replacement가 실패하면 이전 리비전과 상태를 유지합니다. 문서 저장 뒤
history manifest 확정만 실패하면 경고와 pending manifest를 남기고 다음 쓰기 전에
조정합니다.

sidecar `write.lock`은 협조 writer를 직렬화합니다. 변경 전과 replacement 직전에
source hash를 검사하고 삭제되거나 non-regular인 원본을 거부합니다. symlink는
열 때 해석합니다. Excel, hard link, network filesystem 같은 비협조 writer의
마지막 check-to-rename race는 제거하지 못합니다.

## XLSX 호환성 경계

일반 validation과 conditional formatting은 Univer OSS API에 mapping합니다. 미지원
고급 규칙은 package에 남기고 가능한 경우 안내합니다. PNG, JPEG, GIF와 지원 차트는
drawing 또는 SVG preview로 표시합니다. 기존 image와 chart의 이동, 크기, 삭제,
chart title 변경을 저장합니다. 이미지 삽입과 chart type, series, axis, legend,
style 편집은 미구현입니다.

관련 없는 편집을 저장할 때 data validation의 `showDropDown`, `showInputMessage`,
`showErrorMessage`는 worksheet namespace, 범위, 규칙, 수식이 같은 노드에 한해 원본의
명시 여부와 값을 복원합니다. 노드를 안전하게 대응시킬 수 없으면 다른 validation에
속성을 붙이지 않고 저장을 거부합니다. XML attribute 순서, quote 형태, ZIP 압축과
timestamp까지 byte 단위로 보존하는 계약은 아닙니다.

서비스는 수식을 저장하지만 계산하지 않습니다. cached source value와 unavailable
value를 구분하고 Univer 결과는 browser preview로 취급합니다. 수식에 영향을 주는
편집은 native 전체 재계산을 요청합니다.

## 실시간 event와 복구

현재 server-to-browser event는 SSE를 사용합니다. event에는 sequence, revision,
actor, type, 형식별 위치, 작업 data가 있습니다. 제한 replay window와
`Last-Event-ID` 재연결을 지원합니다. gap이나 잘못된 future cursor는
`workbook.reload`와 `state: resync`를 반환합니다. 느린 subscriber는 조용히 event를
잃는 대신 연결을 끊습니다.

쓰기 실패, 연결 중단, resync 뒤 controller는 queue를 멈추고 작업을 재시도하지
않은 채 server snapshot을 확인합니다. 초안을 보여주고 사용자가 명시적으로 다시
불러오게 합니다. 높은 server revision만 보고 특정 쓰기가 성공했다고 판단하지
않습니다. typing animation과 cursor는 presentation 상태이며 영속 쓰기는 작업 또는
batch 단위입니다.

## 보안 경계

문서, 수식, name, value, MCP 및 browser 입력은 신뢰하지 않습니다. 로컬 모드는
브라우저와 MCP token을 분리하고 strict cookie, same-origin 쓰기, request 제한,
loopback listener를 사용합니다. stdout은 stdio protocol 전용입니다. 시작 출력은
의도된 인증 정보 전달 경로이며 일반 진단은 token을 반복하지 않습니다.

XLSX parser 전에는 OOXML ZIP, regular file, 암호화, 안전한 part 이름, 중복, 최대
10,000 parts, part당 256 MiB, 총 확장 512 MiB, 압축 원본 100 MiB 제한을
검사합니다. 이는 보안 상한이며 성능 보장이 아닙니다.

호스팅 모드에는 identity, 문서별 authorization, immutable object version, tenant
격리, audit storage, quota, background job이 추가로 필요합니다.

## 배포 모드

로컬 모드는 선택한 문서를 adapter로 열고 browser와 MCP HTTP를 loopback에
연결하거나 데스크톱 MCP host에서 stdio로 실행합니다. stdio host는 설치 시 문서
경로를 고정하지 않고 각 작업의 `open_workbook` 호출에서 사용자가 제공한 절대
경로를 사용합니다. package gate는 두 UI를 빌드하고 검증한 뒤 Go 바이너리에
포함하고 공백이 있는 절대 경로의 복사본으로 smoke test합니다.

호스팅 모드는 아직 구현하지 않았습니다. 구현 시 사용자와 MCP client 인증, 모든
문서 read/write authorization, immutable object storage, 관계형 metadata와 audit,
제한된 conversion job이 필요합니다.

## 남은 결정

- 두 번째 adapter 뒤의 첫 shared session interface
- DOCX native Word·LibreOffice 검증, 구조 편집과 풍부한 서식
- PPTX canvas와 animation 보존
- HWPX 범위와 binary HWP 연동
- PDF annotation, form, OCR, 재생성 경계
- XLSX 수식 계산과 남은 고급 기능 mapping
- 호스팅 tenant, 영속 event replay, audit 저장

XLSX editor 결정은 [ADR 0001](adr/0001-spreadsheet-engine.md)을 참고하세요.

DOCX의 저장·보안 제한과 native 검증 한계는 [사용 계약](docx-editing.md)과
[ADR 0002](adr/0002-docx-targeted-text-editing.md)를 따릅니다.

## 문서 안 검색

XLSX `search_workbook`과 인증된 `GET /api/search?query=...`는 adapter의 읽기 전용
검색을 공유합니다. 대소문자를 구분하지 않는 일반 문자열로 모든 sheet의 저장된
표시값을 찾습니다. 수식을 계산하지 않으며 결과 200개, 행과 셀을 합한 탐색량
1,000,000개에서 중단하고 `truncated`를 반환합니다. 검색어는 200자까지입니다.
세션 ID와 리비전이 바뀐 응답은 UI에서 폐기합니다. DOCX 검색은 렌더링된 본문과
표의 run을 문단별로 이어서 미저장 입력까지 포함하고 문단당 한 위치를 반환합니다.
두 편집기는 검색 버튼, Ctrl/Cmd+F, Escape와 결과 위치 이동을 제공합니다.
검색은 문서나 리비전을 변경하지 않습니다. 바꾸기와 다중 파일 검색은 포함하지 않습니다.

## 다중 문서 작업 공간

브라우저의 기본 진입점은 열린 문서 목록과 탭을 표시하는 작업 공간입니다.
같은 프로세스에서 최대 8개의 XLSX·DOCX를 보관하며 정규화된 절대 경로가 같은
파일은 기존 세션을 재사용합니다. 폴더를 스캔하거나 업로드 복사본으로 바꾸지 않습니다.
최근 10개 경로는 해당 브라우저의 localStorage에만 기록하고 지울 수 있습니다.

전송 계층 registry는 각 세션에 고정된 작은 서버 facade를 보관합니다.
`GET/POST /api/documents`와 `DELETE /api/documents/{id}`가 목록·열기·닫기를
담당하고 기존 API 및 MCP에 `?document=<id>`를 붙이면 해당 세션으로만 전달합니다.
닫히거나 잘못된 ID는 활성 문서로 fallback하지 않습니다. 기존 MCP 열기는 마지막으로
명시적으로 연 문서를 기본 대상으로 유지하고 `documentId`로 읽기·쓰기·이력을 지정할
수 있습니다. 여러 XLSX가 열린 상태의 대상 없는 쓰기는 거부합니다. DOCX 쓰기는
기존 `sessionId`로 열린 문서를 선택합니다. 각 adapter의 저장과 리비전 규칙은 유지합니다.

편집기는 인증된 same-origin iframe 안에서 독립적으로 동작합니다. 기존 Univer와
DOCX의 전역 focus·단축키·DOM ID를 격리하고 탭 전환 시 unmount하지 않아 초안과
선택 위치를 보존합니다. 처음 선택한 문서만 편집기를 생성하고 숨긴 편집기는 같은
크기로 유지합니다. 각 frame의 HTTP·SSE는 URL의 문서 ID에 고정됩니다. 숨긴 탭의 SSE 연결은
닫고 돌아올 때 마지막 event ID부터 replay합니다. HTTP/1 연결 수가 열린 문서 수만큼
늘어 저장 요청을 막지 않게 하며, 놓친 변경이나 replay gap은 기존 복구 규칙을 따릅니다.
`embedded=1`인 문서 화면만 CSP frame-ancestors self와 X-Frame-Options SAMEORIGIN을
사용하며, 일반 화면은 외부 embedding을 계속 거부합니다. 외부 origin은 허용하지 않습니다.

저장 상태와 닫기 확인은 origin·frame source·Zod payload 검증을 거친 postMessage로
전달합니다. 저장·로딩 중에는 닫지 않고 DOCX는 저장 후 닫기, 입력 폐기 또는 취소를
제공합니다. 전환은 저장을 유발하거나 초안을 폐기하지 않습니다. 프로세스 종료 후
초안 복원, 문서 간 원자적 저장, 비교와 나란히 보기는 제공하지 않습니다.

## 브라우저 로컬 파일 선택

`POST /api/documents/upload`는 인증·Origin을 검증하고 요청당 파일 하나를 streaming
방식으로 받습니다. XLSX는 100 MiB, DOCX는 32 MiB이며 multipart 전체도 제한합니다.
최대 8개 세션 제한, 확장자와 실제 package 검증 후에만 등록합니다. 브라우저가
선택한 파일의 원래 절대 경로를 추측하지 않습니다. 무작위 private 임시 디렉터리에
0600 파일을 만들고 일반 형식 adapter로 엽니다. 닫기나 정상 종료 시 해당
디렉터리만 정리합니다. 비정상 종료 시 OS 임시 파일 정리 정책에 따릅니다.

UI는 이를 작업 복사본으로 표시하고 원본을 변경하지 않는 점과 다운로드를
안내합니다. 임시 경로는 최근 파일 목록과 공개 문서 metadata에 넣지 않습니다.
경로로 직접 연 파일은 기존처럼 그 경로가 저장 대상입니다.
작업 복사본은 저장된 상태여도 닫기 전에 삭제와 다운로드 안내를 확인합니다.
복사본에 대한 저장 확인을 원본 파일의 지속 보관으로 오해하지 않게 합니다.

## 저장 확인과 다운로드

XLSX는 셀 편집 종료와 직렬 작업 대기를 마치고, DOCX는 초안을 일괄 저장한 뒤
`POST /api/save`로 문서 ID·세션·기준 리비전을 확인합니다. 저장 확인은 새 쓰기가
아니며 파일과 리비전·이력을 변경하지 않습니다. `GET /api/download`는 같은 세션과
리비전의 파일을 읽고 외부 변경을 확인한 원본 bytes를 반환합니다. 다운로드가
문서를 재직렬화하지 않습니다. 실패나 충돌이면 파일을 반환하지 않습니다.

브라우저는 저장·다운로드를 수행하는 동안 편집과 이력 작업의 경합을 막고,
응답의 ID·리비전·MIME을 확인한 뒤 파일명과 원래 확장자로 다운로드를 요청합니다.
브라우저 다운로드 요청과 사용자 디스크 저장 완료는 구분합니다. MCP inline에서는
`confirm_workbook_save`로 저장 확인을 제공하고 파일 다운로드는 브라우저를 안내합니다.

## DOCX 문단 구조 계획

[ADR 0004](adr/0004-docx-paragraph-plans.md)에 따라 adapter가 지원하는 연속
문단만 `replace_paragraphs`로 교체합니다. 브라우저는 원본 문단·run의 서식 참조와
별도 초안 ID를 유지합니다. 연속 입력 중에는 입력 node를 유지하고 구조 변경 시
커서를 새 초안 위치로 복원합니다. 저장은 원본 revision과 계획을 함께 전송하고
서버의 전체 검증·원자 저장·단일 commit을 거칩니다. HTTP와 MCP는 같은 typed
작업을 사용하며 전송 계층은 OOXML을 생성하지 않습니다.

계획은 원본 run·문단 서식을 참조하되 새 문단의 고유 ID를 복제하지 않습니다.
본문/셀 경계를 넘거나 보호된 구역·필드·참조를 포함한 계획은 거부합니다. 기존
원본 보존 방식과 달리 변경한 문단 내부는 재구성할 수 있지만 대상 밖 XML·다른
package part를 유지합니다. 구조 초안이나 다른 revision의 활동은 이전 위치에
매핑하지 않습니다. 서버 이력과 안정적인 원격 anchor는 별도 이슈를 따릅니다.
