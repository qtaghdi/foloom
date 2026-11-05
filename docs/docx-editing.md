# DOCX 생성과 텍스트 편집

상태: 비공개 로컬 실험 기능. 새 문서 생성, 본문 일반 run과 단순 표 cell의 text
run 교체를 제공합니다. Word 전체 편집기나 정확한 페이지 미리보기가 아닙니다.
Microsoft Word와 LibreOffice native reopen 및 시각 증거는 아직 없습니다.
실제 문서는 복사본으로 작업하세요.

## 시작

최신 소스에서 `make build`로 바이너리를 만든 뒤 실행합니다. 기존에 설치한 MCP
바이너리는 이 소스 변경만으로 갱신되지 않습니다. 설치 workflow를 사용하는 경우
새 실행 파일로 `mcp install`을 수행한 뒤 호스트를 완전히 다시 시작합니다.
설치된 `document-workspace` 스킬에 DOCX 절대 경로와 작업을 전달할 수 있습니다.

```sh
./bin/document-workspace -check /absolute/path/to/document.docx
./bin/document-workspace /absolute/path/to/document.docx
```

출력되는 인증 URL로 DOCX 브라우저 편집기를 엽니다. 지원 일반 문단은 하나의
본문 편집 영역에서 run 서식을 유지해 표시하며, 보호된 문단의 지원 텍스트는
별도 입력칸으로 편집합니다. 여러 본문·표 입력과 구조를 바꾼 뒤 문서 헤더의
**저장**을 누르면 하나의 리비전으로 저장합니다. Ctrl+S 또는 Command+S도 같은 저장을 수행합니다. 단일 섹션은
원본 용지 크기와 여백을 사용하고, 문단 정렬·앞뒤 간격·글꼴·크기·색상·굵게·
기울임을 기본값, 문단·글자 스타일과 직접 지정 서식에서 읽습니다. 글꼴을 설치하지 않은 환경에서는 대체 글꼴을
사용합니다. 작은 화면에서는 용지와 여백 비율을 줄이고 본문 글자는 읽을 수
있는 크기로 유지합니다.

도구 모음에는 현재 입력의 글꼴과 크기가 표시됩니다. 보기 배율은 브라우저
표시에만 적용합니다.

일반 빈 문단과 비어 있는 run에도 입력칸을 표시합니다. 빈 문단에는
**여기에 입력하세요** 안내가 나타나며 클릭해서 글을 쓸 수 있습니다. 속성만 있는
빈 문단은 문단 속성을 보존하면서 일반 run을 삽입하고 빈 run에는 text만 삽입합니다.
필드·변경 추적 등 보호된 구조에는 빈 입력칸을 추가하지 않습니다. 지원 문단에서 Enter는 실제 DOCX 문단을 나누고 Shift+Enter는 문단 안 줄바꿈을
추가합니다. 문단의 첫 입력 시작에서 Backspace, 마지막 입력 끝에서 Delete는
같은 본문 구간 또는 같은 셀의 인접 문단을 합칩니다. Alt+Shift+↑/↓는 문단을
이동합니다. 여러 줄의 일반 텍스트를 붙이면 여러 문단 초안으로 나눕니다.

구조 변경도 저장 전에는 초안이며 계속 입력·분리·병합한 뒤 한 번에 저장합니다.
서버 실패·충돌에서는 초안을 남기고 자동으로 재시도하지 않습니다. 구조 변경 중
이전 위치 기반 AI 표시는 숨기며 저장 뒤 새로운 revision-scoped ID를 사용합니다.

필드·주석·북마크·변경 추적·구역 속성·명시적 페이지 경계·opaque XML이 있는
문단은 구조 편집을 보호합니다. 지원 텍스트는 기존처럼 편집할 수 있습니다.
병합·중첩 표는 읽기 전용입니다. 지원 일반 문단의 run·문단·표 셀 사이에서
native 선택과 범위 교체를 제공합니다. 같은 본문 구간 안의 선택은 문단을 합칠 수
있지만 다른 셀의 선택을 바꿀 때에는 셀 경계를 유지하고 각 셀 안의 선택 텍스트만
바꿉니다. 보호된 문단이 끼어 있으면 범위 변경을 차단합니다. 선택 밖 run 서식은
유지하고 삽입 텍스트는 선택 시작 run의 서식을 사용합니다.
새 문단은 첫 원본 문단의 문단 서식, 각 run은 참조한 원본 run 서식을 사용합니다.
병합 결과의 문단 서식은 앞 문단을 따릅니다. 탭과 지원하지 않는 제어 문자는
계속 거부합니다. native IME·Word 재편집과 모든 문서 호환성을 보장하지 않습니다.

작업 공간의 **내 컴퓨터에서 열기**로 선택한 파일은 원본을 유지하는 작업 복사본입니다.
**저장**은 이 복사본에 반영하며 **다운로드**는 초안을 저장한 뒤 수정본을 받습니다.
**경로로 열기**로 연 문서는 그 경로에 저장합니다.

본문은 문단 앞 또는 끝에 저장된 명시적 나눔에서 새 용지로 이어집니다. 자동
페이지 매김, 문단 중간 나눔, 섹션별 배치, 그림, 머리글·바닥글과 복잡한 표 배치는
표시 범위에서 제외합니다. 표는 좁은 화면에서
가로 스크롤합니다. 표시용 메타데이터는 저장 입력에 포함되지 않습니다.

연결 중단이나 외부 변경 및 리비전 충돌 시 미저장 입력을 유지하고 저장을
차단합니다. 필요한 입력을 복사한 뒤 **다시 불러오기**를 누릅니다. 미저장 입력이 있으면 자체
확인 UI의 **입력 버리고 불러오기**를 선택해야 복구합니다. **취소**는 초안을
유지합니다. 화면은 Solid 컴포넌트로 구성하며 controller가 저장과 draft를 소유합니다. 저장 결과를
받지 못한 요청은 자동 재시도하지 않습니다. 이 DOCX 기능에는 서버 undo/redo가
없습니다.

## MCP 작업

파일 없이 시작한 stdio와 인증된 HTTP MCP 모두 다음 도구를 제공합니다. 기존
XLSX 도구와 계약은 유지하며 DOCX 활성 문서는 별도로 관리합니다.

| 도구 | 입력과 결과 |
| --- | --- |
| `create_docx` | 새 절대 `.docx` 경로, 순서 있는 paragraph/table block. 생성한 문서를 활성화하고 snapshot과 `browserUrl` 반환 |
| `open_docx` | 기존 절대 `.docx` 경로. 검사 후 활성화하고 snapshot과 `browserUrl` 반환 |
| `get_docx` | 세션 ID, 리비전, 본문·표, run ID, 편집 가능 여부, 호환성 안내 반환 |
| `apply_docx_operations` | 현재 `sessionId`, `baseRevision`, typed operation batch. 원자적 저장 뒤 snapshot 반환 |

생성 예시:

```json
{
  "path": "/absolute/path/to/new.docx",
  "blocks": [
    { "type": "paragraph", "text": "회의록" },
    { "type": "paragraph", "text": "결정 사항을 기록합니다." },
    { "type": "table", "rows": [["항목", "담당"], ["초안", "팀"]] }
  ]
}
```

편집 예시의 식별자는 직전 `get_docx`에서 얻은 실제 값으로 바꿉니다.

```json
{
  "sessionId": "current-session-id",
  "baseRevision": 1,
  "operations": [
    {
      "type": "replace_run_text",
      "targetId": "current-body-run-id",
      "expectedText": "회의록",
      "text": "11월 회의록"
    },
    {
      "type": "replace_cell_text",
      "targetId": "current-table-run-id",
      "expectedText": "초안",
      "text": "검토 완료"
    }
  ]
}
```

ID는 세션과 리비전에 묶여 있습니다. 성공한 저장 뒤에는 최신 snapshot의 ID를
사용합니다. 반복되는 같은 문자열도 ID로 구분하며 문자열 검색으로 위치를 추측하지
않습니다. `expectedText` 불일치, 잘못된 ID, 중복 대상, 오래된 리비전, 미지원
operation이 하나라도 있으면 전체 batch를 거부합니다. 내용이 같은 batch는 파일과
리비전을 바꾸지 않습니다. `browserUrl`은 2분 동안 한 번만 사용할 수 있습니다.

## 지원과 보존 범위

- 생성: 문단, 직사각형 단순 표, 한글·Latin text, 공백과 XML 특수 문자
- 편집: 본문의 일반 text run, 병합되지 않은 단순 표의 일반 text run
- 보존: 대상 text element 이외 XML byte와 다른 ZIP part의 압축 해제 payload
- 읽기 전용 또는 생략: 변경 추적, field, hyperlink, content control, 병합·중첩 표,
  그림, 머리글·바닥글, note, comment와 알 수 없는 구조
- 미지원: run을 가로지르는 교체, 문단·행·열 추가 및 삭제, 새 서식 적용, 표 구조
  편집, 탭·줄바꿈 입력, 정확한 pagination, PDF 변환, DOC·DOCM·암호화·서명 문서,
  strict OOXML과 비표준 main document mapping

원본 run의 서식과 주변 구조는 다시 만들지 않습니다. text element에는 필요한 경우
`xml:space="preserve"`를 추가합니다. 기존 `xml:space="default"` 대상에 양끝 공백을
새로 넣는 작업은 거부합니다. ZIP 압축 byte와 timestamp까지 동일하다는 계약은
아닙니다. 전체 Word 호환성이나 모든 문서의 무손실 편집을 주장하지 않습니다.

## 안전 저장과 제한

기존 파일은 symlink를 해석해 연 뒤 regular file인지 확인합니다. 저장 시 협조
writer lock과 원본 hash를 확인하고, 같은 디렉터리의 임시 파일을 sync·close한 뒤
원자적 replacement를 수행합니다. 최종 교체 직전에도 원본을 검사합니다. Word
등 비협조 writer와 마지막 검사·교체 사이의 race는 남으므로 외부 앱과 동시 저장을
피하세요. 프로세스 중단으로 남은 `.document-workspace.lock`은 자동으로 훔치지
않으며 writer 종료를 확인한 뒤 관리자가 처리해야 합니다.

생성은 완성된 임시 파일을 exclusive hard link로 설치하므로 기존 파일이나 symlink를
덮어쓰지 않습니다. hard link를 제공하지 않는 파일 시스템에서는 생성에 실패합니다.
리비전은 현재 프로세스 세션 안에서 유효하며 다시 열면 새로운 세션 ID로 시작합니다.
영속 이력과 디렉터리 fsync에 의한 전원 장애 복구 보장은 제공하지 않습니다.

| 제한 | 값 |
| --- | --- |
| 압축 파일 / 전체 확장 / part | 32 MiB / 64 MiB / 16 MiB |
| ZIP entry / XML 깊이 / XML node | 2,048 / 64 / part당 100,000 |
| XML token | part당 500,000, 잘게 분할된 text는 누적 buffer로 처리 |
| 검사 식별자 수 | 10,000 |
| batch operation / 교체 text 합계 | 1,000 / 4 MiB |
| run text | UTF-8 64 KiB, 탭·줄바꿈과 XML 비허용 문자 거부 |
| 생성 block / 총 문단·cell | 1,000 / 3,000 |
| 생성 표 | 100행, 50열 이하 |
| 브라우저 쓰기 body | 1 MiB |
| event replay | 최근 commit 128개 |

외부 relationship은 보존만 하고 네트워크로 가져오지 않습니다. archive traversal,
중복 entry, 잘못된 내부 relationship, XML directive, 과도한 구조, macro와 서명은
거부합니다. 브라우저는 문서 text를 HTML로 해석하지 않으며 기존 token, cookie,
Origin과 CSP 경계를 사용합니다.

## 검증

[합성 corpus](../testdata/docx/compatibility/README.md),
[adapter 테스트](../internal/formats/docx/session_test.go),
[MCP·HTTP 통합 테스트](../internal/transport/httpapi/docx_test.go),
[브라우저 복구 테스트](../apps/web-editor/tests/docx.test.mjs)를 함께 사용합니다.

```sh
make verify
make web-test
make compatibility-test
make package-test package-check
```

Word와 LibreOffice 생산 fixture, native reopen, 시각 비교와 실제 호스트별 DOCX
workflow 인증은 남은 검증입니다. 기존 XLSX 인증 결과를 DOCX 인증으로 재사용하지
않습니다.

## 문단 구조 도구 계약

`apply_docx_operations`와 `/api/docx/operations`의 `replace_paragraphs`는 같은
부모의 연속된 `structureEditable` 문단 범위를 받습니다. `targetId`와
`endTargetId`는 처음과 마지막 문단 ID, `expectedText`는 원본 문단 텍스트를 LF로
이어 붙인 값입니다. `text`는 빈 문자열입니다. `paragraphs`의 각 항목은 문단
서식용 `sourceId`와 `runs`를 가지며 각 run도 범위 안 원본 run의 `sourceId`와
새 `text`를 가집니다. source ID는 현재 리비전의 참조이며 새 OOXML ID가 아닙니다.
문단 배열을 늘리거나 줄이고 순서를 바꿔 추가·분리·병합·삭제·이동합니다.
결과는 최소 한 문단이며 한 계획은 최대 1,000문단입니다. 쓰기 범위끼리 겹치면
하나의 계획으로 합쳐야 합니다.

같은 문단을 여러 번 참조해도 원본 고유 ID를 복제하지 않습니다. 원본 run의 서식과
필요한 XML namespace를 함께 옮기며 계획 범위 밖 document XML과 다른 package
part는 유지합니다. 수정된 문단 안 XML의 byte 동일성은 보장하지 않습니다.
`replace_run_text`와 `replace_cell_text`의 LF는 `w:br`이며 새 문단이 아닙니다.

## 연속 선택과 문자 경계

본문의 방향키·Home/End·단어 이동과 드래그 선택은 브라우저의 native 선택을
사용합니다. 범위 쓰기 전에 선택을 문단/run의 UTF-16 위치로 변환하고 grapheme
경계를 검사합니다. 이모지 ZWJ, skin tone과 조합 문자를 중간에서 자르지 않습니다.
지원하지 않는 Intl 환경에서는 문자 경계를 추정해 분할하지 않습니다.

일반 run 안의 연속 타이핑은 입력 DOM을 유지하고 문단 경계를 바꾸는 작업만
초안 계획으로 반영합니다. native IME가 DOM을 소유하는 동안 다시 렌더링하지
않고 compositionend의 최종 텍스트를 한 번 적용합니다. 조합 중에는 저장을
차단하며 실제 OS·기기 IME 인증은 실행한 환경만 별도로 기록합니다. 검색은
미저장 본문의 run 텍스트도 읽고 저장은 확정된 응답 뒤에만 완료 상태를 표시합니다.

## Clipboard와 기본 글자 서식

도구 모음의 **붙여넣기 서식**에서 기본값 **텍스트만** 또는 **기본 서식 유지**를
고릅니다. Ctrl/Cmd+Shift+V는 텍스트만 사용합니다. 기본 서식은 굵게·기울임,
120 UTF-8 byte 이하 글꼴 이름, 4~144pt 크기와 불투명 RGB 색으로 제한합니다.
문단·표·그림·링크·목록·고급 서식의 완전한 clipboard 이식은 아닙니다.
지원하지 않는 항목은 안내하고 텍스트와 지원 글자 서식만 사용합니다. 이미지 alt는
텍스트로 보존할 수 있지만 외부 그림을 내려받거나 임베디드 코드를 실행하지 않습니다.
보호된 구조의 별도 입력칸에서는 기존 일반 텍스트 붙여넣기를 사용합니다.

HTML은 live DOM에 붙이지 않고 inert template에서 제한된 텍스트·서식 값만 읽습니다.
문서가 있는 부모의 source run을 참조하고 새 run에 명시적 `style`을 추가합니다.
서버가 색·글꼴·크기와 전체 계획을 다시 검증하고 지정하지 않은 원본 run 속성을
유지합니다. 잘라내기는 clipboard 기록이 성공한 다음 초안에서 범위를 삭제합니다.
텍스트 없는 이미지 clipboard는 선택을 삭제하지 않습니다. 탭과 제어 문자, 요청
크기 등 저장 제한은 유지하며 지원하지 못한 입력을 성공으로 표시하지 않습니다.
