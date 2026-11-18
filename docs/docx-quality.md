# DOCX 품질 검증과 승격 기준

추적: #84, 전체 목표 #82. 기능 구현 순서는 #83과 #85의 구조·입력 모델,
#86~#88의 작성 기반, #89~#98의 일반 문서·고급 개체입니다. 각 이슈의 완료
조건은 별도로 유지합니다. 아래 검사 기반을 추가했다고 편집기 전체를 완료하거나
실험 기능을 production으로 승격하지 않습니다.

## 자동 보존 검사

```sh
make docx-compatibility-test
make verify web-test compatibility-test package-test package-check
```

첫 명령은 corpus의 출처·권리·생성 앱·기능·native 상태·SHA-256을 검사합니다.
등록되지 않은 DOCX 또는 manifest와 다른 bytes는 실패합니다. 복사본에 실제
본문·표 변경을 저장하고 다시 열어 대상 밖 XML과 다른 package part를 비교합니다.
서로 다른 한글·이모지·공백·XML 특수 문자·조합 문자로 세 번 저장·재열기하여
반복 저장에 따른 변형 누적도 검사합니다. 단순한 no-op 반복은 이 검사를 대신하지
않습니다. 기존 DOCX CI의 adapter 검사에서 같은 테스트를 실행합니다.

native 앱에서 확인할 adapter 저장 결과는 새 절대 경로에 내보낼 수 있습니다.

```sh
DOCX_COMPATIBILITY_OUTPUT=/tmp/docx-candidate-unique make docx-compatibility-test
```

원본 corpus는 직접 열어 수정하지 않습니다. 출력 경로에 같은 이름이 있으면
실패하여 기존 자료를 덮어쓰지 않습니다. 결과 파일은 사용자 문서가 없는 합성
자료이며 생성된 출력과 스크린샷은 커밋하지 않습니다.

## 기능별 지원표

`보존`은 현재 합성 fixture 또는 adapter 검사가 다룬 범위이며 모든 외부 문서에
대한 보장이 아닙니다. 표시와 편집은 서로 다른 기준으로 판단합니다.

| 기능 | 표시·편집 | 보존·재열기 증거와 남은 범위 |
| --- | --- | --- |
| 본문·혼합 run·입력 | 고른 문단의 추가·분리·병합·이동, 혼합 run 선택과 서식 붙여넣기를 지원합니다. Foloom에서 복사한 탭·연속 공백·줄바꿈이 유지됩니다. 새 문단에 Word ID를 저장합니다. | 합성 corpus의 대상 밖 XML과 반복 저장, Chrome의 범위 선택과 macOS 한글 입력을 확인했습니다. 복합 보호 경계, 일본어·중국어 실제 입력기와 모바일 키보드는 추적 중입니다. [#83](https://github.com/qtaghdi/foloom/issues/83), [#85](https://github.com/qtaghdi/foloom/issues/85), [#234](https://github.com/qtaghdi/foloom/pull/234), [#246](https://github.com/qtaghdi/foloom/pull/246) |
| 글자·문단 서식·스타일 | 지원 글자 서식, 문단 정렬·간격·방향, 문단·글자 스타일과 언어를 지정합니다. 편집 run의 라틴·동아시아 글꼴을 구분해 표시합니다. | 부분 run 변경 뒤 주변 속성과 테마 글꼴 보존을 비교했습니다. 테마 조판과 Word에서의 고급 혼합 서식은 추적 중입니다. [#87](https://github.com/qtaghdi/foloom/issues/87), [#184](https://github.com/qtaghdi/foloom/pull/184), [#198](https://github.com/qtaghdi/foloom/pull/198), [#238](https://github.com/qtaghdi/foloom/pull/238) |
| 목록·탭 | 글머리·번호·상위 번호를 포함한 다단계 목록과 지원하는 탭 정지점·리더를 편집합니다. | Word 번호 정의와 LibreOffice에서 다시 저장한 복합 목록 번호를 확인했습니다. 기존 복합 하위 항목의 재시작과 목록 조판은 추적 중입니다. [#89](https://github.com/qtaghdi/foloom/issues/89), [#224](https://github.com/qtaghdi/foloom/pull/224), [#226](https://github.com/qtaghdi/foloom/pull/226) |
| 표 | 단순 표의 삽입·삭제, 행·열과 높이 등을 변경합니다. 병합·중첩 및 복합 범위가 있는 표의 구조를 보호합니다. | 사각 표의 바뀌지 않은 셀을 비교했고 표로 시작하는 문서의 원본 표를 유지했습니다. [#91](https://github.com/qtaghdi/foloom/issues/91), [#206](https://github.com/qtaghdi/foloom/pull/206) |
| 그림 | 지원 인라인 그림을 삽입·교체·삭제하고 크기·회전·자르기·대체 텍스트·캡션을 편집합니다. 그림과 번호 캡션을 본문의 지원 위치에서 함께 옮기고 전용 인라인 그림 문단을 정렬하거나 위아래 감싸기로 전환합니다. | media 및 관계 파일과 저장·실행 취소 바이트를 비교했고 LibreOffice에서 캡션 순서, 위아래 감싸기 위치와 인라인 복원을 확인했습니다. 임의 좌표·다른 감싸기·복합 구조는 추적 중입니다. [#92](https://github.com/qtaghdi/foloom/issues/92), [#222](https://github.com/qtaghdi/foloom/pull/222), [#241](https://github.com/qtaghdi/foloom/pull/241), [#251](https://github.com/qtaghdi/foloom/pull/251) |
| 용지·구역·머리글·인쇄 | 용지·여백·구역 경계, 지원 머리글·바닥글, 홀짝·첫 페이지, 일부 쪽 번호 설정과 인쇄를 제공합니다. | Chrome·LibreOffice PDF로 명시된 홀짝 구역의 빈 페이지와 뒤 페이지 구분을 확인했습니다. Word 자동 조판과 차이가 날 수 있는 범위는 따로 기록합니다. [#90](https://github.com/qtaghdi/foloom/issues/90), [#93](https://github.com/qtaghdi/foloom/issues/93), [#233](https://github.com/qtaghdi/foloom/pull/233) |
| 링크·북마크·각주·필드 | 지원 링크·북마크·각주·미주, 일부 상호 참조, 날짜, 제목 목차와 그림·표 목록을 작성하거나 갱신합니다. 기존 내부 링크의 대상을 바꾸고, 번호가 붙은 표 캡션을 작성하거나 지원하는 텍스트 상호 참조의 저장값을 갱신할 수 있습니다. | LibreOffice에서 날짜·목차·번호 목록 및 각주·미주를 재저장했습니다. 복합 참조의 구조 편집과 Word 페이지 번호 재계산은 추적 중입니다. [#95](https://github.com/qtaghdi/foloom/issues/95), [#217](https://github.com/qtaghdi/foloom/pull/217), [#228](https://github.com/qtaghdi/foloom/pull/228), [#236](https://github.com/qtaghdi/foloom/pull/236), [#244](https://github.com/qtaghdi/foloom/pull/244), [#250](https://github.com/qtaghdi/foloom/pull/250), [#253](https://github.com/qtaghdi/foloom/pull/253) |
| 주석·변경 추적 | 지원 주석과 답글·해결, 텍스트·굵게·문단 정렬의 변경 기록과 수락·거절을 제공합니다. | 지원하는 Word 변경 기록의 마크업과 원래 서식 복원을 비교했습니다. 복합 변경과 외부 편집기 간 위치 보존은 추적 중입니다. [#96](https://github.com/qtaghdi/foloom/issues/96), [#197](https://github.com/qtaghdi/foloom/pull/197), [#202](https://github.com/qtaghdi/foloom/pull/202), [#231](https://github.com/qtaghdi/foloom/pull/231) |
| 수식·도형·텍스트 상자·차트·SmartArt·OLE | 단순 수식·사각형과 일부 텍스트 상자·평문 콘텐츠 컨트롤을 편집합니다. 독립 데이터가 있는 원형 차트를 만듭니다. 지원 차트의 제목·크기·단일 계열 수치를 변경하고 지원 인라인 배치를 삭제합니다. SmartArt 내부 문구를 표시합니다. | LibreOffice에서 일부 개체를 재저장했습니다. 링크·내장 통합 문서·다중 계열 데이터, 복합 배치, SmartArt와 OLE 편집은 추적 중입니다. [#97](https://github.com/qtaghdi/foloom/issues/97), [#207](https://github.com/qtaghdi/foloom/pull/207), [#215](https://github.com/qtaghdi/foloom/pull/215), [#239](https://github.com/qtaghdi/foloom/pull/239), [#245](https://github.com/qtaghdi/foloom/pull/245) |
| 이력·복구·탐색 | 확정 이력과 초안 이력, 내보내기, 복구와 검색·개요를 제공합니다. 원격 복원 작업을 활동에서 확인할 수 있습니다. | Chrome에서 서버 재시작, 확정 복구, 선택과 원격 화면 및 검색 위치를 확인했습니다. 실제 Word 및 보조 입력 검증은 개별 이슈에서 추적합니다. [#86](https://github.com/qtaghdi/foloom/issues/86), [#88](https://github.com/qtaghdi/foloom/issues/88), [#94](https://github.com/qtaghdi/foloom/issues/94), [#98](https://github.com/qtaghdi/foloom/issues/98), [#223](https://github.com/qtaghdi/foloom/pull/223) |

## native 검증 절차

1. 공개 또는 저장소 소유 자료만 사용하고 생성 앱·버전·날짜·사용 권한·기능·
   SHA-256을 manifest에 기록합니다. 개인 문서와 계정 metadata는 포함하지 않습니다.
2. 원본을 복사하고 adapter로 편집·저장한 파일을 별도 native 검증 폴더로 내보냅니다.
3. Word와 LibreOffice에서 복구 모드를 사용하지 않고 엽니다. 복구·손상 경고,
   페이지·본문·표·개체 누락, 글꼴 대체와 배치 차이를 기록합니다.
4. native 앱에서 텍스트와 해당 기능을 수정하고 다른 파일로 저장한 뒤 다시 엽니다.
   adapter에서도 재열기하여 텍스트·관계·스타일·번호·고유 ID·참조를 대조합니다.
   native 앱의 재직렬화에는 byte 동일성을 요구하지 않으며 의미와 시각을 비교합니다.
5. OS·앱 버전·글꼴·화면 크기·fixture와 candidate commit·해시·실행 단계·
   관찰 결과·미검증 항목을 PR에 기록합니다. 성공한 단계를 다른 단계로 확대하지 않습니다.

## 승격 gate

- 모든 필수 기능에 보존·표시·편집·native 재편집 증거를 연결합니다.
- 데이터 손실·기본 작성 차단·거짓 저장 성공과 남은 보안 결함이 없어야 합니다.
- 실패·취소·stale revision·외부 writer·원자 저장 실패에서 파일과 초안을 보존합니다.
- 새 기능에도 ZIP/XML·크기 제한, 인증·Origin, 외부 연결·필드·코드 비실행을 유지합니다.
- UI의 입력·저장·다운로드·재열기, 세 viewport·IME·키보드·접근성·성능을 검증합니다.
- DOCX 전체 검사와 XLSX 회귀를 통과하고 알려진 차이와 남은 native 수동 검사를 공개합니다.
- 최종 후보의 Word·LibreOffice 검증이 없으면 실험 상태를 유지합니다.
  공개 배포와 PR 병합은 소유자의 별도 요청을 따릅니다.

## 2026-11-05 실행 범위

macOS의 Microsoft Word 16.114.4에서 `generated-basic.docx` 복사본을 읽기
전용으로 열었습니다. 복구 대화상자 없이 한글·반복 텍스트·굵게·기울임과 1행
2열 표의 A/B를 확인했습니다. 이어 실제 adapter가 저장·내보낸
`generated-basic.docx`도 복구 대화상자 없이 열어 `changed longer 한글`과
`표 변경`, 수정하지 않은 굵게·기울임·반복 text를 확인했습니다. 이는 adapter
저장 결과의 native 읽기 증거이며 Word에서 다시 수정·저장한 증거는 아닙니다.

Word가 활성화되지 않아 작성·편집·저장이 비활성 상태였습니다. Word 생산 fixture
확보, native 수정·저장·재열기, 정량 시각 비교와 LibreOffice 검증은 남아 있습니다.
구독이나 계정 설정은 변경하지 않았습니다. 따라서 #84 전체 완료와 native 편집
호환성을 주장하지 않습니다.

## 2026-11-08 LibreOffice 자동 재저장

macOS 26.7.2에서 번들 LibreOfficeDev 26.8.0.0.alpha0
(`2c87e51eeaa2b413ff4ae097b2705eea1995d8e5`)의 Writer로 Foloom이 편집한
합성 corpus 두 파일을 열고 DOCX로 재저장했습니다. 입력과 출력에서 각각
PDF를 생성해 재저장 결과를 Writer가 다시 열 수 있는지도 확인했습니다.

```sh
qa_dir="$(mktemp -d /tmp/foloom-lo-qa.XXXXXX)"
DOCX_COMPATIBILITY_OUTPUT="$qa_dir/foloom" make docx-compatibility-test
mkdir "$qa_dir/libreoffice" "$qa_dir/pdf-before" "$qa_dir/pdf-after"
soffice --headless --convert-to 'docx:Office Open XML Text' \
  --outdir "$qa_dir/libreoffice" "$qa_dir"/foloom/*.docx
soffice --headless --convert-to pdf --outdir "$qa_dir/pdf-before" "$qa_dir"/foloom/*.docx
soffice --headless --convert-to pdf --outdir "$qa_dir/pdf-after" "$qa_dir"/libreoffice/*.docx
```

- 두 파일의 문단 텍스트와 표 셀 `A`, `표 변경`이 같았습니다.
- `generated-preservation.docx`의 머리글 텍스트, 외부 링크 대상, `PAGE` 명령,
  삽입 변경 기록은 유지됐습니다. Writer가 머리글을 여러 part로 나누고
  단순 필드를 시작·구분·종료 요소가 있는 필드로 바꿨습니다.
- Writer가 합성 파일에 있던 custom XML과 그림 배치에 사용되지 않는
  `word/media/image1.png`를 재저장 중 제거했습니다. Foloom의 저장 결과에는
  두 part가 모두 남아 있었습니다. 배치된 그림의 재저장 검증은 별도로 필요합니다.
- 재저장 전후의 PDF는 모두 Letter 한 쪽이었습니다. 96 DPI RGB 비교에서
  기본 파일은 1,281픽셀, 보존 파일은 1,383픽셀이 달랐습니다.
  이 수치는 동일한 번들 환경의 결과이며 글꼴별 시각 일치를 증명하지 않습니다.

이 실행은 합성 파일의 headless 재저장과 재열기까지 검증했습니다. 실제 입력을
포함한 LibreOffice 검증, 생산 앱에서 만든 대표 fixture, Word의 수정·저장·재열기는
#84에 남아 있습니다.
