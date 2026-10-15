# MCP 호스트 호환성

이 문서는 지원 연결 모드와 호스트 인증에 필요한 증거를 기록합니다. 프로토콜
연결 성공만으로 MCP App UI가 실제로 표시되고 동작한다고 판단하지 않습니다.

## 지원 모드

| 호스트 | 전송 | 구조화 MCP 도구 | native MCP App | 인증된 browser fallback | 상태 |
| --- | --- | --- | --- | --- | --- |
| Claude Desktop | stdio | 구현됨 | 수동 확인 필요 | 구현됨 | candidate 인증 대기 |
| Codex Desktop | stdio | 구현됨 | 확인되지 않음 | 2026-09-30 확인 | fallback만 역사적 인증 |
| 일반 HTTP MCP client | Streamable HTTP | 구현됨 | 호스트별 | 별도 viewer URL | 프로토콜 테스트 있음 |

host version, OS, transport, UI mode, fixture, 사람 및 AI 편집, conflict, native
reopen을 같은 candidate에서 기록해야 인증됩니다. browser fallback 통과를 native
embedded UI 통과로 표현하지 않습니다.

## Codex browser fallback 기록, 2026-09-30

- 호스트: Codex Desktop production 26.924.22138, build 11645
- 운영체제: macOS 26.5.1, build 25F80
- 전송: release binary의 stdio MCP와 같은 process의 임시 `127.0.0.1` UI
- fixture: A1과 B2에 값이 있는 disposable XLSX
- 실행: `open_workbook`의 일회용 URL을 Browser panel에서 열어 HTTP-only cookie로
  교환하고 token이 없는 통합 문서 root로 redirect
- 렌더링: 이름, revision 1, sheet tab, A1 `Browser fallback`, B2
  `Live workbook session` 표시
- 사람 편집: F15에 `Human committed`, revision 1에서 2로 증가
- AI 편집: `update_presence`로 C3 선택 후 `apply_operations`로 `AI committed`,
  화면과 revision 3에 반영
- 재열기: stdio 종료 뒤 XLSX를 다시 열어 F15와 C3 모두 보존 확인
- 범위: fallback의 실행, 렌더링, browser 저장, MCP presence, AI event 동기화만
  인증하며 native MCP App은 미확인

## 로컬 호스트 binary 빌드

```bash
make build
```

binary는 `bin/document-workspace`에 생성됩니다. desktop 앱은 shell 작업 디렉터리를
상속하지 않으므로 설정에 절대 경로를 사용합니다.

macOS에서는 `./bin/document-workspace mcp install`이 실행 파일을 고정된 사용자
경로로 복사하고 Claude Desktop 및 Codex MCP 설정과 `document-xlsx` skill을
설치합니다. `mcp status`로 각 구성 요소를 확인하고 `mcp uninstall`로 관리 항목만
제거할 수 있습니다.

## Claude Desktop stdio 설정

```json
{
  "mcpServers": {
    "document-workspace": {
      "command": "/absolute/path/to/document-workspace/bin/document-workspace",
      "args": ["-transport", "stdio"]
    }
  }
}
```

설정 뒤 Claude Desktop을 완전히 종료하고 다시 실행합니다. MCP message는 stdout,
진단은 stderr로 출력합니다. native MCP App이 없으면 `open_workbook`을 호출하고
일회용 `browserUrl`을 Browser panel에서 엽니다.

## Codex stdio 설정

```toml
[mcp_servers.document_workspace]
command = "/absolute/path/to/document-workspace/bin/document-workspace"
args = ["-transport", "stdio"]
```

설정 뒤 새 Codex session을 시작합니다. native MCP App이 없으면
`open_workbook`의 일회용 URL을 앱 내 browser에서 엽니다.

Claude는 MCP prompt 목록의 `document-xlsx`를 선택하거나 host가 노출한 slash
command를 사용할 수 있습니다. Codex는 `$document-xlsx`로 같은 workflow를
명시적으로 시작할 수 있습니다. host UI가 prompt 이름을 namespace와 함께 표시할
수 있으므로 정확한 표기는 설치된 host 버전에서 확인합니다.

## 호스트 승인 모드

서버는 통합 문서 읽기를 read-only, presence update를 non-destructive, 쓰기를
destructive로 표시합니다. 이는 호스트가 승인 정책을 정할 정보이며 특정 버튼을
강제하지 않습니다.

일반 편집은 작업 batch 전체를 확인한 뒤 `이번 작업만 허용`을 권장합니다.
`항상 허용`은 선택한 도구 또는 범위의 영속 host rule이므로 통합 문서, server
command, 이후 작업 범위를 모두 신뢰할 때만 사용합니다. 자동 모드는 호스트가
호출별 확인 여부를 정하므로 prompt 없이 쓰기가 실행될 수 있습니다.

호스트 승인은 표시된 요청 하나에만 적용됩니다. 서버는 승인 뒤에도 기준 리비전,
전체 batch, 통합 문서 제한, 원자적 저장을 검증합니다. conflict가 나면 오래된
승인을 재사용하지 않고 새 proposal을 검토해야 합니다.

## 수동 인증 절차

대표 통합 문서의 복사본만 사용합니다.

1. `document-workspace mcp status`에서 실행 파일, 두 MCP 설정, 두 skill을 확인합니다.
2. `document-workspace`만 활성화한 새 호스트 대화를 시작합니다.
3. 초기화, `document-xlsx` prompt 또는 skill, server instruction 노출을 확인합니다.
4. 절대 경로로 `open_workbook`을 호출한 뒤 `get_workbook`, `read_range`를 호출합니다.
5. native iframe 또는 일회용 browser fallback을 엽니다.
6. UI에서 셀 하나를 편집하고 리비전이 정확히 한 번 증가하는지 확인합니다.
7. AI가 `update_presence`, `apply_operations`를 호출하게 하고 선택과 값 반영을
   확인합니다.
8. stale revision을 제출해 부분 변경 없이 거부되는지 확인합니다.
9. Excel 또는 LibreOffice에서 다시 열어 값과 주변 서식을 확인합니다.
10. host version, OS, transport, mode, 결과, 차이를 이 문서에 기록합니다.

## 알려진 제약

- 자동 설치는 현재 macOS만 지원합니다. Windows와 Linux는 수동 host 설정을
  사용합니다.
- 로컬 MCP는 사용자가 제공한 절대 `.xlsx` 경로만 열며 디렉터리 검색이나 파일
  선택기를 제공하지 않습니다.
- hosted service에는 OAuth, 문서별 authorization, stable HTTPS, 영속 storage가
  필요합니다. 로컬 bearer token은 hosted identity가 아닙니다.
- inline MCP App 지원과 일반 MCP tool 지원은 다를 수 있습니다.
- stdio browser fallback은 같은 machine의 loopback에 접근 가능한 host가
  필요합니다. 원격 connector는 launch URL을 사용할 수 없습니다.

참고: [OpenAI MCP server quickstart](https://developers.openai.com/plugins/build/app-quickstart),
[OpenAI MCP App UI 안내](https://developers.openai.com/plugins/build/chatgpt-ui)
