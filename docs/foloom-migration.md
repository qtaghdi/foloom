# 폴룸 이름 전환

## 바뀌는 이름

| 항목 | 이전 | 현재 |
| --- | --- | --- |
| 제품 | document-workspace | 폴룸 · Foloom |
| GitHub | `qtaghdi/document-workspace` | `qtaghdi/foloom` |
| Go module | `github.com/qtaghdi/document-workspace` | `github.com/qtaghdi/foloom` |
| 실행 명령 | `document-workspace` | `foloom` |
| 소스 진입점 | `cmd/document-workspace` | `cmd/foloom` |
| 로컬 빌드 | `bin/document-workspace` | `bin/foloom` |
| 릴리스 | `document-workspace_<version>_<os>_<arch>` | `foloom_<version>_<os>_<arch>` |
| Claude MCP 키 | `document-workspace` | `foloom` |
| Codex MCP 키 | `document_workspace` | `foloom` |
| 호스트 스킬 | `document-workspace` | `foloom` |

## 소스 checkout

GitHub 저장소 이름 변경 뒤 기존 URL은 GitHub의 리디렉션을 사용할 수 있지만,
새 origin을 권장합니다. 로컬 폴더 이름은 자유이며 현재 Codex 작업 공간의 경로는
자동으로 이동하지 않습니다.

```bash
git remote set-url origin https://github.com/qtaghdi/foloom.git
make build
./bin/foloom -version
```

이전 `bin/document-workspace`가 남아 있어도 새 빌드 결과가 아닙니다. 새 명령을
사용하고 자체 스크립트의 명령 경로·archive 이름·Go import를 갱신하세요.
저장소 visibility와 기존 이슈·PR·브랜치·이력은 유지합니다.

## macOS MCP 설치

진행 중 문서를 저장하고 호스트에서 실행 중인 작업이 끝난 뒤 새 바이너리로 실행합니다.

```bash
./bin/foloom mcp install
./bin/foloom mcp status
```

설치 위치는 `~/Library/Application Support/foloom/bin/foloom`입니다. 설치기는
이전 Claude 서버 키와 Codex 관리 구간을 새 `foloom` 등록으로 바꿉니다. 그 외
호스트 설정은 유지하고 변경 전 설정을 `.document-workspace.bak`으로 백업합니다.
손상된 이전·현재 관리 구간이나 올바르지 않은 JSON은 쓰기 전에 거부합니다.

원래 배포 내용과 일치하는 `document-workspace` 스킬 파일만 정리하고 사용자 수정본은
남깁니다. 기존 `foloom` 경로에 사용자 수정 스킬이 있으면 덮어쓰지 않고 설치를
중단합니다. 이전 설치 디렉터리·실행 파일은 자동 삭제하지 않아 수동 연결과 실행 중
프로세스가 끊기지 않게 합니다. 전환을 확인한 후 소유자가 별도로 정리할 수 있습니다.

Claude Desktop과 Codex를 재시작하면 `foloom` MCP 연결과 `$foloom` 스킬을 사용합니다.
이전 호스트 프로세스는 실행 중인 바이너리와 설정을 즉시 교체하지 않습니다.
`foloom mcp uninstall`은 새 설치를 제거하며 사용자 수정 스킬은 유지합니다.

## 호환성을 위해 유지하는 식별자

다음은 제품 표시 이름이 아니라 기존 저장 자료와 연결의 식별자입니다.

- DOCX의 `.document-workspace.lock`과 임시 저장 파일 접두사
- XLSX의 `.xlsx-viewer-history`와 기존 lock·임시 저장 규칙
- 브라우저 `document-workspace:locale:v1`, `document-workspace:recent:v1`
- MCP App `ui://document-workspace/xlsx/v1.html`
- 기존 `document-xlsx` MCP 프롬프트 이름
- 엔진 내부 명령·개체 식별자

기존 문서 이력·동시 writer 잠금·설정·최근 경로를 이름 변경으로 잃지 않도록
유지합니다. 사용자 문서의 제목·본문·수식·서식은 브랜드 치환 대상이 아닙니다.
과거 변경 이력·계획·native 검증 증거와 Python MVP에는 당시 이름을 남깁니다.

이 작업은 도메인 구매·상표 등록·공개 배포를 포함하지 않습니다. `foloom.com`은
등록된 도메인이므로 제품 링크나 소유한 주소로 사용하지 않습니다.

## 이번 변경의 확인 결과

- 프론트엔드 77개 회귀 검사, OSS·TypeScript 검사, Go 전체·race·vet와 Python 회귀 통과
- XLSX 호환성 corpus, 내장 자산·비공개 정책·현재 플랫폼 패키지·설치·해제 검사 통과
- 이전 MCP 등록·기본 스킬 전환, 사용자 수정 스킬 보존, 손상된 관리 구간의 사전 거부 확인
- 실제 1440×900, 1024×768, 390×844 브라우저에서 브랜드·언어·가로 넘침 확인
- XLSX 저장 확인, DOCX 한국어 입력·저장·다운로드와 내려받은 OOXML 내용 유지 확인
- GitHub 저장소 이름·설명과 origin을 전환하고 비공개 상태를 유지

실제 사용자 호스트 재시작과 native Word·Excel 검증은 이번 UI·브랜드 확인에
포함하지 않습니다. 운영 중 호스트 설정은 이 PR이 병합된 뒤 새 설치 명령으로
전환할 수 있습니다. 현재 사용자의 실행 중 MCP 프로세스는 교체하지 않았습니다.
