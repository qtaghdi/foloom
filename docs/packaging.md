# 패키징 검증

릴리스 실행 파일에는 별도로 빌드한 두 인터페이스가 포함됩니다.

- 인증된 브라우저 편집기용 `index.html`과 해시가 붙은 assets
- 독립 실행형 MCP App 리소스인 `app.html`

깨끗한 checkout에서 전체 패키징 검사를 실행합니다.

```bash
make package-verify
```

고정된 pnpm lockfile을 설치하고 두 인터페이스를 빌드 및 검증합니다. Go 실행
파일을 컴파일하고 공백이 포함된 절대 경로의 XLSX 복사본을 열어 두 내장 번들의
제공 경로를 검사합니다. GitHub Actions도 Pull Request와 `main` 푸시에서 깨끗한
checkout으로 같은 검사를 실행합니다.

`make build`는 `bin/document-workspace`를 컴파일하기 전에 두 인터페이스를 다시
빌드하고 검증합니다. `make go-build`는 개발용 Go 전용 명령이며 누락된
프론트엔드 출력을 만들지 않습니다.

## 실패 동작

`make package-check`는 진입 파일이 없거나, 브라우저 진입점이 없거나 비어 있는
해시 asset을 참조하거나, MCP App 진입점이 독립 실행형이 아니면 해결 방법을
포함한 오류로 실패합니다. 회귀 테스트는 생성 출력 누락, 오래된 브라우저 참조,
완전한 fixture를 다룹니다.

해시 assets와 `app.html`은 빌드 출력이므로 계속 무시합니다. 브라우저
`index.html`은 해시 참조 변경을 검토할 수 있도록 추적합니다. Go 컴파일러가
내장 디렉터리를 가져가기 전에 항상 `web-build`를 실행해야 합니다.

## 알파 릴리스 압축 파일

내장 인터페이스를 빌드한 뒤 현재 플랫폼용 릴리스 테스트를 실행합니다.

```bash
make release-package-test VERSION=0.1.0-alpha.local
```

압축 파일 하나를 만들고 SHA-256 체크섬과 필수 내용을 검증합니다. 압축을 풀어
내장 버전을 확인하고 전송 계층을 시작하지 않은 채 XLSX fixture 복사본을
검사합니다.

전체 플랫폼 압축 파일은 다음 명령으로 만듭니다.

```bash
make release-verify VERSION=0.1.0-alpha.local
```

`dist/`에는 AMD64와 ARM64용 macOS, Linux, Windows 미서명 압축 파일과
`SHA256SUMS`가 생성됩니다. 각 압축 파일에는 실행 파일, README, 시작 안내, 형식
지원표, 호스트 호환성 안내, 로컬 보안 안내가 들어갑니다. Go 바이너리는 앞서
빌드한 브라우저 및 MCP App 번들과 설치할 `document-xlsx` skill을 포함합니다.

`Private alpha package` workflow는 권한 있는 사용자의 수동 실행으로만 동작합니다.
비공개 저장소 범위의 workflow artifact를 올리고 저장소 내용에는 읽기 권한만
가집니다. tag로 실행되지 않으며 GitHub Release를 만들 수 없습니다. 공개 배포를
다시 도입하려면 별도 이슈, 소유자 결정, 배포 조건, 정책 검토가 필요합니다.

알파 결과물은 서명되거나 notarize되지 않았습니다. macOS 및 Windows 설치
프로그램, 자동 업데이트, 파일 선택기, 호스팅 서비스를 포함하지 않습니다.
[시작 안내](getting-started.md)는 운영체제 보안을 전역으로 우회하라고 권하지
않으면서 지원되는 명령줄과 MCP 호스트 흐름을 설명합니다.

## 빌드 정보와 사전 검사

릴리스 빌드는 요청한 버전, 소스 commit, commit 날짜를 포함합니다. 사용자는
`document-workspace -version`으로 이를 기록할 수 있습니다. 기존 연동은
`-file`을 계속 사용하고 대화형 사용자는 통합 문서 경로 하나를 위치 인자로
전달할 수 있습니다.

`document-workspace -check workbook.xlsx`는 서비스와 같은 XLSX 세션 경계를
통해 통합 문서를 열고 구조 호환성과 수식 정책 metadata를 출력한 뒤 종료합니다.
HTTP나 stdio 전송을 시작하지 않고 셀 내용도 출력하지 않습니다.

macOS의 `document-workspace mcp install`은 현재 실행 파일을 사용자 Application
Support에 복사하고 Claude Desktop 및 Codex stdio 설정과 skill을 설치합니다.
package 자동 테스트는 실제 사용자 설정 대신 임시 홈을 사용합니다.

PR에서 변경 범위별로 패키지 검사를 선택하는 기준과 수동 전체 검사 방법은
[변경 범위별 CI](ci.md)를 참고하세요.
