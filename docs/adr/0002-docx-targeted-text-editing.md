# ADR 0002: DOCX 표적 텍스트 편집

상태: 비공개 로컬 실험에 채택. Native Word 호환성 승격은 보류.
날짜: 2026-11-03
관련 이슈: #35

## 문제와 결정

사용자는 DOCX 생성과 본문·표 편집을 우선 요청했습니다. 임의의 Word 문서를 상위
수준 문서 model로 가져왔다가 전체 직렬화하면 알 수 없는 OOXML과 원본 서식이
소실될 수 있습니다. 따라서 Go 표준 라이브러리의 ZIP과 namespace-aware XML
검사로 일반 text run을 식별하고, 원본 byte offset을 이용해 대상 text만 바꿉니다.
외부 엔진, 유료 서비스, 새 frontend framework는 추가하지 않습니다.

`internal/formats/docx`는 검사, typed operation, 보안 제한, 저장과 event를
소유합니다. `httpapi`는 DOCX 전용 도구와 인증된 브라우저를 연결합니다. 브라우저는
원본 문서를 생성하는 주체가 아니라 server snapshot의 지원 대상 입력 surface입니다.
XLSX의 안정 계약과 별도 구체 세션을 유지하며 공통 runtime 추출은 미룹니다.

새 문서는 제한된 paragraph/table block으로만 생성합니다. 임시 파일 완성 후
exclusive hard link로 목적지를 설치해 덮어쓰기를 차단합니다. 기존 문서는 원본
hash와 writer lock을 확인한 뒤 같은 디렉터리 임시 파일을 원자적으로 교체합니다.

## 대안과 실험 증거

상위 수준 document model 재직렬화 방식은 이번 구현에서 평가하지 않았습니다.
현재 범위에는 원본 전체를 재생성할 요구가 없고 표적 수정만으로 유용한 작업을
제공할 수 있기 때문입니다. 두 방식의 성능이나 fidelity 우위를 주장하지 않습니다.

합성 corpus에서 반복 text의 정확한 대상 선택, 혼합 run 서식, 공백, 한글, 단순 표,
다른 XML과 image part의 보존을 검사합니다. 대상 밖 XML byte와 다른 part의 payload
hash를 비교하고 저장·재열기를 반복합니다. 잘못된 mixed batch, stale ID·리비전,
외부 writer, 취소, rename 실패와 lock 충돌은 무변경을 확인합니다.

[보존 실험](../docx-preservation-prototype.md)의 gate 중 합성 자료 범위 D-G1,
D-G2, D-G4, D-G5와 실제 MCP·HTTP 경로를 자동 검사합니다. D-02부터 D-04의 native
생산 자료와 Word reopen D-G3는 미확인입니다. D-G6는 정식 호환성 승격에 대해
미완료입니다. 이번 작업은 전체 gate 통과를 주장하지 않고, 제한과 경고가 표시되는
비공개 실험 surface를 추가합니다.

## 결과와 한계

전체 Word layout이나 자유로운 구조 편집을 제공하지 않습니다. field와 변경 추적은
보호하며 병합·중첩 표를 수정하지 않습니다. 브라우저는 지원 text와 단순 굵게·기울임
표시만 제공합니다. run을 넘는 교체, 구조 추가·삭제와 서버 undo/redo는 후속 범위로
남깁니다. Native fixture와 화면 증거를 확보하기 전에는 복사본 사용을 안내합니다.

상세 계약, 제한과 검증 명령은 [DOCX 사용 안내](../docx-editing.md)를 따릅니다.
