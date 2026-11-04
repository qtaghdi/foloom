# 변경 이력

이 프로젝트의 중요한 변경 사항을 기록합니다. 형식은
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/)를 따르며 정식 릴리스부터
Semantic Versioning을 적용할 예정입니다.

## 미출시

### 추가

- #45: 실제 저장 응답·수신 event 기반의 session 활동 패널을 추가했습니다.
  저장 중·저장됨·확인 필요를 구분하며 문서 내용이나 token은 기록하지 않습니다.

- #43: 실제 선택 범위의 굵게·기울임·글자 크기를 서버에 저장하는 자체 XLSX
  도구 모음과 선택 범위 표시, 기본 ribbon의 고급 도구 전환을 추가했습니다.

- XLSX browser·MCP App과 DOCX의 앱 UI를 Solid 컴포넌트와 반응형 표시 상태로
  전환하고, DOCX 초안 폐기 확인을 자체 UI로 제공합니다.
- 실제 Solid DOM에서 engine container·입력 focus 보존, 저장·충돌·복구와 root
  해제를 검사하는 컴포넌트 테스트를 추가했습니다.

- DOCX 문단·단순 표 생성, 본문·표 run 텍스트의 리비전 기반 batch 편집, 원본
  OOXML 표적 보존, MCP 도구 4개와 인증된 브라우저 편집기를 추가했습니다.
  미지원 구조는 보호하며 Word native 호환성은 아직 검증하지 않았습니다.
- DOCX archive·XML 제한, 외부 writer 감지, 덮어쓰기 없는 생성, 원자적 저장,
  충돌 시 미저장 입력 유지와 합성 호환성 corpus를 추가했습니다.

- browser Alert의 접기, 펼치기, 닫기 control과 MCP의 browser-first workbook 편집
  workflow를 추가했습니다.
- macOS에서 실행 파일, Claude Desktop과 Codex MCP 설정, `document-xlsx` skill을
  함께 관리하는 `mcp install`, `status`, `uninstall` 명령을 추가했습니다.
- 파일 없이 시작한 stdio MCP에서 절대 `.xlsx` 경로를 동적으로 열고 전환하는
  `open_workbook` 입력과 필요한 범위만 읽는 `document-xlsx` prompt를 추가했습니다.
- macOS, Linux, Windows의 AMD64와 ARM64용 미서명 비공개 알파 archive, SHA-256
  manifest, 내용 검사, 현재 platform 실행 smoke test를 추가했습니다.
- 공개 Release 권한이 없는 수동 private-alpha workflow, CLI build metadata,
  위치 인자 경로, cell을 출력하지 않는 preflight를 추가했습니다.
- checksum, 미서명 앱, browser, Claude Desktop, Codex, 승인, backup을 다루는
  비공개 알파 안내를 추가했습니다.
- safety, compatibility, 전체 suite, packaging, release performance를 한 번에
  기록하는 Excel 실행 전 검증과 필수 전체 regression CI를 추가했습니다.
- 25,000셀 CI와 400,000셀 release 성능 profile, latency 및 Go heap budget,
  Markdown 증거를 추가했습니다.
- X-07과 X-08용 Microsoft Excel fixture, 편집, 수식, reopen 절차와 증거 양식을
  추가했습니다.
- X-01부터 X-06까지 재현 가능한 safety 검증과 CI, mixed batch와 동시 writer의
  memory·disk·history·event 원자성 테스트를 추가했습니다.
- cached 및 unavailable 수식 값을 구분하고 browser 결과를 preview로 표시하며
  native 전체 재계산을 요청하는 수식 정책을 추가했습니다.
- source hash, 협조 writer lock, 외부 변경 conflict, event checkpoint, replay
  resync, slow subscriber disconnect, blind retry 없는 browser recovery를
  추가했습니다.
- persistence failure injection, HTTP/SSE, frontend recovery, history 10개와
  snapshot 32 MiB 제한, restart, 단조 증가 revision, unavailable UI 테스트를
  추가했습니다.
- browser와 MCP 인증 negative matrix, Origin 무변경, 인증 정보 및 문서 비노출,
  로컬 credential 전달 경계를 추가했습니다.
- clean checkout package CI, bundle validation, 공백 절대 경로 실행 smoke test를
  추가했습니다.
- keyboard 편집, 원자적 사각 붙여넣기, server history UI, 접근 가능한 상태,
  reduced motion, safe area, 반응형 화면 증거를 추가했습니다.
- 저장 실패가 기준 memory, revision, undo/redo를 훼손하지 않도록 모든 XLSX 변경과
  history 복원을 candidate에서 처리합니다.
- XLSX 안정 release checklist, DOCX 보존 실험, 공통 제품 명세, 형식별 지원표를
  추가했습니다.
- 교체 가능한 SpreadsheetEngine 뒤의 Univer OSS UI와 Vite, Tailwind CSS build를
  React application 의존성 없이 추가했습니다.
- 사람 cell 저장, AI commit 동기화, 인증 UI asset integration test, Univer와
  ONLYOFFICE 및 자체 엔진 비교 ADR을 추가했습니다.
- 통합 문서, secret, dependency, build artifact ignore 규칙, monorepo command,
  Univer Pro 차단, issue 및 Pull Request workflow를 추가했습니다.
- 공식 MCP Go SDK와 Excelize 기반 service, get_workbook, read_range,
  apply_operations, revision conflict, 원자적 file replacement를 추가했습니다.
- 인증 browser와 MCP endpoint, 직접 cell 편집, AI cursor·typing·commit SSE,
  사람 및 AI range presence, Last-Event-ID replay를 추가했습니다.
- 사각 붙여넣기, 기본 서식, merge, style과 merged-cell rendering, browser와 MCP
  credential 분리, workbook과 batch limit를 추가했습니다.
- MCP App resource, open_workbook, app 전용 협업 도구, browser 및 self-contained
  bundle, stdio transport와 실제 subprocess test를 추가했습니다.
- Claude Desktop과 Codex 설정, host matrix, architecture skill, browser boundary
  validation을 추가했습니다.
- Claude와 Codex의 stdio 읽기, 승인, 쓰기, conflict, reopen 기록과 native MCP App이
  없는 host의 browser fallback을 추가했습니다.
- Codex Browser panel의 실행, 사람 편집, AI presence와 commit, reopen을
  인증했습니다.
- browser와 MCP 편집이 같은 revision, ordered event, XLSX를 공유하는 integration
  test를 추가했습니다.
- 수식, style, merge, validation, conditional formatting, drawing, link, name,
  comment 보존 corpus와 xlsx-compatibility-qa skill을 추가했습니다.
- 전 경계의 행·열 삽입 및 삭제, server 기준 bounded undo/redo, viewport range
  loading, compatibility notice를 추가했습니다.
- list 및 숫자·text·formula·rank·average·date·color scale·data bar·unique·duplicate
  규칙의 OSS Univer mapping과 LibreOffice fixture를 추가했습니다.
- image rendering, bar·line·area·pie·doughnut chart preview, object API를 Univer
  Pro 없이 추가했습니다.
- image와 chart의 이동·크기·삭제, 기존 chart title AI 변경, 사람 drawing transform
  저장과 원격 object 변경 reload를 추가했습니다.

### 변경

- XLSX 상단의 이력과 글자 서식 도구를 그룹으로 묶고 선택 상태를 강조합니다.

- XLSX 수식 입력줄을 기본 표시하고 도구와 호환성 안내를 간결하게 정리합니다.
  활동에서 기록된 시트와 셀 범위로 이동할 수 있습니다.

- 문서 편집 헤더를 간결하게 정리하고 DOCX 문단 폭 입력과 키보드 이동,
  조건부 저장 버튼, AI 변경 위치와 이전·이후 미리보기를 제공합니다.

- #47: Univer OSS의 한국어 locale와 앱 호환성·연결·충돌 메시지를 통일했습니다.
  문서 내용과 수식은 번역하지 않고 알 수 없는 원인은 그대로 표시합니다.

- #44: DOCX를 용지·문단·표 중심의 읽기 배치로 바꾸고 입력 테두리를 focus
  중심으로 줄였습니다. 원본 run 편집과 초안·cursor 유지 계약은 보존합니다.

- #42: XLSX와 DOCX의 header·문서명·상태·버튼을 흰색과 indigo 강조색의 공통
  디자인으로 통일했습니다.

- 독립 기능·수정별 커밋과 해당 검증·문서 동반 원칙을 AGENTS.md에 명시하고,
  불가분 변경은 하나로 유지해 형식적인 파일 단위 분리를 피합니다.

- 소유자가 읽는 문서, 변경 이력, GitHub issue 및 Pull Request 양식, 제품 문구의
  기본 언어를 한국어로 바꿨습니다. AI 전용 지침, 코드 주석, commit message,
  명령어, API 식별자는 영어를 유지합니다.
- native Excel 증거, 초대 tester 반복 사용, 배포 조건, 소유자 결정 전까지
  프로젝트를 비공개 로컬 알파로 제한했습니다.
- 자동 release trigger, contents write, 알려진 GitHub Release publisher를
  private-alpha 동안 차단합니다.
- 모든 변경에 issue 번호 branch와 연결 Pull Request를 의무화했습니다.
- 제품과 저장소를 document-workspace로 바꾸고 XLSX를 다중 형식 플랫폼의 첫
  adapter로 재정의했습니다.
- monorepo를 apps, cmd, internal/formats, internal/transport, testdata/xlsx,
  legacy로 정리하며 기존 계약을 유지했습니다.
- MCP App URI, Go module, executable, frontend package, local skill을 새 namespace로
  바꿨습니다.
- Go를 기준 service로 정의하고 Univer Pro 평가를 repository 소유 OSS 전략으로
  교체했습니다.
- Univer preset을 명시적 OSS plugin으로 바꾸고 MCP App bundle을 12.7 MB에서
  약 7.8 MB로 줄였습니다.
- npm에서 pnpm workspace와 정확한 Univer version catalog로 이전했습니다.
- MCP App version, server instruction, tool safety annotation, OpenAI alias를
  추가했습니다.
- browser orchestration, HTTP 및 MCP App transport, UI, workbook mapping, Univer
  command mapping을 TypeScript module로 나눴습니다.
- Go HTTP adapter와 workbook domain을 책임별 파일로 나누면서 route, tool, event,
  revision, 저장 계약을 유지했습니다.
- operation과 event를 required field가 분명한 discriminated union으로 바꿨습니다.
- host의 manual, automatic, one-time, persistent 승인을 문서화하고 image 및 chart
  write-back 한계를 안내에 반영했습니다.

- CI의 main 푸시 중복 검사를 제거하고 PR 변경 범위로 실행할 검사를 선택합니다.
  문서 전용 변경은 무거운 검사를 생략하고 DOCX 전용 변경은 XLSX 전용 회귀·안전성·
  성능 gate 대신 DOCX 검증을 실행합니다. 수동 실행은 전체 검증을 유지합니다.

### 수정

- XLSX 샘플 금액의 채우기와 글자 색상을 바로잡아 본문 대비를 복원했습니다.

- 같은 호스트의 다른 로컬 포트에 열린 DOCX와 XLSX가 브라우저 인증을 서로
  덮어쓰던 문제를 수정했습니다.

- #48: 좁은 화면의 활동 패널·touch target·안전 영역과 focus 복귀를 검증하고,
  DOCX 미지원 문자·요청 크기를 저장 전에 안내해 초안을 유지합니다.

- #45: 자기 저장의 SSE와 응답이 겹쳐 활동이 중복되던 경우를 방지했습니다.

- #43: 자체 도구로 focus를 옮겨도 해당 workbook의 선택과 서식을 참조하고,
  초기 렌더링 뒤 같은 engine에서 고급 ribbon을 전환하도록 보완했습니다.

- #46: 셀을 가리는 고정 경고를 inline 안내로 바꾸고 복구·초안 폐기 확인의
  keyboard focus 이동·취소 복귀와 Escape 동작을 추가했습니다.

- 저장이 확인된 실행 취소·다시 실행 후 재열기에서 불필요한 미저장 이탈 경고가
  나오지 않도록 복구 상태와 확정된 화면 이동을 구분합니다.

- 관련 없는 XLSX 편집 후에도 데이터 유효성 검사의 `showDropDown`,
  `showInputMessage`, `showErrorMessage` 명시 여부와 값을 보존합니다.
- XML 문자 참조로 저장된 데이터 유효성 검사 수식을 디코딩하고, 내용이 같은 저장은
  리비전과 undo 이력을 늘리지 않으며 유휴 browser가 원격 리비전을 동기화합니다.
- clean CI에서 Python MVP 회귀가 동작하도록 dependency를 고정했습니다.
- local edit, AI commit, history restore, reload, recovery 뒤 undo/redo availability를
  동기화했습니다.
- AI formatting과 merge 뒤 browser revision을 갱신해 stale write를 막았습니다.
- 초기 200행 대신 sheet dimension 기반 bounded range로 worksheet를 읽습니다.
- 크거나 미지원 object는 visible notice와 함께 생략해 시작을 막지 않습니다.

### 보안

- 프런트엔드 의존성 검사에서 발견한 MCP client SDK와 Zod의 알려진 취약점을
  수정 버전으로 갱신했습니다.

- credential을 fixed-length hash로 비교하고 framing, resource, capability,
  base URI, object, form 제한을 추가했습니다.
- expanded byte, part size와 수, encrypted·duplicate·unsafe ZIP entry를 검사하는
  XLSX preflight를 추가했습니다.
- Univer를 1.0.2로 올리고 알려진 dependency 취약점이 없음을 확인했습니다.
- 필요한 data와 blob image, font, worker만 허용하고 script는 same-origin으로
  제한했습니다.
- random token, strict cookie, Origin 검사, body와 range 제한, security header를
  추가했습니다.
- stdio browser fallback을 2분 single-use token, loopback, bounded token set으로
  보호했습니다.
- image byte를 검증하고 제한 안의 PNG, JPEG, GIF만 표시합니다.
- history manifest, image dimension, chart series, point, label을 memory load 전에
  제한합니다.

## 0.0.1 - 2026-09-28

### 추가

- 초기 Python XLSX viewer 및 editor 개념 증명을 추가했습니다.
- worksheet tab, 직접 cell 편집, list validation control, 제한된 expression 기반
  conditional formatting을 추가했습니다.
