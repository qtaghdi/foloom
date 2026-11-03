# ADR 0003: 앱 소유 편집 UI에 Solid 도입

상태: 채택
날짜: 2026-11-03
관련 이슈: #37

## 배경

소유자는 React 사용 경험이 많으며 1인 개발에서 코드 길이를 최적화하기보다
Solid를 활용해 자체 UI를 만들기를 선택했습니다. 기존 화면은 TypeScript로 DOM과
상태를 수동 관리하고 있어 문서 형식과 패널이 늘수록 표시 상태를 맞출 책임이
커집니다. 프레임워크 도입 목적은 앱 UI의 컴포넌트와 반응형 표시 상태 구성입니다.
성능 우위나 DOCX 저장 호환성 향상을 주장하지 않습니다.

## 결정

Solid + TypeScript + Vite를 사용합니다. SolidStart와 별도 Node 서버는 도입하지
않습니다. Go 서버가 정적 browser bundle과 기존 self-contained MCP App을 계속
제공합니다. 두 Vite build에 동일한 Solid JSX 플러그인을 사용합니다.

앱 소유 UI의 경계는 다음과 같습니다.

- `WorkbookShell`: XLSX 문서 이름, 저장 상태, history, AI presence, 경고·복구 panel
- `AppView`: controller에서 받은 표시 상태를 Solid signal로 발행하는 view adapter
- `DocumentApp`, `DocumentContent`: DOCX 도구 모음, 경고, 본문과 표 run 입력, 복구 확인
- `createDocumentView`: DOCX controller의 view 계약과 Solid 표시 상태 연결
- `StatusPanel`: 접근 가능한 접기·펼치기·닫기와 보호된 복구 action

`WorkbookController`와 `DocumentController`는 리비전, 쓰기, 초안과 충돌 복구를
소유합니다. 화면 signal을 저장의 진실로 사용하지 않습니다. 외부 payload는 기존
Zod 검증을 통과한 뒤 화면 상태에 들어옵니다. 문서 text는 JSX text와 textarea
value로 표시하며 HTML로 해석하지 않습니다.

`SpreadsheetEngine`은 유지합니다. Solid는 grid container의 lifecycle과 바깥 UI만
소유하고, container 내부 DOM은 Univer가 소유합니다. 저장 상태나 presence 변경으로
엔진을 다시 만들지 않습니다. Univer 내부의 React 의존성과 기본 grid UI는
유지합니다. 이번 변경은 Univer 전체 UI의 자체 구현을 뜻하지 않습니다.

## Lifecycle와 검증

Solid root를 해제할 때 controller, SSE 구독, engine, timer와 window listener를
정리합니다. 연결과 초기 읽기가 진행 중이어도 해제된 화면에 engine을 늦게 만들지
않습니다. 취소된 페이지 이탈은 controller를 해제하지 않습니다. 저장이 확인된
이력 복원 뒤의 재열기는 미저장 복구 상태와 구분해 불필요한 이탈 확인을 막습니다.
실제 대기 중인 쓰기나 불확실한 실패 상태의 경고는 유지합니다.

DOCX draft 변경은 snapshot을 다시 만들지 않으므로 입력 element와 focus를
유지합니다. 저장된 새 snapshot을 받은 뒤 최신 ID로 다시 표시합니다. 충돌·연결
실패에서는 기존 draft를 유지하고 저장을 차단합니다. draft 폐기 확인은 자체 Solid
UI로 제공하며, 명시적으로 선택한 뒤에만 다시 불러옵니다.

기존 Node controller 회귀 테스트에 더해 Vitest와 happy-dom에서 실제 Solid DOM
컴포넌트를 실행합니다. 경고의 text escaping, history·복구 접근성, engine DOM 보존,
DOCX 일괄 저장·focus·충돌·실패, root 해제와 지연 연결을 검사합니다. 실제 브라우저,
세 가지 화면 크기, Go 통합·race·vet, Python 회귀와 패키지 검사도 유지합니다.

## 대안

React는 기존 Univer 의존성과 일부 editor 연동에서 이점이 있지만 소유자의 선택을
우선했습니다. Svelte와 Vue 역시 가능하지만 이번 요구에는 Solid를 선택했습니다.
일반 TypeScript DOM 구현은 작은 화면에서는 단순하나 반복되는 표시 상태와 UI
lifecycle 관리를 컴포넌트로 옮기는 이점을 얻기 위해 변경합니다.

## 후속 범위

Univer 도구 모음·서식·sheet 탐색의 자체 UI 대체는 지원 API와 기능 검증을 거쳐
별도로 진행합니다. DOCX 편집 엔진, 문서 구조·서식 범위 확대, 원본 보존과 native
검증은 독립된 작업입니다. Solid 도입만으로 이 기능들이 구현되지는 않습니다.
