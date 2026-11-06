# XLSX 수식 정책

## 기능 경계

폴룸은 XLSX 수식 표현식을 저장하고 보존합니다. Go 서비스는 수식
결과를 계산하지 않습니다. 수식 옆에 반환되는 값은 원본 XLSX 패키지의 cached
value이거나 사용할 수 없는 값입니다. 서비스가 계산한 결과로 표시하지 않습니다.

브라우저는 Univer 오픈 소스 수식 엔진으로 대화형 미리보기를 제공합니다. 편집에는
유용하지만 Microsoft Excel 호환성의 기준 결과는 아닙니다. 함수 지원, 동적 배열,
외부 참조, locale 동작, 날짜, 정밀도, 계산 순서가 다를 수 있습니다.

수식에 영향을 줄 수 있는 편집 뒤에는 파일을 열거나 저장할 때 전체 native 재계산을
요청하도록 XLSX 계산 속성을 기록합니다. 강제 전체 계산 표시도 설정합니다. 기준이
되는 cached 결과 생성은 Microsoft Excel 또는 다른 native 스프레드시트 앱의
책임입니다.

## API 계약

통합 문서 snapshot은 `formulaPolicy`를 제공합니다.

- `storage: preserved`
- `serverCalculation: none`
- `browserCalculation: preview`
- `nativeRecalculation: requested_after_formula_affecting_edits`

수식 셀은 `formulaValueStatus`를 제공합니다. `cached`는 원본 패키지에서 온 값을,
`unavailable`은 cached 원본 값이 없음을 뜻합니다. 이 상태는 값의 정확성이나
최신성을 보장하지 않습니다.

수식이 있는 통합 문서에서 셀 값이나 수식 변경, 사각 범위 붙여넣기, 행 또는 열
삽입 및 삭제는 native 재계산을 요청합니다. 수식을 새로 추가해도 해당 세션의 정책과
경고가 활성화됩니다. 서식, 병합, 이미지, 차트만 바꾸면 재계산을 요청하지 않습니다.

## 사용자 경험

통합 문서에 수식이 있으면 호환성 안내에서 브라우저 결과가 미리보기임을 밝힙니다.
AI 클라이언트도 구조화된 도구 결과로 같은 정책과 수식 값 상태를 받습니다. 화면에
표시된 브라우저 결과나 cached 원본 값을 검증된 Excel 계산으로 설명하면 안 됩니다.

수식이나 의존 셀을 편집한 뒤에는 저장한 파일을 대상 스프레드시트 앱에서 다시 열고
재계산이 끝난 뒤 결과를 사용하세요. 외부 링크는 native 앱이 사용자 보안 설정에
따라 해결할 때까지 사용할 수 없을 수 있습니다.

## 검증

자동 테스트는 표현식 round-trip, 수식 값 상태, 정책 metadata, 의존성 변경 뒤 계산
속성, 새 수식 경고, 호환성 corpus를 통한 보존을 검증합니다. 이 테스트는 Microsoft
Excel과 계산 결과가 같음을 입증하지 않습니다.

남은 릴리스 증거는 Microsoft Excel로 만든 fixture와 변경한 수식, 의존성,
미지원 함수, 외부 참조, 오래된 cached 값을 실제 Excel에서 다시 여는 결과입니다.
Excel 버전과 관찰한 재계산 동작을 릴리스 증거에 기록합니다.
