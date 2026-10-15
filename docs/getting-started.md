# 비공개 XLSX 알파 시작 안내

document-workspace는 현재 기존 XLSX 통합 문서 하나를 여는 로컬 프로세스로
동작합니다. stdio로 Claude Desktop 또는 Codex에 연결하고, MCP App이나 인증된
loopback 브라우저 대체 화면으로 스프레드시트 UI를 제공합니다.

알파는 저장소 소유자와 명시적으로 초대한 테스터만 사용할 수 있습니다.
Microsoft Excel 대체품이나 호스팅 협업 서비스가 아닙니다. 실제 Excel 릴리스
검증이 끝날 때까지 중요한 통합 문서는 복사본으로 테스트하세요.

## 압축 파일 선택

권한 있는 관리자는 GitHub Actions의 `Private alpha package` workflow를 실행하거나
`make release-verify`로 로컬에서 빌드할 수 있습니다. workflow artifact는 비공개
저장소 안에만 남으며 GitHub Release가 아닙니다. 압축 파일 이름은 다음 규칙을
사용합니다.

| 운영체제 | 프로세서 | 압축 파일 접미사 |
| --- | --- | --- |
| macOS | Apple silicon | `darwin_arm64.tar.gz` |
| macOS | Intel | `darwin_amd64.tar.gz` |
| Linux | x86-64 | `linux_amd64.tar.gz` |
| Linux | ARM64 | `linux_arm64.tar.gz` |
| Windows | x86-64 | `windows_amd64.zip` |
| Windows | ARM64 | `windows_arm64.zip` |

모든 결과물에는 `SHA256SUMS`가 포함됩니다. 압축을 풀기 전에 선택한 파일을
검증하세요.

```bash
shasum -a 256 -c SHA256SUMS
```

현재 알파 압축 파일은 서명되지 않았습니다. macOS 또는 Windows가 확인되지 않은
개발자 경고를 표시할 수 있습니다. 다운로드한 파일과 체크섬을 확인하고 신뢰할 수
있을 때 운영체제의 일회성 승인 절차를 사용하세요. 운영체제 보호 기능을 전역으로
끄지 마세요.

배포 결정을 대신해 압축 파일을 재배포하거나 저장소 접근 권한을 주지 마세요.
소스나 바이너리를 공개하려면 [제품 계획](product-plan.md)의 종료 조건을 충족해야
합니다.

## 통합 문서 검사

MCP 호스트를 설정하기 전에 사전 검사를 실행합니다.

```bash
./document-workspace -check /absolute/path/to/workbook.xlsx
```

이 명령은 OOXML 패키지, 지원 크기와 압축 경계, 시트 메타데이터, 호환성 경고,
수식 정책을 검증합니다. 네트워크 listener를 시작하거나 셀 내용을 출력하지
않습니다.

## 브라우저 편집기 실행

통합 문서를 위치 경로로 전달합니다.

```bash
./document-workspace /absolute/path/to/workbook.xlsx
```

터미널에 출력된 viewer URL을 그대로 여세요. URL에는 로컬 세션 인증 정보가
포함됩니다. 터미널 출력이나 URL을 공유하지 마세요. 서비스는 기본적으로
`127.0.0.1:8765`에만 연결되므로 통합 문서를 네트워크에 공개하지 않습니다.

주소, 전송, 대체 화면 옵션은 `./document-workspace -help`로 확인합니다. 버그를
신고할 때는 `./document-workspace -version` 결과를 기록하세요.

## macOS MCP 자동 설치

압축을 푼 실행 파일에서 다음 명령을 한 번 실행합니다.

```bash
./document-workspace mcp install
```

이 명령은 다음 작업을 수행합니다.

- 실행 파일을 사용자 Application Support 아래의 고정 경로로 복사합니다.
- 기존 항목을 보존하면서 Claude Desktop과 Codex에 stdio MCP를 등록합니다.
- Claude와 Codex의 사용자 skill 디렉터리에 `document-xlsx`를 설치합니다.
- 변경한 host 설정의 직전 내용을 `.document-workspace.bak`으로 보관합니다.

설치 뒤 Claude Desktop과 Codex를 완전히 종료하고 다시 실행하세요. 상태 확인과
제거는 다음 명령을 사용합니다.

```bash
./document-workspace mcp status
./document-workspace mcp uninstall
```

자동 설치는 현재 macOS만 지원합니다. 손상된 JSON이나 `document-workspace` 관리
구간이 있는 설정은 덮어쓰지 않고 오류를 반환합니다. 제거는 이 프로그램이
관리하는 MCP 항목, skill, 복사된 실행 파일만 제거합니다.

## 문서 작업 시작

Claude에서 MCP prompt 목록의 `document-xlsx`를 선택합니다. host가 MCP prompt를
slash command로 표시하면 다음처럼 사용할 수 있습니다.

```text
/document-xlsx "/absolute/path/to/workbook.xlsx" "자산 분류 지침대로 수정해줘"
```

host 버전에 따라 명령이 MCP server namespace 아래에 표시될 수 있습니다. Codex는
다음처럼 명시적으로 skill을 호출합니다.

```text
$document-xlsx "/absolute/path/to/workbook.xlsx" 자산 분류 지침대로 수정해줘
```

두 host 모두 자연어로 절대 경로와 작업을 전달해도 MCP 도구를 사용할 수 있습니다.
`open_workbook`은 지정한 XLSX를 활성 문서로 만들며 같은 대화에서 다른 절대 경로를
전달하면 안전하게 전환합니다. 임의 디렉터리를 검색하거나 파일 경로를 추측하지
않으므로 사용자가 절대 경로를 제공해야 합니다.

workflow는 통합 문서 metadata를 먼저 확인하고 필요한 범위만 읽습니다. 가능한
경우 하나의 작업 묶음으로 변경하고, 최신 리비전으로 저장한 뒤 바뀐 범위를 다시
읽습니다. MCP App이 표시되지 않으면 `open_workbook`이 반환한 일회용
`browserUrl`을 여세요.

## 수동 연결 대체 절차

자동 설치를 사용할 수 없거나 설정을 직접 관리하려면 Claude Desktop MCP 설정에
실행 파일만 등록합니다. 통합 문서 경로는 설정에 고정하지 않습니다.

```json
{
  "mcpServers": {
    "document-workspace": {
      "command": "/absolute/path/to/document-workspace",
      "args": ["-transport", "stdio"]
    }
  }
}
```

Codex `config.toml`의 수동 설정은 다음과 같습니다.

```toml
[mcp_servers.document_workspace]
command = "/absolute/path/to/document-workspace"
args = ["-transport", "stdio"]
```

기존 `-file` 또는 위치 인자 방식은 특정 통합 문서를 항상 여는 연동을 위해 계속
지원합니다.

## 승인과 안전

읽기 도구는 read-only로 표시합니다. 통합 문서를 바꾸는 도구는 destructive로
표시해 MCP 호스트가 자체 승인 정책을 적용할 수 있게 합니다. 작업 묶음, 통합 문서,
서버 설정에 익숙해질 때까지는 일회성 승인을 권장합니다. 호스트가 승인해도 서버는
오래된 리비전, 잘못된 작업 묶음, 크기 제한을 넘은 작업을 거부합니다.

프로세스는 같은 디렉터리에서 원자적으로 교체하는 방식으로 선택한 통합 문서에
직접 변경을 저장합니다. 통합 문서 옆에는 제한된 로컬 실행 취소 이력도 만듭니다.
저장 중에는 다른 편집기를 닫고 중요한 파일은 별도 백업을 유지하세요.

검증된 경계는 [호스트 호환성](host-compatibility.md),
[로컬 보안](security.md), [형식 지원](format-support.md),
[XLSX 복구](xlsx-recovery.md)를 참고하세요.
