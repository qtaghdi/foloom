# DOCX 생성과 텍스트 편집

상태: 비공개 로컬 실험 기능. 새 문서 생성, 본문 일반 run과 단순 표 cell의 text
run 교체를 제공합니다. Word 전체 편집기나 정확한 페이지 미리보기가 아닙니다.
Microsoft Word와 LibreOffice native reopen 및 시각 증거는 아직 없습니다.
실제 문서는 복사본으로 작업하세요.

## 시작

최신 소스에서 `make build`로 바이너리를 만든 뒤 실행합니다. 기존에 설치한 MCP
바이너리는 이 소스 변경만으로 갱신되지 않습니다. 설치 workflow를 사용하는 경우
새 실행 파일로 `mcp install`을 수행한 뒤 호스트를 완전히 다시 시작합니다.

```sh
./bin/document-workspace -check /absolute/path/to/document.docx
./bin/document-workspace /absolute/path/to/document.docx
```

출력되는 인증 URL로 DOCX 브라우저 편집기를 엽니다. 지원하는 각 run이 입력칸으로
표시되며, 여러 본문·표 입력을 바꾼 뒤 **변경 저장**을 누르면 하나의 리비전으로
저장합니다. Ctrl+S 또는 Command+S도 같은 저장을 수행합니다. 굵게와 기울임은 간단히 표시하지만 페이지 배치, 글꼴, 그림과 전체
서식은 재현하지 않습니다. 표는 좁은 화면에서 가로 스크롤합니다.

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
