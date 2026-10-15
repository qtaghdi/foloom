# 제품 계획

## 비전

document-workspace는 Claude, Codex 등 MCP 클라이언트에서 사람과 AI가 같은 사무
문서를 편집하는 작업 공간입니다. 하나의 리비전 세션에서 활동 표시, 명시적
commit, conflict, undo, 형식별 안전 저장을 제공합니다.

모든 proprietary desktop 기능을 복제한다고 주장하지 않습니다. 원본 보존,
렌더링, 편집 호환성을 별도로 측정하고 유용하고 안전한 상호운용을 목표로 합니다.

## 제품 원칙

1. 편의 기능보다 사용자 원본 보존을 우선합니다.
2. 형식마다 typed 작업과 호환성 계약을 둡니다.
3. 리비전, 승인, 이력, presence 동작을 일관되게 유지합니다.
4. 미지원 또는 손실 가능 기능을 저장 전에 알립니다.
5. 내장 UI가 없어도 MCP 도구는 유용해야 합니다.
6. 출처가 기록된 corpus로 fidelity를 검증합니다.
7. macro, 수식, embedded content를 코드로 실행하지 않습니다.

지원은 보존, 렌더링, 편집의 세 단계로 따로 평가합니다. 한 단계 통과가 다음
단계를 뜻하지 않습니다.

## 1단계: XLSX production adapter

상태: 저장소 소유자가 통제하는 통합 문서에 사용할 수 있는 비공개 로컬 알파.

완료된 기반:

- Go 통합 문서 session, 리비전, 원자적 저장
- Streamable HTTP와 stdio MCP
- MCP App과 loopback browser fallback
- 사람 편집과 순서 있는 AI event
- 범위 붙여넣기, 서식, 병합, 행과 열 작업
- 제한된 영속 undo와 redo
- 큰 sheet viewport loading
- 일반 validation 및 conditional formatting 렌더링
- 기존 image와 chart preview 이동, 크기, 삭제, 저장
- 생성 및 LibreOffice 호환성 fixture
- Excel 전 자동 safety, regression, packaging, performance gate
- 미서명 macOS, Linux, Windows 비공개 archive pipeline
- macOS 단일 명령 MCP 설치, 동적 XLSX 열기, `document-xlsx` prompt 및 skill

안정 릴리스 전 남은 작업:

1. 출처가 기록된 Microsoft Excel fixture와 native reopen 증거
2. 실제 주장할 Claude 및 Codex 모드별 인증
3. 수식 cache와 재계산의 native Excel 증거
4. 최종 candidate의 보이는 브라우저 상호작용 재검사
5. 초대 tester의 반복 사용, 배포 조건 선택, 공개 여부에 대한 별도 소유자 결정

### 비공개 알파 종료 조건

다음을 모두 충족할 때까지 저장소와 결과물을 비공개로 유지합니다.

1. 의도한 candidate의 native Excel fidelity와 수식 증거가 있습니다.
2. 소유자와 초대 tester 최소 2명이 대표 통합 문서 복사본으로 반복 작업합니다.
3. 원본 손실, authorization, false success 결함이 남아 있지 않습니다.
4. source 및 binary 배포 조건과 dependency 의무를 검토합니다.
5. 저장소 소유자가 새 이슈와 Pull Request에서 공개 배포를 명시적으로 승인합니다.

landing page, 결제, 호스팅 계정, signed installer, 자동 update는 반복 사용 가치가
확인될 때까지 연기합니다. 첫 안정 범위는 [XLSX 체크리스트](xlsx-release-checklist.md)를
따릅니다. macro 실행, 암호화 편집, 완전한 Excel 수식 engine, 모든 proprietary
extension 보존은 약속하지 않습니다.

## 2단계: 공통 문서 플랫폼

상태: 저장소 경계는 마련됐고 두 번째 adapter 전까지 shared runtime 추출을
미룹니다.

- 형식 adapter는 `internal/formats/<format>`에 둡니다.
- 각 adapter는 inspect, typed operation, 저장, compatibility report를 소유합니다.
- 전송은 format logic 없이 adapter를 호출합니다.
- 공통 session, event, proposal 추상화는 실제 XLSX와 DOCX가 같은 계약을 입증한
  뒤에만 만듭니다.
- 현재 XLSX는 명시적인 절대 경로로 활성 문서를 전환합니다. 파일 선택기와 여러
  문서 동시 session은 신뢰할 수 있는 launcher 또는 session 생성 workflow에서
  추가합니다.

## 3단계: DOCX

첫 목표는 단순 paragraph와 table 편집, native 저장, 관련 없는 OOXML part
보존입니다. Word 및 LibreOffice fixture와 [DOCX 보존 실험](docx-preservation-prototype.md)을
통해 저장 방식을 먼저 결정합니다. 정확한 pagination, 복잡한 field, floating
layout, tracked-change 작성은 초기 범위가 아닙니다.

## 4단계: PPTX와 HWPX

PPTX는 slide text, 단순 shape, image 위치, slide 순서를 시작점으로 하고 theme,
master, note, relationship을 보존합니다. animation 작성과 embedded object는
미룹니다.

HWPX는 한글 text, paragraph 속성, 단순 table, section을 우선합니다. Hancom에서
만든 fixture와 native reopen이 필요합니다. binary HWP는 원본을 바꾸지 않는 연구
또는 변환 경로로 별도 취급합니다.

## 5단계: PDF

PDF는 page rendering, text inspection, annotation, 지원 form filling, page 구성을
제한 범위로 봅니다. word processor식 arbitrary text reflow는 목표가 아닙니다.
OCR, secure redaction, signature는 독립된 높은 위험 기능입니다. 시각 overlay를
underlying content 삭제라고 표현하지 않습니다.

## 향후 호스팅

로컬 사용에서 반복 가치가 확인된 뒤에만 hosted mode를 검토합니다. identity,
문서별 authorization, tenant 격리, encrypted object storage, immutable version,
audit, quota, background conversion, privacy 및 retention 정책이 필요합니다.

## 배포 판단

현재는 비공개 저장소와 비공개 알파 artifact만 사용합니다. 공개 source, 공개
binary, 상업 서비스는 서로 별도 결정입니다. 공개 전에는 license, third-party
notice, signing, update, support, privacy, security response, 비용과 운영 책임을
명시해야 합니다.
