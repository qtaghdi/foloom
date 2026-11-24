# 제3자 구성 요소와 소스 공개

Foloom 자체 소스는 Apache-2.0으로 제공합니다. 제3자 구성 요소는 각 원저작자의
라이선스와 고지를 유지하며 Foloom 라이선스로 재허가하지 않습니다.

| 구성 요소 | 확인된 직접 의존성 라이선스 또는 원문 |
| --- | --- |
| Univer OSS (`@univerjs/*`) | Apache-2.0 |
| MCP TypeScript client | Apache-2.0 |
| MCP ext-apps, Solid, Zod | MIT |
| MCP Go SDK | 패키지의 LICENSE에 Apache-2.0, MIT 전환 및 문서 CC-BY-4.0 조건 명시 |
| Excelize 및 Go 간접 의존성 | `go.mod`, `go.sum`에 고정된 각 모듈의 LICENSE |
| 기존 Python 검증 의존성 | `legacy/xlsx-python/requirements.txt`에 고정된 패키지의 LICENSE |

직접 및 간접 의존성의 전체 목록은 manifest와 lockfile에 기록합니다. 의존성
소스는 저장소에 vendoring하지 않으며 설치 시 각 배포본의 라이선스·고지를 따릅니다.
공개 바이너리나 설치 파일 배포 전에는 실제 번들에 포함된 버전별 제3자 라이선스와
필요한 저작권·NOTICE를 수집하여 함께 제공하고 배포 조건을 검토해야 합니다.

합성 DOCX·XLSX 검증 자료는 저장소가 작성한 비민감 자료입니다. 자료의 출처와
생성 방식은 각 `testdata` corpus 문서에 기록합니다. Foloom 이름과 로고는
상표 권리의 양도나 공식 제휴를 의미하지 않습니다.
