# 실제 Microsoft Excel 검증

## 목적과 릴리스 경계

지원되는 desktop Microsoft Excel을 사용할 수 있을 때 이 절차를 수행합니다. 증거가
기록되기 전에는 X-07 native fidelity와 X-08 수식 기대값이 blocked입니다.
Excelize나 LibreOffice reopen은 유용하지만 Microsoft Excel을 대신하지 않습니다.

항상 복사본을 사용하고 document-workspace와 Excel을 같은 파일의 동시 writer로
실행하지 않습니다. source hash 검사만으로 비협조 앱과의 마지막 race를 제거할 수
없습니다.

## 시작 전 기록

[Excel 증거 양식](release-evidence/xlsx-native-excel-template.md)에 다음을 기록합니다.

- candidate commit과 clean working tree 여부
- OS 버전 및 architecture
- Excel 제품 version, build, license channel, architecture
- locale, 계산 mode, 외부 link 허용 여부
- fixture 경로, 출처, 재배포 권리, 크기, SHA-256
- document-workspace binary hash와 편집 transport 또는 UI mode

출처와 권리가 알려진 비민감 fixture만 사용합니다. 고객 문서, 개인정보, 인증 정보,
외부 link token, 식별 metadata를 commit하지 않습니다.

## Microsoft Excel fixture 만들기

1. clean candidate에서 `make verify`와 `make compatibility-test`를 실행합니다.
2. `feature-rich.xlsx`를 임시 디렉터리에 복사합니다. committed fixture는 열거나
   덮어쓰지 않습니다.
3. 열기 전 file size와 SHA-256을 기록합니다.
4. Excel에서 외부 link update를 물으면 기본적으로 **업데이트하지 않음**을
   선택하고 prompt와 선택을 기록합니다.
5. 수식, style, merge, validation, conditional formatting, chart, image, hyperlink,
   defined name, comment와 repair 또는 compatibility warning을 기록합니다.
6. normal `.xlsx`로 **다른 이름으로 저장**하고 macro, 암호화, legacy mode를
   사용하지 않습니다.
7. Excel을 완전히 닫고 저장 파일을 다시 열어 inventory와 차이를 기록합니다.
8. Excel 출력의 size와 SHA-256을 기록합니다. package hash 변경 자체는 실패가
   아닙니다.
9. 재배포 가능하면 `excel-macos-<major-version>.xlsx` 같은 이름으로 corpus에
   추가하고 정확한 출처, build, OS, 날짜, source 및 output hash, 차이를 기록합니다.
10. `compatibility_test.go` fixture 표에 추가하며 설명되지 않은 손실을 허용하도록
    assertion을 약화하지 않습니다.

초기 Save As에서 Excel이 기능을 제거하면 두 hash와 차이를 모두 남기고 retained
subset fixture로 쓸지 결정합니다. 생성 원본과 동등하다고 표현하지 않습니다.

## Native 편집 및 reopen 표

각 행마다 새 복사본을 사용하고 명시한 작업 하나만 실제 제품 진입점으로 수행한 뒤
프로세스를 종료하고 Excel에서 다시 엽니다.

| 사례 | 제품 작업 | Excel 확인 |
| --- | --- | --- |
| 셀 | 관련 없는 값 하나 변경 | 한 번만 저장, 주변 수식과 style 유지 |
| 수식 | 수식 하나 변경 | 표현식 보존, 계산 동작 기록 |
| 의존성 | 수식 입력 변경 | 전체 재계산 요청과 결과 기록 |
| 붙여넣기 | 사각 범위 | shape, 값, 수식, 원자성 유지 |
| 서식 | 지원 기본 서식 변경 | 요청 style 표시, 관련 없는 custom format 유지 |
| 병합 | 임시 범위 병합 및 해제 | 범위 상태와 관련 없는 merge 유지 |
| 행 | 삽입 및 삭제 | reference, validation, rule, drawing, 주변 data 검사 |
| 열 | 삽입 및 삭제 | reference, validation, rule, drawing, 주변 data 검사 |
| 이미지 | 기존 image 이동, 크기, 삭제 | 위치와 크기 저장, 관련 없는 media 유지 |
| 차트 | 기존 chart 이동, 크기, 제목 변경 | title과 위치 저장, type, series, axis, legend, style 유지 |
| 이력 | commit, undo, redo | 최종 상태가 한 번 정확히 reopen |
| 충돌 | session 사이 외부 수정 | stale write 거부, Excel 외부 변경 유지 |

각 사례에 source 및 saved hash, 기준과 최종 revision, 정확한 작업, 예상 및 관찰
결과, Excel warning, 필요한 화면과 log, pass/fail/blocked를 기록합니다.

## 수식 검증

각 항목을 별도 복사본으로 검사합니다.

1. 편집하지 않은 기존 지원 수식
2. 변경한 수식 표현식
3. 변경한 의존 셀
4. browser preview 미지원이지만 Excel 지원 함수
5. 외부 원본이 없는 외부 workbook reference
6. 의도적으로 오래된 cached value를 가진 수식

편집 전 수식 text, 제품의 cache 상태, browser preview, Excel 표시값을 기록합니다.
편집 후 서비스를 중지하고 Excel에서 열어 자동 재계산, 보안 또는 link prompt,
수식 text, 결과를 기록합니다. 자동 계산이 꺼져 있으면 **지금 계산** 또는 전체
재계산 전에 그 상태를 기록합니다. browser preview를 Excel 결과로 쓰지 않습니다.

외부 원본을 해결하지 못하는 것은 예상 한계입니다. reference 보존, 정직한 경고,
가짜 기준값을 만들지 않는 것이 통과 조건입니다.

## 완료 규칙

X-07은 지원 편집의 의도한 변경이 모두 보존되고, inventory한 관련 없는 기능이
승인 및 기록된 제한 없이 사라지지 않을 때 통과합니다. X-08은 수식 표현식,
의존성, 미지원 함수, 외부 참조, stale cache, native 재계산의 실제 Excel 결과가
있을 때 통과합니다.

repair prompt, 조용한 기능 손실, 잘못된 수식 결과, 중복 편집, stale write 승인은
실패입니다. 실패 복사본은 Git 밖에 보관하고 비식별 증거로 이슈를 열며 Excel이
수동 repair할 수 있다는 이유로 pass 처리하지 않습니다.
