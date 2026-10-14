# DOCX 원본 보존 프로토타입 명세

상태: 실험 설계. 이 문서는 DOCX 어댑터, fixture corpus, 엔진 선택, 실험 성공을
주장하지 않습니다. 관련 계약은 [제품 명세](product-spec.md),
[형식별 지원표](format-support.md), [아키텍처](architecture.md)를 따릅니다.

## 확인할 결정

제한된 구조 편집을 DOCX로 저장한 뒤 관련 없는 내용을 유지하고 Word에서 복구
메시지 없이 열 수 있는지 확인합니다. 동일한 입력으로 다음 두 방식을 비교합니다.

1. 손대지 않은 package/XML을 유지하는 표적 수정
2. 문서를 가져와 다시 직렬화하는 상위 수준 document model

두 방식은 실험 후보이며 선택된 라이브러리가 아닙니다. 유료 서비스나 외부 변환
업로드를 사용하지 않습니다.

## 제한된 작업

| 작업 | 대상 | 필수 동작 |
| --- | --- | --- |
| `replace_run_text` | 본문 paragraph의 일반 text run 하나 | 지정 text만 교체하고 서식과 주변 run 유지 |
| `replace_cell_text` | 병합되지 않은 단순 table cell의 text run 하나 | table과 cell 속성을 다시 만들지 않고 text 교체 |
| `create_basic_document` | 새 출력 경로 | paragraph와 단순 table 생성, 기존 목적지는 거부 |

생성은 별도 실험입니다. template, paragraph 분할, 여러 run을 넘는 교체, list,
병합 및 중첩 table, 변경 추적 작성, field, floating image, pagination, PDF 변환은
제외합니다. 미지원 구조 옆의 지원 대상만 바꿀 수 있으며 미지원 구조 자체 편집은
명시적으로 거부합니다.

## 대상 식별과 리비전

검사는 paragraph, run, table, cell에 세션 범위의 opaque ID를 반환합니다. 명령은
기준 리비전, 대상 ID, 예상 이전 text, 새 text를 포함합니다. 반복되는 text가
정상이므로 문자열 검색만으로 대상을 찾지 않습니다.

ID는 검사한 리비전에서만 유효합니다. 성공한 commit 뒤에는 다시 검사해야 합니다.
오래된 리비전, 없는 대상, 예상 text 불일치, 미지원 대상이 있으면 전체 batch를
거부합니다. 이후 브라우저도 MCP와 같은 작업 model을 사용해야 합니다.

## Fixture 계획

구현을 시작할 때 `testdata/docx/compatibility`에 manifest를 만듭니다.

| ID | 출처 | 내용과 목적 |
| --- | --- | --- |
| D-01 | 버전이 기록된 생성기 | 최소 paragraph와 단순 table |
| D-02 | 정확한 버전의 Microsoft Word | 한글과 Latin text, 혼합 run, 공백, 반복 text, 단순 table |
| D-03 | 정확한 버전의 Microsoft Word | section, header/footer, note, comment, image, link, field, 변경 추적과 격리된 일반 대상 |
| D-04 | 정확한 버전의 LibreOffice | 같은 지원 대상을 다른 producer로 검증 |
| D-05 | 통제된 비정상 입력 | 잘린 archive, 중복 및 위험 entry, 잘못된 relationship, 과도한 확장, 미지원 대상 |

각 항목에 SHA-256, producer와 버전, format variant, 날짜, 재배포 권리, 기능 목록,
대상, 예상 변경, 예상 거부를 기록합니다. 비민감 합성 자료와 복사본만 사용합니다.
Word fixture가 없으면 native 검증은 blocked로 남깁니다.

## 실험 절차

1. 원본 archive entry와 압축 해제 byte hash, relationship, 기능 수, 선택한 의미
   속성, 원본 hash, native 렌더링 기준을 기록합니다.
2. 변경 없는 import/save를 새 경로로 수행하고 독립적으로 다시 열어 비교합니다.
3. 각 지원 편집을 새 복사본에서 실행합니다. 같은 길이 교체와 긴 text 교체를
   분리합니다.
4. 새 출력에 저장하고 adapter와 Word에서 다시 열어 대상과 관련 없는 속성을
   검증합니다. LibreOffice 결과는 Word 결과를 대신하지 않습니다.
5. 저장과 다시 열기를 두 번 반복해 누적 손실을 확인합니다.
6. valid/invalid 혼합 batch, 오래된 대상, 반복 text, 잘못된 XML 문자, 미지원 구조,
   저장 중단을 검증합니다. 거부 시 내용과 리비전은 바뀌면 안 됩니다.
7. 두 전략의 시간, 최대 memory, 환경, 버전, 입출력 hash와 증거를 기록합니다.

## 비교 규칙

ZIP metadata 때문에 전체 byte가 달라질 수 있으므로 압축 해제 entry payload를
따로 비교합니다. 손대지 않은 entry는 payload hash가 같아야 합니다. 수정한 XML은
namespace-aware 구조 비교와 사전에 정한 text 변경 allowlist를 사용합니다. 의미
있는 공백, run 속성, element 순서, 알 수 없는 속성과 element, reference,
relationship은 보존해야 합니다.

part 누락, 깨진 relationship, extension 삭제, 관련 없는 node 변경은 실패입니다.
Word에서 repair prompt가 없어야 하며 대상 text, 주변 기능, image, 서식이
유지되어야 합니다. no-op과 같은 길이 편집은 page 수와 대상 밖 영역이 설명 없이
바뀌면 안 됩니다. 긴 text의 reflow는 별도로 평가합니다.

## 보안과 저장 경계

입력 byte, 확장 byte, entry 수, XML 깊이, text 크기, 실행 시간 제한을 먼저
정합니다. archive traversal, 중복 entry, 잘못된 relationship, 암호화 입력, 외부
entity를 거부하고 외부 relationship을 가져오거나 embedded content를 실행하지
않습니다. 실험은 새 목적지에 저장하며 기존 목적지가 있으면 바꾸지 않습니다.

## 결정 기준과 결과물

| Gate | 통과 조건 | 실패 시 조치 |
| --- | --- | --- |
| D-G1 package 보존 | no-op과 관련 없는 내용의 구조 및 hash 검사 통과 | native 편집 전략 거부 또는 범위 축소 |
| D-G2 지원 변경 | D-01부터 D-04에서 선언한 대상만 변경 | 대상 지정 수정 또는 범위 축소 |
| D-G3 native reopen | Word가 repair 없이 열리고 의미 및 시각 검사 통과 | Word 호환성 주장 금지 |
| D-G4 실패 동작 | invalid 및 stale batch와 저장 실패가 부분 변경을 남기지 않음 | writable adapter 연동 차단 |
| D-G5 기본 생성 | 새 문서가 열리고 목적지 충돌이 무변경 | 편집이 성공해도 생성은 미지원 유지 |
| D-G6 연동 준비 | license, 제한, typed 계약, 재현 가능한 증거 기록 | prototype 격리 유지 |

Markdown 실험 보고서에는 후보 버전과 license, fixture manifest, 명령, 예상 및 실제
결과, gate별 상태, 구조 차이, native 화면, 성능, 남은 제한을 포함합니다. 비공개
자료는 Git 밖에 둡니다. gate가 통과하면 저장 방식과 제한된 editor를 선택하는
ADR을 작성하고 실제 전송 및 브라우저 경로로 구현합니다. 공통 session 추상화는
실제 XLSX와 DOCX 소비자가 계약을 입증한 뒤에만 추출합니다.
