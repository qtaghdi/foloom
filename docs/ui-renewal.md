# 문서 편집 UI 개선

전체 추적: #41. 커밋 지침: #39, PR #40.

## 문제와 작업 순서

| 순서 | 이슈 | 확인한 문제 | 완료 기준 |
| --- | --- | --- | --- |
| 1 | #42 | 형식별 header·색상·버튼 체계 분리 | 공통 문서 shell과 디자인 tokens |
| 2 | #43 | 기본 ribbon 의존과 중복 이력 | 실제 선택과 연결된 자체 XLSX 도구 |
| 3 | #44 | DOCX의 run 입력 폼 형태 | 용지·문단·표 중심 작성과 draft 보존 |
| 4 | #45 | 잠깐 표시되는 AI 상태 | 실제 응답·event 기반 session 활동 |
| 5 | #46 | 셀을 가리는 warning overlay | inline 안내와 안전한 복구·focus |
| 6 | #47 | 한국어·영어 혼재 | OSS locale와 사용자 메시지 통일 |
| 7 | #48 | 반응형·keyboard·긴 문서 검증 부족 | 세 viewport와 실패·저장 회귀 증거 |

각 기능은 해당 테스트·설명·변경 이력을 함께 독립 커밋으로 기록합니다. 전체 변경은
공통 디자인에 의존하므로 하나의 통합 PR에서 기능별 커밋을 검토할 수 있게 합니다.
서로 다른 도구·문서 개선을 한 커밋에 묶지 않습니다.

## 디자인 기준

Image Gen으로 만든 XLSX와 DOCX 전체 화면 reference를 기준으로 합니다. concept은
실제 화면에 사용하는 raster asset이 아니며 저장소 밖에 보관합니다. 실서비스 UI는
Solid와 semantic HTML로 구현합니다.

- 흰색 header·toolbar, slate-100 작업 바탕, slate-900 본문, indigo-600 강조색
- slate-200 divider, 6px control radius, 44px 이상 touch target
- 공통 header: 제품명·문서명·형식, 저장 상태, 활동 버튼
- 형식별 toolbar: 지원하는 동작만 표시하고 unavailable 상태는 명확히 구분
- 본문: spreadsheet engine 또는 문서 용지. 오른쪽 활동 rail은 접을 수 있게 구성
- 알림: 본문을 가리지 않는 inline 안내. 실패·충돌·미저장 상태를 조용히 숨기지 않음
- 문서 text·서식·표는 원본 내용을 표시하며 concept 예시 데이터를 문서에 주입하지 않음

concept에서 강조한 title 크기는 원본 서식과 다를 수 있으므로 문서 본문에 임의의
heading 서식을 적용하지 않습니다. 페이지처럼 보이는 용지는 읽기 배치이며 native
pagination을 뜻하지 않습니다. 원본 보존과 controller의 저장 권위는 유지합니다.

## 감사 기준

keyboard·focus 표시와 복귀, 의미 있는 label, live status, 긴 제목·오류 줄바꿈,
축소 모션, touch target, 모바일 overflow, 큰 문서 표시 비용을 검사합니다.
문서 text는 HTML로 해석하지 않고, 활동에는 문서 내용·token을 보관하지 않습니다.

## 검증

기능별 Solid DOM·controller 검사와 실제 browser의 저장·reopen·복구를 확인합니다.
최종 검사는 `make verify web-test compatibility-test package-test package-smoke`를
사용합니다. 실제 호스트와 native Office 검증은 실행된 범위만 주장합니다.
