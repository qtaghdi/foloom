# 변경 범위별 CI

자동 검사는 `main` 대상 PR에서 실행합니다. `main` 푸시에는 다시 실행하지
않습니다. 같은 PR에 후속 커밋이 올라오면 이전 동일 workflow 실행을 취소합니다.
비공개 알파 archive의 수동 생성 정책은 그대로 유지합니다. 패키지와 archive는
하나의 Make 실행에서 검사하여 공통 브라우저 빌드를 한 번만 수행합니다.

| 변경 범위 | 회귀 검사 | XLSX 안전성·성능 | 패키지 |
| --- | --- | --- | --- |
| 알려진 Markdown 문서·이슈 template만 변경 | 분류 규칙 검사만 | 생략 | 생략 |
| DOCX adapter·전용 HTTP·전용 UI·fixture | DOCX 검증 | 생략 | 실행 |
| 패키지 검사·archive 스크립트 | 분류 규칙 검사만 | 생략 | 실행 |
| XLSX·공통 코드·의존성·빌드·workflow·알 수 없는 경로 | 전체 검증 | 실행 | 실행 |
| 각 workflow 수동 실행 | 해당 workflow 전체 검증 | 해당 검사 실행 | 해당 검사 실행 |

경로는 `.github/scripts/check-scope.mjs`에서 분류합니다. 여러 범위가 섞이면
필요한 검사를 모두 포함합니다. 빈 diff는 전체 검증을 선택하고 diff 계산이
실패하면 CI를 실패시킵니다. rename은 삭제와 추가 경로를 모두 검사하므로
코드를 문서 폴더로 이동해도 검사를 놓치지 않습니다. PR 전체의 merge base부터
head까지 비교하며 파일 이름 개수에 따른 GitHub 경로 필터 제한을 사용하지 않습니다.

기존 workflow와 required job 이름은 유지합니다. workflow 전체의 `paths` 조건을
사용하지 않고 job 안에서 설치·검사 step만 생략하여 필수 검사가 대기 상태로
남지 않게 합니다. 분류 실패는 검사 실패로 나타납니다. 각 실행의 Summary에
실행·생략 여부를 표시합니다. 이 방식은 [GitHub 필수 검사 안내](https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks)의 workflow 생략과 job 조건 구분을 따릅니다.

`make docx-verify`는 두 편집기 asset을 빌드하고 DOCX controller, Solid DOM,
DOCX adapter와 DOCX HTTP·MCP·SSE 테스트, 해당 Go race·vet를 검사합니다.
공통 Solid DOM 검사는 두 형식의 공통 UI를 포함합니다. 패키지 검사는 두 형식을
함께 배포하므로 XLSX 실행 smoke도 포함하지만 XLSX corpus·안전성·성능 gate는
DOCX 전용 PR에서 실행하지 않습니다.

전체 재검증이 필요하면 Actions 화면에서 각 verification workflow의
`Run workflow`를 선택합니다. 수동 실행은 변경 경로와 관계없이 해당 workflow의
모든 검사를 실행합니다. 로컬에서는 다음 명령을 사용할 수 있습니다.

```sh
node --test .github/scripts/check-scope.test.mjs
make docx-verify
make verify web-test compatibility-test
make xlsx-safety-verify xlsx-performance-verify package-verify release-package-test
```

경로 분류 규칙을 변경하면 문서·DOCX·XLSX·공통·삭제·알 수 없는 경로 사례의
회귀 검사를 함께 갱신해야 합니다. workflow 변경 자체는 항상 전체 검증 대상입니다.
