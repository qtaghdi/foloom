# XLSX 안정 릴리스 체크리스트

상태: 릴리스 계획이며 인증 결과가 아닙니다.
기준: 구현 `4a6cfeb`, 공통 명세 `e79e7e0`.
[제품 명세](product-spec.md)와 [형식별 지원표](format-support.md)를 따릅니다.

## 릴리스 범위

첫 안정 버전은 프로세스당 기존 로컬 XLSX 하나, 같은 리비전 세션을 사용하는 사람
및 AI 편집, native XLSX 저장을 지원합니다. 셀, 수식 보존, 기본 서식, 사각 범위
붙여넣기, 병합, 행과 열 작업, 기존 이미지와 차트의 이동·크기 조절·삭제, AI의
차트 제목 변경, 제한된 실행 취소와 다시 실행을 포함합니다.

빈 문서 및 template 생성, 변환, 호스팅 협업, staged proposal 검토, macro 실행,
암호화 편집, 이미지 삽입, 고급 차트 작성, 완전한 Excel 수식 계산은 제외합니다.
브라우저 계산은 Excel의 기준 결과로 표현하지 않습니다. 수식과 의존 셀 변경,
미지원 함수, 외부 참조, 오래된 cache를 실제 Excel에서 다시 열어 확인해야 합니다.

## 현재 증거

- [복구 안내](xlsx-recovery.md)와 전용 테스트가 X-03부터 X-05의 candidate
  rollback, failure injection, source hash, writer lock, replay reset, 대기 queue를
  검증합니다.
- `make xlsx-safety-verify`는 X-01부터 X-06의 commit, 환경, lockfile 및 fixture
  hash, 명령, 결과를 기록합니다.
- `make xlsx-pre-excel-verify`는 Excel 없이 가능한 safety, compatibility corpus,
  전체 frontend 및 Go 검사, clean package, 400,000-cell release profile을
  실행합니다.
- 구현 commit `88965f1`의 clean 결과는
  [사전 Excel 검증 기록](release-evidence/xlsx-pre-excel-2026-10-13.md)에 있습니다.
- 실제 Microsoft Excel과 보이는 호스트 UI 검증은 여전히 수동 gate입니다.

XLSX 사전 검사는 regular file과 정상 OOXML ZIP만 허용합니다. 암호화 entry,
위험하거나 중복된 part 이름, 10,000개 초과 part, 확장 후 256 MiB 초과 part,
총 512 MiB 초과 확장, 100 MiB 초과 압축 원본을 parser 전에 거부합니다.

이력은 최근 snapshot 10개만 유지하며 32 MiB를 넘는 snapshot은 commit을 실패시키지
않고 보관에서 제외합니다. 모든 undo와 redo에서도 리비전은 증가합니다.

## Gate 표

| Gate | 요구사항 | 현재 증거 | 종료 조건 |
| --- | --- | --- | --- |
| X-01 입력 경계 | DOC-01 | file 및 OOXML 사전 검사와 무변경 실패 테스트 | candidate 실행과 오류 문구 검토 |
| X-02 transaction | EDIT-01, EDIT-03, SAVE-01 | 혼합 batch 원자성과 같은 리비전 동시 writer 테스트 | candidate에서 불변 조건 검토 |
| X-03 저장 실패 | SAVE-01, REC-01 | create, write, short-write, sync, close, rename, rollback, pending history 실패 테스트 | candidate 결과 검토 |
| X-04 외부 writer | REC-01 | 변경 전·교체 전 source 검사, writer lock, 외부 교체와 late change 테스트 | single-writer 경계와 마지막 race 수용 |
| X-05 연결과 재시도 | REC-01 | replay gap, slow subscriber, session mismatch, lost response, paused queue, reconnect, resync 테스트 | blind retry를 하지 않는 동작 검토 |
| X-06 이력 | HIST-01 | undo/redo, restart, 교체, snapshot 제한, 단조 증가 리비전, UI disabled 상태 | candidate 불변 조건 검토 |
| X-07 native 보존 | COMP-01 | 생성 및 LibreOffice corpus, object round-trip | 출처가 기록된 Excel fixture와 모든 지원 편집의 native reopen |
| X-08 수식 | COMP-01 | 수식 저장, 값 상태, 구조화 정책, 경고, 재계산 속성 테스트 | 실제 Excel에서 변경 수식, 의존성, 미지원 함수, 외부 참조, stale cache 검증 |
| X-09 호스트 | HOST-01, AUTH-01 | 도구와 Codex 브라우저 대체 화면 기록 | candidate Claude 및 Codex 모드, 권한, 사람·AI 편집, 충돌, native reopen |
| X-10 UI와 제한 | EDIT-02, COMP-01 | 키보드, 붙여넣기, 이력, 상태, focus, touch, reduced motion, 반응형 shell | candidate의 보이는 브라우저 grid 검사 |
| X-11 접근과 진단 | AUTH-01 | 인증 조합, cross-role, origin, token 비교, 비노출, header, loopback, 일회용 링크 테스트 | candidate 실행과 불변 조건 검토 |
| X-12 패키징 | HOST-01 | clean CI, frozen 설치, 두 UI bundle, 실행 smoke test, version 및 preflight, 다중 플랫폼 archive와 checksum | candidate matrix 실행과 배포 조건 결정 |

근거 코드는 [session tests](../internal/formats/xlsx/session_test.go),
[corpus tests](../internal/formats/xlsx/compatibility_test.go),
[persistence](../internal/formats/xlsx/persistence.go),
[transport tests](../internal/transport/httpapi/server_test.go)입니다.

## 경고로 면제할 수 없는 안전 조건

조용한 원본 손실, 부분 commit, stale write 승인, 거짓 저장 성공은 릴리스를
차단합니다. 경고만 추가해 위험한 overwrite를 허용할 수 없습니다. X-04의 hash
검사만으로 check와 replacement 사이 race를 제거할 수 없으며 Excel과 조정된다고
주장하면 안 됩니다. X-03은 history manifest와 rollback 및 reload 실패를
포함해야 합니다. X-11은 시작 시 의도된 인증 정보 전달과 일반 로그를 분리합니다.

## Candidate 검증 절차

1. `make xlsx-pre-excel-verify`를 실행하고 aggregate, safety, performance 보고서를
   Git 밖에 보관합니다.
2. commit, 환경, lockfile과 fixture hash, X-01부터 X-06 결과를 검토합니다.
3. 표에서 빠진 실패 경로를 재현 가능한 테스트로 확인합니다.
4. corpus 복사본에 사람 및 AI 편집을 수행하고 Excel과 LibreOffice에서 다시 엽니다.
5. 각 주장 모드마다 [호스트 절차](host-compatibility.md)를 수행합니다. native MCP
   App과 브라우저 대체 화면 결과를 구분합니다.
6. 데스크톱, 작은 노트북, 모바일 대체 화면에서 keyboard와 reduced motion을
   기록합니다.
7. README, 형식표, 호스트표, 제한, 변경 이력을 맞추고 비공개 자료와 인증 정보는
   Git에 넣지 않습니다.

설정된 한계는 입력 100 MiB, 범위당 10,000셀, batch당 1,000작업, event 512개,
history snapshot 10개, snapshot당 32 MiB입니다. 이는 성능 보장이 아닙니다.

## 승인 기록

- [ ] X-01부터 X-12가 닫혔거나 안전한 검증 모드로 제한되었습니다.
- [ ] Microsoft Excel fixture 출처와 native reopen 증거를 기록했습니다.
- [ ] 관련 없는 기능의 설명되지 않은 손실이 없습니다.
- [ ] 주장하는 Claude 및 Codex 모드를 candidate에서 인증했습니다.
- [ ] 수식 동작과 history 및 resource 제한이 제품 문구와 일치합니다.
- [ ] 원본 손실, 권한, 거짓 성공 결함이 열려 있지 않습니다.
- [ ] candidate 검사가 통과하고 릴리스 문서가 일치합니다.

각 gate에 ID, candidate commit, 날짜, 검토자, fixture hash, 명령 또는 수동 절차,
예상 및 실제 결과, 증거 경로, 상태를 기록합니다. blocked native 검사를 parser
재열기로 대신하지 않습니다.

## 구현 순서

1. X-03부터 X-05의 복구 및 overwrite gap을 집중 테스트로 닫습니다.
2. Excel corpus와 수식 정책 검증을 완료합니다.
3. candidate 호스트, UI, 접근, 패키징 gate를 닫습니다.
4. 검증된 범위만 배포하고 이미지 삽입 및 고급 차트는 별도 후속 작업으로 둡니다.
