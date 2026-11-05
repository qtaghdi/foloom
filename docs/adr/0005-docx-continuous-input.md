# ADR 0005: DOCX native 연속 선택과 범위 입력

날짜: 2026-11-05. 관련 이슈: #85. 선행 기반: #83, ADR 0004.
상태: 지원 일반 문단의 연속 입력에 채택.

## 문제와 결정

run별 textarea는 서로 다른 서식과 문단을 넘는 선택을 표현하지 못합니다. 지원
일반 문단을 하나의 contenteditable 본문에 배치하고 native DOM 선택을 문서 위치로
변환합니다. 원본 run ID를 가진 span은 서식과 source 참조를 유지하며 텍스트를
HTML로 실행하지 않습니다. 보호된 구조의 입력은 기존 fallback을 유지합니다.

같은 run 안의 일반 입력은 native DOM을 유지하고 input에서 초안 텍스트를 갱신합니다.
범위 교체·문단 나눔·삭제는 beforeinput과 키 intent를 기존 revision-safe 문단 계획에
적용합니다. 선택 밖 run을 유지하고 삽입은 선택 시작 run 서식을 사용합니다.
셀을 가로지른 선택은 셀별 계획으로 나누어 표 topology를 유지합니다.

선택 offset은 UTF-16 계약을 사용하되 Intl.Segmenter로 grapheme 경계를 검사합니다.
잘못된 경계를 추정해서 수정하지 않습니다. composition 동안은 DOM을 다시 만들지
않고 최종 텍스트를 한 번 적용합니다. 조합 중 저장과 원격 commit은 초안을 덮어쓰는
근거가 될 수 없습니다. 실제 OS·기기 검증 없이 native IME 지원 완료를 주장하지 않습니다.

## 대안과 영향

run 입력칸 사이에서 가짜 선택을 그리는 방법은 native 드래그·접근성·IME와
별도 선택을 만들므로 사용하지 않습니다. 외부 editor engine이나 HTML 저장 계약은
추가하지 않습니다. 서버는 기존 typed 문단 계획과 원본 보존·원자 저장 규칙을
그대로 사용하며 DOM이나 임의 HTML을 저장 입력으로 받지 않습니다.

Clipboard 서식 정규화, IME 저장 대기·선택 lifecycle, 서버 undo는 독립 동작으로
각각 검증하고 별도 커밋에서 연결합니다. 단계별 미검증 범위를 계속 기록합니다.
