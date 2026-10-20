# 실제 Microsoft Excel 증거

상태: 미실행. 사용 전에 날짜가 포함된 Markdown 파일로 복사하세요.

## Candidate와 환경

- Candidate commit:
- Clean working tree:
- 기록 시각:
- 검토자:
- 운영체제와 architecture:
- Excel 제품 version과 build:
- Excel license channel과 architecture:
- Locale:
- 계산 mode:
- 외부 link 정책:
- document-workspace binary SHA-256:
- 제품 진입점과 transport:

## Fixture 출처

- Fixture filename:
- 원본 application과 version:
- Original source:
- 생성 또는 Save As 날짜:
- 재배포 권한:
- 민감 데이터 검토:
- 원본 크기와 SHA-256:
- Excel 생성 파일 크기와 SHA-256:
- 기존 Excel warning 또는 normalization:

## 기능 목록

| 기능 | Excel Save As 전 | Excel Save As와 reopen 후 | 메모 |
| --- | --- | --- | --- |
| 수식과 외부 reference | 미실행 | 미실행 | |
| Style과 number format | 미실행 | 미실행 | |
| 병합 셀 | 미실행 | 미실행 | |
| Data validation | 미실행 | 미실행 | |
| Conditional formatting | 미실행 | 미실행 | |
| Chart | 미실행 | 미실행 | |
| Image | 미실행 | 미실행 | |
| Hyperlink | 미실행 | 미실행 | |
| Defined name | 미실행 | 미실행 | |
| Comment | 미실행 | 미실행 | |
| Excel repair 또는 compatibility prompt | 미실행 | 미실행 | |

## 편집과 reopen 결과

| 사례 | Source hash | 작업과 revision | 예상 | Excel 관찰 | 상태 | 증거 |
| --- | --- | --- | --- | --- | --- | --- |
| 셀 | | | | 미실행 | Blocked | |
| 수식 | | | | 미실행 | Blocked | |
| 의존성 | | | | 미실행 | Blocked | |
| 붙여넣기 | | | | 미실행 | Blocked | |
| 서식 | | | | 미실행 | Blocked | |
| 병합과 해제 | | | | 미실행 | Blocked | |
| Validation 표시 속성 | | | 원본 속성 명시 여부와 값, dropdown, input 및 error message 유지 | 미실행 | Blocked | |
| 행 | | | | 미실행 | Blocked | |
| 열 | | | | 미실행 | Blocked | |
| 이미지 | | | | 미실행 | Blocked | |
| 차트 | | | | 미실행 | Blocked | |
| Undo와 redo | | | | 미실행 | Blocked | |
| 외부 충돌 | | | | 미실행 | Blocked | |

## 수식 결과

| 사례 | 수식 text | 이전 cache 상태 | Browser preview | Excel prompt와 재계산 | Excel 결과 | 상태 |
| --- | --- | --- | --- | --- | --- | --- |
| 기존 수식 | | | | 미실행 | 미실행 | Blocked |
| 변경 수식 | | | | 미실행 | 미실행 | Blocked |
| 변경 의존성 | | | | 미실행 | 미실행 | Blocked |
| Browser 미지원 함수 | | | | 미실행 | 미실행 | Blocked |
| 사용할 수 없는 외부 reference | | | | 미실행 | 미실행 | Blocked |
| Stale cached value | | | | 미실행 | 미실행 | Blocked |

## 차이와 제한

- Repair prompt:
- 관련 없는 기능 차이:
- 예상 producer normalization:
- 승인한 제한과 연결 이슈:
- 미해결 실패:

## 승인

- X-07 native fidelity: Blocked
- X-08 수식 기대값: Blocked
- 검토자 결론:
- 후속 이슈:
