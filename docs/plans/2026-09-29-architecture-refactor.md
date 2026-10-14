# 아키텍처 정리 기록

## 목표

통합 문서 동작, 공개 MCP 도구, HTTP route, 보이는 UI를 바꾸지 않고 기존 domain,
transport, application, spreadsheet engine 경계에 맞춰 Go와 TypeScript 구조를
정리합니다.

## 당시 상태와 범위

- `main.ts`가 시작, 리비전, write 직렬화, subscription, presence, 상태 표시를 모두
  담당했습니다.
- `api.ts`가 HTTP와 MCP App transport를 한 class에서 분기했습니다.
- `univer-engine.ts`가 Univer lifecycle, mapping, event 적용, A1 helper를 함께
  담당했습니다.
- Go의 `server.go`와 `session.go`도 서로 다른 책임을 한 파일에 모았습니다.

작업은 local architecture skill 추가, TypeScript orchestration·transport·view·engine
분리, discriminated union, 외부 입력 Zod 검증, Go 파일 책임 분리를 포함했습니다.
시각 redesign, React, 새 작업, hosted storage, Python MVP 제거는 제외했습니다.

## 결정

- 새 package를 만들기 전에 기존 Go package 안에서 책임별 파일을 나눕니다.
- browser와 editor resource는 명시적 dispose가 있는 stateful class를 사용합니다.
- Zod는 외부 TypeScript 경계에서만 사용합니다.
- function declaration과 arrow function은 architecture 규칙이 아니라 지역 가독성
  선택입니다.

```text
main.ts
  -> WorkbookController
      -> WorkbookClient
          -> HTTP transport 또는 MCP App transport
      -> AppView
      -> SpreadsheetEngine
          -> Univer adapter, mapper, event 적용

Go CLI
  -> HTTP 또는 stdio adapter
      -> xlsx.Session
          -> validation과 operation
          -> XLSX read와 persistence
          -> ordered event stream
```

## 결과

2026-09-29 완료했습니다.

- `document-workspace-architecture` skill을 추가하고 검증했습니다.
- `WorkbookController`, `WorkbookClient`, HTTP 및 MCP App transport, `AppView`,
  Zod boundary schema, strict operation과 event union을 도입했습니다.
- A1 변환, Univer workbook mapping, command mapping을 engine lifecycle에서
  분리했습니다.
- Go HTTP, MCP, middleware, workbook type, read, operation, event, persistence를
  package ownership 변경 없이 나눴습니다.
- `make verify`, `git diff --check`, em dash 검사를 통과했습니다.
- 실제 Go HTTP 진입점에서 렌더링, revision 2 사람 편집, revision 3 MCP AI 편집과
  presence, 640 x 800 화면을 확인했습니다.
- Claude Desktop과 Codex 재시작 인증은 설치된 host 설정이 필요한 별도 수동
  검사로 남겼습니다.
