# 형식별 지원표

기준: 2026-10-08, 구현 commit `4a6cfeb`.
이 문서는 현재 기능과 초기 제공 범위이며 모든 문서의 완전 호환 선언이 아닙니다.
공통 요구사항은 [제품 명세](product-spec.md)를 따릅니다.

## 상태와 증거 규칙

- 일부 구현: 제한된 기능이 있으며 남은 한계가 있음
- 계획: 제품 범위로 선택했지만 구현 없음
- 연구: 가능성과 범위가 아직 결정되지 않음
- 연기: 초기 제공 범위에서 의도적으로 제외

지원으로 올리려면 작업 이름, 실제 진입점, 대표 fixture, 재현 가능한 결과가
필요합니다. 라이브러리 기능만으로 제품 지원이 되지 않습니다. AI 도구와 사람
editor의 진입점도 따로 기록합니다.

## 현재 제품 지원

현재 프로세스는 기존 XLSX 하나만 엽니다. 나머지 형식은 아직 제품에서 사용할 수
없습니다.

| 형식 | 열기와 검사 | 새로 만들기 | 렌더링 | 편집 및 native 저장 | 관련 없는 내용 보존 | 변환 및 export |
| --- | --- | --- | --- | --- | --- | --- |
| XLSX | 일부 구현 | 계획 | 일부 구현 | 일부 구현 | corpus 범위 일부 구현 | 계획 |
| DOCX | 계획 | 계획 | 계획 | 계획 | 계획 | 계획 |
| PPTX | 계획 | 계획 | 계획 | 계획 | 계획 | 계획 |
| HWPX | 계획 | 계획 | 계획 | 계획 | 계획 | 연구 |
| binary HWP | 연구 | 연기 | 연구 | 연구 | 연구 | 연구 |
| PDF | 계획 | export pipeline으로 계획 | 계획 | 제한 작업 계획 | 계획 | 연구 |

PDF export engine은 선택되지 않았습니다. PDF 생성은 새 출력을 만드는 것이며 word
processor형 빈 PDF editor를 약속하지 않습니다. DOC, XLS, PPT, macro 형식, ODF,
CSV 등은 이 초기 표 밖에 있으며 별도 결정을 거쳐야 합니다.

## 형식별 첫 범위

| 형식 | 첫 유용한 제공 | 보존하거나 경고할 내용 | 연기 또는 미결정 |
| --- | --- | --- | --- |
| XLSX | 기존 통합 문서의 셀, 수식, 기본 서식, 범위, 시트 크기, 기존 drawing | 손대지 않은 수식, 서식, validation, conditional formatting, chart, image, link, name, comment | 이미지 삽입, 고급 차트, 전체 수식 계산, macro, 암호화 |
| DOCX | 단순 문서 생성, paragraph/run/table 검사와 편집, native 저장 | section, header/footer, comment, note, media, relationship, unknown part | 정확한 pagination, 변경 추적 작성, 복잡한 field와 floating layout, engine 선택 |
| PPTX | 기본 deck 생성, slide 검사, text와 단순 shape, image 이동 및 크기, slide 순서 | theme, master, note, relationship, 미지원 animation metadata | animation 작성, embedded application object, 전체 재생 fidelity |
| HWPX | 기본 문서 생성, section, text, 단순 table, native 저장 | paragraph 및 문자 속성, control, media, untouched package | 복잡한 layout과 control, engine, proprietary extension |
| binary HWP | 복사본에서 검사 또는 변환 가능성 prototype | 평가 중 원본 binary 무변경 | native 편집, 생성, round-trip 보장 |
| PDF | page 렌더링, text 검사, annotation, 지원 form, page 구성 | 검증 범위의 page content, annotation relationship, form value | 임의 text reflow, OCR, 보안 redaction, signature, engine |

보존 항목은 무조건 약속이 아니라 검증 의무입니다. 보존할 수 없으면 제한을 알리고
안전하지 않은 native overwrite를 거부합니다. 시각 overlay를 underlying content의
보안 삭제로 표현하면 안 됩니다.

## XLSX 상세 기준

| 기능 | 현재 동작 | 남은 증거 또는 작업 |
| --- | --- | --- |
| 셀, 수식, 범위, 기본 서식 | 사람 및 AI 작업 저장, cached value와 browser preview 구분, native 재계산 요청 | Excel fixture와 native 재계산 증거 |
| 행과 열 삽입 및 삭제 | 사람 editor와 AI 작업 경로 | 구조 편집 fixture 확대 |
| 이미지 | 기존 이미지 표시, 이동, 크기, 삭제 저장 | 이미지 삽입 |
| 차트 | 선택 유형 preview, 이동, 크기, 삭제, AI 제목 변경 | type, series, axis, legend, style, 렌더링 확대 |
| validation 및 conditional formatting | 일반 규칙 표시 및 동작 | 고급 규칙과 작성 범위 |
| undo와 redo | 서버 소유, 리비전 기반, 제한된 영속 이력 | 무제한 이력이나 audit system이 아님 |
| 보존 | 생성 및 LibreOffice fixture에서 관련 없는 편집과 reopen | Microsoft Excel fixture와 더 넓은 corpus |
| 데스크톱 호스트 | MCP 전송, browser-first editor, 선택적 app resource | 호스트별 모드 인증 |

증거는 [XLSX corpus](../testdata/xlsx/compatibility/README.md),
[corpus tests](../internal/formats/xlsx/compatibility_test.go),
[작업 계약](../internal/formats/xlsx/types.go),
[전송 테스트](../internal/transport/httpapi/server_test.go),
[호스트 인증](host-compatibility.md)을 참고하세요.

## 변환 정책

일반 변환 경로는 아직 구현하거나 인증하지 않았습니다. 각 방향마다 원본, 대상,
engine, license, 예상 손실, 검증 증거를 기록한 뒤 노출합니다. 외부 service를 쓰면
별도의 데이터 처리 결정이 필요합니다.

| 후보 | 상태 | 주요 인수 기준 |
| --- | --- | --- |
| DOCX에서 PDF | 계획 | page layout, font, table, image, 선택 가능한 text |
| PPTX에서 PDF | 계획 | slide 크기, object 위치, font, animation 손실 고지 |
| XLSX에서 PDF | 계획 | print area, scaling, page break, 수식 결과 정책 |
| HWPX에서 PDF | 연구 | 한글, font 가용성, table layout, 가능한 engine |
| HWP에서 HWPX | 연구 | 가능성, 원본 보존, 손실 보고 |
| PDF에서 편집 가능한 DOCX/HWPX | 연기 | 원본 복원이 아닌 파생 문서임을 명시 |

## Fixture와 릴리스 Gate

각 fixture에 출처, 생성 앱과 버전, 재배포 권리, 내용 목록, 예상 결과를 기록합니다.
비공개 사용자 문서를 corpus에 넣지 않고 항상 복사본을 사용합니다.

| 형식 | 필수 corpus | 초기 release gate |
| --- | --- | --- |
| XLSX | 생성기, LibreOffice, Excel 파일과 수식, drawing, 고급 규칙, 대형 sheet | 지원 편집의 save 및 reopen, 관련 없는 기능 보존, 호스트 workflow 기록 |
| DOCX | Word와 LibreOffice, 한글 및 Latin, table, section, note, media | Word에서 단순 생성 및 구조 편집 reopen, untouched part 보존, 차이 기록 |
| PPTX | PowerPoint와 LibreOffice, theme, master, 혼합 text, media, note | PowerPoint에서 지원 object 및 slide 편집 reopen, relationship 보존 |
| HWPX | 버전이 알려진 Hancom 파일, 한글, table, section, control | Hancom Office에서 생성과 제한 편집 reopen, 미지원 control 명시 |
| binary HWP | 허가된 Hancom fixture | 지원 승격 전 연구 보고, 원본 쓰기 금지 |
| PDF | text, scan, annotation, form, restricted 예시 | 독립 viewer에서 지원 작업 reopen, 시각 및 구조 검사 |

모든 adapter에서 잘못된 입력, 거부 batch, stale revision, 저장 실패, resource 제한,
미지원 내용 옆의 관련 없는 편집을 검증합니다. 구조와 화면 결과를 따로 비교하고
fixture별 시각 tolerance를 결과 확인 전에 정합니다.
