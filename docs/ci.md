# 변경 범위별 CI

PR을 열거나 대상 브랜치를 바꾸거나 새 커밋을 올리면 GitHub Actions가
변경 범위를 한 번 판단합니다. 선행 브랜치를 대상으로 하는 PR도 같은 검사를
받습니다. 새 커밋이 올라오면 같은 PR의 이전 검사 실행을 취소합니다.

기능과 유지보수는 `develop` 대상 PR에서 검증하고, 릴리스는 `develop`에서
`main`으로 별도 PR을 열어 통합 상태를 검증합니다. 추가 GitHub Actions 비용은
발생시키지 않으며 로컬 검증을 우선합니다. 원격 검사 미실행은 PR에 명시하고
실행하지 않은 검사는 성공으로 기록하지 않습니다.

| 변경 범위 | 실행 검사 |
| --- | --- |
| 알려진 Markdown 문서와 이슈 양식 | PR 정책, 범위 판정 |
| DOCX adapter, 전용 HTTP·UI·테스트·fixture | DOCX, 패키지 |
| 공용 웹 UI, 스프레드시트 UI | 웹, 패키지 |
| XLSX adapter | 통합 회귀, XLSX 안전성·성능, 패키지 |
| XLSX fixture, 기존 Python 검증 코드 | 통합 회귀 |
| XLSX 성능 도구 또는 안전 검사 스크립트 | 변경된 성능 또는 안전 검사 |
| 패키지 스크립트, 생성된 공용 HTML 진입점 | 패키지 |
| 공용 Go 코드, 의존성, CI, 빌드, 미분류 경로 | 통합 회귀, XLSX 안전성·성능, 패키지 |

PR 정책과 범위 판정은 항상 실행합니다. 통합 회귀가 선택되면 중복되는 DOCX·웹
검사를 별도로 실행하지 않습니다. 패키지 검사는 두 편집기가 함께 배포되므로
두 형식의 빌드와 실행 확인을 포함합니다.

DOCX·웹·패키지 검사는 한 작업에서 선택한 Make 목표를 동시에 전달합니다.
공통 의존성 설치와 브라우저 빌드는 한 번만 실행하며, DOCX 또는 패키지의
Go 검증과 릴리스 패키지 검증은 그대로 유지합니다. 통합 회귀가 선택되면
별도 편집기 검사는 중복 실행하지 않고 필요한 패키지 검사는 유지합니다.
수동 패키지 검사는 독립 워크플로로 계속 사용할 수 있습니다.

분류 규칙은 `.github/scripts/check-scope.mjs`에서 관리합니다. PR의 merge base부터
head까지 경로를 비교하고, 이름 변경을 삭제와 추가로 나눠 분류합니다.
변경이 섞이면 필요한 검사를 합칩니다. 빈 diff는 전체 검사를 선택합니다.

`Verify selected PR checks`는 범위 판정과 선택된 검사가 모두 성공했는지
확인합니다. 선택된 검사가 실패하거나 취소되거나 생략되면 성공 처리하지
않습니다. 실행한 검사와 생략한 검사는 Actions Summary에 표시합니다.
[GitHub 필수 검사 안내](https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks)에 따라 경로 조건으로 워크플로 전체를 생략하지 않습니다.

별도로 재검증할 때는 Actions에서 기존 회귀·안전·성능·패키지 워크플로를
수동 실행할 수 있습니다. 비공개 알파 패키지 발행 정책은 그대로 적용됩니다.
로컬에서는 다음 명령을 사용할 수 있습니다.

```sh
node --test .github/scripts/check-scope.test.mjs
make docx-verify
make verify web-test compatibility-test
make xlsx-safety-verify xlsx-performance-verify package-verify release-package-test
```

분류를 바꾸면 문서, DOCX, XLSX, 공용, 혼합, 삭제, 미분류 경로 사례의 검사를
함께 갱신합니다.
