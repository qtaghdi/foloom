# ADR 0001: 스프레드시트 엔진 경계 뒤에서 Univer 사용

- 상태: 승인됨
- 날짜: 2026-09-28

## 배경

document-workspace에는 MCP App 안에서 동작하고 사람이 직접 편집하며 AI의 커서,
선택, 입력, 확정 변경을 보여주는 Excel형 화면이 필요합니다. 호스트가 내장 앱을
표시하지 못해도 MCP 도구는 계속 사용할 수 있어야 합니다.

후보는 Univer, ONLYOFFICE Docs, 자체 엔진이었습니다. 자체 엔진은 grid, 수식,
선택, clipboard, 접근성, 협업 계층을 모두 직접 구현해야 합니다.

## 결정 기준

1. 셀과 범위 작업을 코드로 직접 제어할 수 있어야 합니다.
2. AI 현재 위치와 원격 선택 영역을 표현할 수 있어야 합니다.
3. 추가 nested iframe 없이 MCP App에 포함할 수 있어야 합니다.
4. XLSX 가져오기 및 저장의 원본 보존 가능성이 있어야 합니다.
5. 키보드, clipboard, 수식, 서식, 대용량 시트 사용성이 필요합니다.
6. 비공개 상용 서비스에도 맞는 라이선스와 배포 구조여야 합니다.
7. Go 통합 문서 도메인을 다시 작성하지 않고 엔진을 교체할 수 있어야 합니다.

## 비교

| 기준 | Univer | ONLYOFFICE Docs | 자체 엔진 |
| --- | --- | --- | --- |
| Excel형 기능 | 핵심 기능이 강하지만 일부 고급 기능은 Pro | 즉시 사용 가능한 범위가 가장 넓음 | 초기 기능이 약함 |
| XLSX 보존 | 교환 계층과 호환성 검증 필요 | 성숙한 변환 및 편집 경로 | 모두 직접 구현하고 유지 |
| AI 제어 | Facade API와 command 사용 | plugin 및 Office API 사용 | 큰 개발 뒤 완전 제어 |
| 원격 커서 | sheet range 기반 presence 가능 | 사람 공동 편집은 성숙, AI identity는 별도 필요 | 직접 구현 |
| MCP App 포함 | 앱 frame에서 직접 표시 | 자체 editor frame 때문에 nested frame 검토 필요 | 직접 표시 가능 |
| 초기 비용 | 중간 | 표준 편집은 낮음에서 중간 | 매우 높음 |
| 상업 조건 | OSS core는 Apache-2.0, Pro는 별도 검토 | 보통 Developer license 필요 | 내부 소유, 높은 인력 비용 |
| 교체 가능성 | adapter로 양호 | adapter로 양호 | 해당 없음 |

## 결정

Univer 오픈 소스 패키지를 `SpreadsheetEngine` 인터페이스 뒤의 UI 엔진으로
사용합니다. Univer Pro 패키지와 서비스는 사용하지 않습니다. 의존성 검사는
lockfile에 `@univerjs-pro/*`가 들어오면 실패합니다.

Go 서비스가 리비전, 검증된 작업, 충돌, 저장, 인증, MCP 도구의 기준 상태를
소유합니다. Univer는 렌더링과 상호작용 엔진일 뿐 원본 상태가 아닙니다. 협업과 AI
presence는 Go의 작업 로그, 리비전, SSE와 이후 양방향 presence channel로
구현합니다. XLSX 저장은 Excelize를 계속 사용합니다.

React는 필수 아키텍처가 아닙니다. 현재 연동은 TypeScript, Vite, Tailwind CSS를
사용하고 Univer 자체 editor style을 포함합니다.

## 결과

장점:

- 애플리케이션 frame 안에서 직접 동작합니다.
- Facade API를 MCP 작업에 자연스럽게 연결할 수 있습니다.
- command model을 실행 취소, 다시 실행, 협업으로 확장할 수 있습니다.
- 엔진 경계 덕분에 ONLYOFFICE나 자체 엔진으로 바꿀 경로가 남습니다.

비용과 한계:

- 협업, presence overlay, 높은 XLSX 원본 보존은 이 저장소가 직접 유지합니다.
- preset mode의 초기 bundle이 크므로 release 전 plugin mode와 code splitting을
  평가해야 합니다.
- 수식과 XLSX 보존은 API 존재만으로 가정하지 않고 대표 corpus로 검증합니다.

## 재검토 조건

다음 조건 중 하나라도 충족되면 결정을 다시 검토합니다.

- 호환성 corpus에서 허용할 수 없는 XLSX 손실이 확인됩니다.
- MCP App 제약 때문에 Univer를 안정적으로 실행할 수 없습니다.
- plugin mode 이후에도 합의한 시작 용량 기준을 만족하지 못합니다.
- OSS API로 권한이나 리비전 보장을 약화하지 않고 AI actor를 표현할 수 없습니다.

Univer가 실패하면 같은 엔진 경계 뒤에서 ONLYOFFICE를 평가합니다. 전체
스프레드시트 stack을 처음부터 자체 구현하지 않습니다. Univer Pro 도입에는 새
ADR과 저장소 소유자의 명시적 승인이 필요합니다.

## 참고 자료

- [Univer 설치와 preset mode](https://docs.univer.ai/guides/sheets/getting-started/installation)
- [Univer range 및 selection API](https://docs.univer.ai/guides/sheets/features/core/range-selection)
- [Univer collaboration presence](https://docs.univer.ai/server/collaboration/presence)
- [Univer import 및 export](https://docs.univer.ai/guides/sheets/features/import-export)
- [ONLYOFFICE 공동 편집](https://api.onlyoffice.com/docs/docs-api/get-started/how-it-works/co-editing/)
- [ONLYOFFICE spreadsheet API](https://api.onlyoffice.com/docs/office-api/usage-api/spreadsheet-api/Api/Methods/GetSelection/)
- [OpenAI MCP App UI 안내](https://developers.openai.com/plugins/build/chatgpt-ui)
