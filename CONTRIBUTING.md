# 기여 가이드

## 저장소 모델

폴룸은 여러 언어를 사용하는 모노레포입니다. Go는 형식 세션,
저장, 리비전 제어, 실시간 이벤트, MCP 도구를 담당합니다. TypeScript는 브라우저
상호작용 화면을 담당합니다. 형식 계약을 바꿀 때는 영향을 받는 모든 경계를 함께
조정해야 합니다.

Univer 오픈 소스 패키지만 허용합니다. 저장소 소유자가 별도의 아키텍처 결정을
승인하지 않는 한 Univer Pro 패키지와 서비스는 사용할 수 없습니다.

## 필수 작업 흐름

문서, 유지보수, CI, 작은 수정까지 모든 변경은 이슈와 Pull Request를 사용합니다.
`main`에 직접 커밋하거나 푸시하지 않습니다.

1. 같은 문제와 범위의 열린 이슈가 있는지 검색합니다.
2. 문제, 목표, 완료 조건, 범위, 제외 항목, 위험, 필요한 증거를 이슈에 기록합니다.
3. 최신 `main`에서 이슈 번호를 포함한 브랜치를 만듭니다.
4. 목적이 분명한 커밋을 만들고 범위가 바뀌면 이슈를 갱신합니다.
5. 저장소 양식과 `Closes #<number>`를 사용해 Pull Request를 엽니다.
6. 검증 결과, 호환성 및 보안 영향, 수동 확인, 후속 작업을 기록합니다.
7. 필수 검사가 통과하고 저장소 소유자가 승인한 뒤에만 병합합니다.

관련 없는 작업을 발견하면 별도 이슈와 Pull Request를 만듭니다. 작은 변경은 짧은
이슈를 쓸 수 있지만 이 절차를 생략할 수는 없습니다.

## 브랜치

다음 형식의 소문자 브랜치 이름을 사용합니다.

```text
<type>/<issue-number>-<short-description>
```

예시:

```text
feat/42-ai-presence
fix/57-revision-conflict
docs/61-oss-architecture
```

커밋에서 허용하는 `feat`, `fix`, `docs`, `refactor`, `test`, `build`, `ci`,
`chore`, `perf`, `revert`, `security` 유형을 사용합니다.

## 커밋

Conventional Commits 형식과 영문 명령형 제목을 사용합니다.

```text
<type>(<scope>): <summary>
```

권장 범위는 `go`, `web`, `mcp`, `xlsx`, `realtime`, `docs`, `build`,
`deps`입니다.

예시:

```text
feat(web): render AI range presence
fix(workbook): reject stale batch revisions
docs(architecture): define OSS collaboration path
build(web): update embedded assets
```

제목은 72자 이내로 쓰고 마침표로 끝내지 않습니다. 변경 이유, 마이그레이션,
호환성 위험, 운영 영향이 diff만으로 분명하지 않으면 본문을 작성합니다.

계약을 깨는 변경에는 `!`와 `BREAKING CHANGE:` 푸터를 사용합니다.

```text
feat(mcp)!: replace cell edit payload

BREAKING CHANGE: apply_operations now requires operation IDs.
```

## 개발

브라우저 의존성 설치:

```bash
make web-install
```

표준 검증 실행:

```bash
make verify
```

내장 UI와 Go 바이너리 빌드:

```bash
make build
```

프로덕션 의존성 감사:

```bash
make audit
```

## Pull Request

- 연결된 이슈 항목에 `Closes #<number>`를 넣어 범위를 정의한 이슈 하나를 정확히
  연결합니다.
- Conventional Commits 형식의 제목을 사용합니다.
- 구현 세부 사항보다 사용자 또는 시스템 문제를 먼저 설명합니다.
- 포함 범위와 제외 범위를 명시합니다.
- 검증 명령과 결과를 기록합니다.
- XLSX 호환성, 리비전, 보안, MCP 계약 영향을 밝힙니다.
- 중요한 변경은 `CHANGELOG.md`에 기록합니다.
- 시스템 경계가 바뀌면 아키텍처 문서를 갱신합니다.
- `internal/transport/httpapi/static/assets`의 생성 파일은 커밋하지 않습니다.
- 릴리스 바이너리는 프론트엔드를 먼저 생성해 Go 바이너리에 포함하도록
  `make build`로 만듭니다.
- 관련 없는 정리를 Pull Request에 섞지 않습니다.
- 작업이 바뀌면 이슈와 Pull Request 설명도 갱신하고, 미룬 작업이나 관련 없는
  작업은 후속 이슈로 분리합니다.

## 문서와 텍스트

사용자와 저장소 소유자가 읽는 문서, 변경 이력, 이슈 양식, Pull Request 양식,
제품 문구는 한국어를 사용합니다. AI 전용 문서인 `AGENTS.md`와
`.agents/skills/**`, 코드 주석, 커밋 메시지는 영어를 유지합니다. 명령어, API,
고유 기술 용어는 정확성을 위해 영어로 둘 수 있습니다. em dash 문자는 사용하지
않습니다.
