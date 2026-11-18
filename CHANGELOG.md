# 변경 이력

이 프로젝트의 중요한 변경 사항을 기록합니다. 형식은
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/)를 따르며 정식 릴리스부터
Semantic Versioning을 적용할 예정입니다.

## 미출시

### 수정

- #90 쪽 번호를 이어 가는 홀수·짝수 구역의 빈 쪽과 이후 머리글·바닥글을 미리보기와 인쇄에 반영합니다.

- #230 DOCX에서 문단을 나누거나 여러 문단을 붙여넣으면 새 문단에 고유한 Word ID를 부여합니다.

- #218 새 번호 목록에 상위 단계 번호를 포함해 Word 다단계 개요를 만들 수 있습니다.

- #221 DOCX 탭이 포함된 run의 텍스트를 저장할 때 탭과 뒤쪽 텍스트를
  중복하지 않고 원래 run 서식을 유지합니다.

- #213 DOCX의 확정 이력을 다른 탭에서 되돌리거나 다시 실행할 때 고유 문단
  ID가 유지된 본문 선택을 복원하고 활동에 실행 취소와 다시 실행을 구분해 표시합니다.

- #85 취소되거나 끝난 DOCX 텍스트 전용 단축키가 다음 붙여넣기의 서식을 지우는 문제를 수정했습니다.

- #90 명시적으로 나뉜 페이지의 머리글·바닥글에 구역의 연속 쪽 번호와 새 시작 번호의 홀짝을 반영합니다.

- #86 DOCX 이용 안내에 확정 실행 취소·다시 실행과 서버 재시작 뒤 이력 복구를 반영합니다.

- #180 DOCX MCP 도구가 실제로 지원하는 번호 형식, 필드, 주석 및 변경 추적 작업을 정확히 안내합니다.

- #87 DOCX에서 선택한 글꼴을 브라우저가 확인하지 못하면 대체 글꼴 표시 가능성을 안내합니다.

- #89 가져온 다단계 목록의 단계별 번호 재시작 규칙을 저장 화면과 편집 초안에 반영합니다.

- #150 새 관계나 파일 유형이 추가된 DOCX를 LibreOffice에서 다시 열 수 있도록 패키지 메타데이터를 저장합니다.

- #148 압축하지 않은 항목을 가진 DOCX를 편집하고 저장한 뒤 LibreOffice에서 다시 열 수 있습니다.

### 변경

- #106 실행 명령·Go module·설치 스킬·릴리스 파일명과 GitHub 저장소를 `foloom`으로
  전환합니다. MCP 기본 설정은 새 이름으로 이관하고 수정한 스킬과 문서 이력·lock·
  브라우저 설정 키는 보존합니다.

- #106 폴룸(Foloom)의 벡터 로고·워드마크·아이콘·소개 이미지와 브랜드 가이드를
  추가하고 작업 공간·XLSX·DOCX·MCP App의 로고, 강조색, 소개 문구와 브라우저
  메타데이터를 통일합니다. 문서의 원래 서식과 저장 상태 구분은 유지합니다.

- #113 PR 변경 경로를 한 번 분류해 관련 검사만 실행하고, 선행 브랜치 대상 PR도 자동 검사합니다.

### 추가

- #92 DOCX 본문의 전용 인라인 그림을 왼쪽·가운데·오른쪽으로 정렬할 수 있습니다.

- #225 DOCX 본문 문단의 정렬 변경을 추적해 저장하고 수락하거나 이전 정렬로 되돌릴 수 있습니다.

- #220 DOCX 본문의 번호 캡션을 모아 별도 그림 목록과 표 목록을 삽입합니다.

- #220 DOCX의 그림 목록과 표 목록을 현재 본문 캡션으로 갱신합니다.

- #212 DOCX 본문 문단 끝에 날짜를 표시하는 Word DATE 필드를 삽입합니다.

- #212 DOCX의 지원 날짜 필드를 선택해 서버의 현재 UTC 날짜로 갱신합니다.

- #92 본문의 인라인 그림과 번호 캡션을 같은 구역의 인접한 일반 문단 너머로 함께 옮길 수 있습니다.

- #96 일반 본문에서 굵게 적용·해제를 Word 서식 변경으로 기록하고 검토할 수 있습니다.

- #96 Word에서 가져온 일반 본문 글자 서식 변경을 수락하거나 거절할 수 있습니다.

- #85 DOCX 기본 서식 복사·붙여넣기에서 밑줄과 취소선을 보존합니다.

- #83 표로 시작하는 DOCX의 첫 표 앞에 빈 본문 문단을 추가할 수 있습니다.

- #83 평문 웹 링크나 북마크 링크가 있는 본문 문단을 인접한 일반 문단과 순서를 바꿀 수 있습니다.

- #89 자기 단계 번호만 표시하는 하위 목록의 시작값을 기존 번호 형식과 수준을 유지하며 다시 지정할 수 있습니다.

- #89 기존 번호 형식과 들여쓰기를 유지하며 선택한 목록 항목을 지정 번호부터 다시 매길 수 있습니다.

- #92 기본 인라인 그림을 90도와 270도로 회전하고 문단에서 회전된 크기로 확인할 수 있습니다.

- #95 일반 최상위 번호 목록의 북마크 번호를 Word 참조로 삽입할 수 있습니다.

- #95 자동 목차에 표시된 제목을 현재 본문에서 다시 불러올 수 있습니다.

- #95 평문 북마크가 있는 쪽을 가리키는 Word PAGEREF 상호 참조를 삽입할 수 있습니다.

- #95 본문 제목 1~3 수준을 모아 Word 자동 목차 필드를 삽입할 수 있습니다.

- #92 번호를 유지하면서 일반 인라인 그림의 캡션 문구를 바꿀 수 있습니다.

- #97 기존 SmartArt의 실제 도형 문구를 읽기 전용 목록으로 확인할 수 있습니다.

- #97 지원되는 인라인 차트의 표시 크기를 비율을 유지하면서 바꿀 수 있습니다.

- #97 지원되는 내부 Word 차트의 평문 제목을 확인하고 기존 데이터와 함께 저장할 수 있습니다.

- #97 지원 인라인 사각형의 너비와 높이를 각각 바꿀 수 있습니다.

- #97 문단의 텍스트와 다른 도형을 유지하면서 선택한 인라인 사각형을 삭제할 수 있습니다.

- #97 지원 인라인 사각형의 채우기와 테두리 색을 바꿀 수 있습니다.

- #97 본문 문단에 기본 인라인 사각형을 삽입하고 브라우저에서 모양과 색을 확인할 수 있습니다.

- #90 머리글·바닥글에 현재 쪽 번호와 문서 전체 쪽 수를 한 줄로 표시할 수 있습니다.

- #90 구역마다 왼쪽 제본 여백을 정하고 편집 화면과 인쇄에서 본문 위치를 확인할 수 있습니다.

- #90 구역마다 용지 모서리에서 머리글·바닥글까지 거리를 정하고 화면에서 배치를 확인할 수 있습니다.

- #87 선택한 글자의 라틴 문자와 동아시아 문자 맞춤법 검사 언어를 각각 지정할 수 있습니다.

- #87 글자 서식에서 동아시아 글꼴을 별도로 지정하고 다른 문자권의 글꼴과 언어 설정을 유지합니다.

- #92 일반 본문의 인라인 그림에 Word가 계산하는 번호와 캡션을 넣을 수 있습니다.

- #97 Word 제곱근 수식을 삽입하고 일반 피제곱근을 수정할 수 있습니다.

- #96 기존 주석이 없는 문서에서 새 주석에 답글을 달고 해결하거나 다시 열 수 있습니다.

- #96 Word에서 가져온 최상위 주석에 답글을 쓰고 한 곳에서 읽을 수 있습니다.

- #97 편집 가능한 본문 문단에 Word 인라인 텍스트 상자를 삽입할 수 있습니다.

- #214 편집 가능한 본문 문단 끝에 평문 콘텐츠 컨트롤을 삽입하고 저장한 개체를 수정할 수 있습니다.

- #95 평문 북마크의 내용을 Word REF 필드로 참조하고 브라우저에서 대상 문단으로 이동할 수 있습니다.

- #96 Word에서 가져온 최상위 주석의 해결 상태를 보고 해결하거나 다시 열 수 있습니다.

- #97 독립 텍스트 상자의 평문을 수정하고 기존 위치·크기·호환 표현을 유지할 수 있습니다.

- #95 참조 한 곳에 연결된 평문 미주를 지우면서 다른 미주·각주와 원본 관계를 유지할 수 있습니다.

- #90 머리글·바닥글에 현재 쪽 번호와 별도로 전체 쪽 수 필드를 넣고 제거할 수 있습니다.

- #92 기본 인라인 그림을 180도로 회전하고 기존 크기와 자르기를 유지할 수 있습니다.

- #89 영문자 번호 목록을 새로 만들거나 가져온 대문자·소문자 번호 목록을 표시할 수 있습니다.

- #95 기존 평문 미주를 본문 참조와 변경하지 않은 텍스트의 서식을 유지하며 수정할 수 있습니다.

- #96 검토 도구에서 일반 본문 텍스트를 교체하거나 삭제하고 Word 변경 추적으로 저장할 수 있습니다.

- #95 일반 본문 문단에 미주를 넣고 각주와 함께 열어 내용을 확인할 수 있습니다.

- #95 일반 본문 북마크를 삭제하고 내부 링크의 표시 텍스트와 서식을 유지할 수 있습니다.

- #97 여러 서식 구간이 있는 평문 콘텐츠 컨트롤도 변경하지 않은 텍스트의 서식을 유지하며 편집할 수 있습니다.
- #97 본문에 있는 일반 콘텐츠 컨트롤의 텍스트를 원본 서식을 유지하며 편집할 수 있습니다.

- #89 시작 값을 지정해 대문자·소문자 로마 숫자 목록을 만들 수 있습니다.

- #95 일반 본문 북마크의 이름과 그곳으로 이동하는 내부 링크를 함께 바꿀 수 있습니다.

- #90 DOCX 머리글·바닥글에 Word가 계산하는 쪽 번호 필드를 넣거나 뺄 수 있습니다.

- #92 기본 채우기 형식의 인라인 PNG·JPEG에서 표시 영역을 자르거나 해제할 수 있습니다.

- #90 기존 DOCX 구역 나눔의 설정을 유지하면서 앞뒤 문단으로 옮길 수 있습니다.

- #96 DOCX에서 가져온 일반 텍스트 삽입·삭제 변경을 검토하고 수락하거나 거절할 수 있습니다.

- #83 본문 문단 안에 끝나는 북마크를 문단과 함께 앞뒤로 이동할 수 있습니다.

- #95 DOCX의 일반 각주와 연결된 본문 참조를 제거할 수 있습니다.

- #95 DOCX의 일반 각주 내용을 편집할 수 있습니다.

- #95 DOCX 본문에 텍스트 각주를 연결하고 브라우저에서 내용을 미리 볼 수 있습니다.

- #95 기존 웹 링크와 북마크 링크를 해제하면서 표시 문구와 서식을 유지합니다.

- #92 기존 인라인 그림의 크기와 대체 텍스트를 유지하면서 PNG·JPEG 파일을 교체할 수 있습니다.
- #92 기존 인라인 그림의 너비를 지정해 비율을 유지한 채 크기를 바꿀 수 있습니다.

- #90 기존 DOCX 구역 나눔을 제거하고 뒤 구역의 설정으로 합칠 수 있습니다.

- #89 가져온 다단계 목록에서 법률식 번호를 켠 단계는 참조한 번호를 모두 십진수로 표시합니다.
- #89 가져온 DOCX의 대문자·소문자 로마 숫자 목록 번호를 미리보기에 표시합니다.

- #97 DOCX의 단순 분수를 선택하고 분자·분모를 바꿀 수 있습니다.
- #97 DOCX에 분자와 분모를 입력해 간단한 분수 수식을 삽입할 수 있습니다.

- #96 DOCX 본문 한 문단에서 서식 구간의 일부를 선택해 글자 단위 주석을 달 수 있습니다.
- #96 DOCX 본문에서 선택한 전체 서식 구간에 주석을 달 수 있습니다.

- #95 기존 DOCX 웹 링크의 표시 문구와 서식을 유지하면서 주소를 바꿀 수 있습니다.

- #92 기존 인라인 PNG·JPEG를 선택해 삭제할 수 있습니다.

- #90 DOCX 본문 문단 끝의 명시적 페이지 나눔을 해제할 수 있습니다.

- #92 기존 인라인 PNG·JPEG의 대체 텍스트를 변경하거나 비울 수 있습니다.

- #130 Foloom의 기능과 시작 방법을 소개하는 `/about` 페이지를 추가했습니다.

- #94 DOCX 문서 개요에서 번호를 입력해 미리보기 페이지로 이동할 수 있습니다.

- #94 DOCX 문서 개요에서 저장된 본문 북마크를 찾아 이동할 수 있습니다.
  Escape로 개요를 닫으면 열기 버튼으로 초점을 돌려보냅니다.

- #91 단순 DOCX 표에서 행의 최소 높이를 설정할 수 있습니다.
  다른 행 속성과 셀 내용은 보존합니다.

- #89 DOCX 탭 위치·정렬·채움선과 명시적 해제, 실제 탭 문자 삽입을 추가합니다.
  가져온 탭을 편집 가능한 텍스트로 보존하며 위치별 변경과 undo를 검증합니다.

- #89 독립된 번호 정의로 글머리·번호 목록을 생성하고 시작 번호·수준·이어쓰기·
  해제를 편집합니다. Enter의 다음 항목·빈 항목 종료와 초안 실행 취소를 연결하고
  기존 번호 정의와 사용자 지정 글머리를 보존합니다.

- #88 미저장 초안까지 원본에 쓰지 않고 다른 이름의 DOCX로 내보냅니다. 지원
  브라우저는 사용자가 선택한 대상에 기록하고 그 외에는 다운로드를 요청합니다.

- #88 메타데이터가 손상된 복구 사본도 목록에서 확인하고 폐기할 수 있게 해
  보관 한도가 보이지 않는 파일로 계속 점유되지 않도록 합니다.

- #88 여러 서버가 같은 복구 보관소를 사용할 때 OS 파일 잠금으로 보관 한도와
  삭제를 보호하고 비정상 종료 시 잠금을 자동 해제합니다.

- #88 DOCX 복구 보관에 동의하면 미저장 편집을 검증한 별도 패키지로 보관합니다.
  복구 사본 목록에서 초안 포함 여부를 보고, 원본 변경 없이 새 탭으로 엽니다.

- #88 `foloom -workspace`로 기존 파일 없이 빈 작업 공간을 시작합니다.

- #88 동의한 DOCX의 확정 버전을 비공개 로컬 복구 사본으로 보관하고,
  재시작 후 새 작업 탭으로 열거나 폐기할 수 있는 API를 추가합니다.

- #88 저장된 DOCX를 다른 이름의 작업 복사본으로 엽니다. 미저장 초안은
  원본 탭에 남기고, 복사본은 독립된 리비전과 파일 이름으로 다운로드합니다.

- #88 작업 공간에서 이름을 지정해 빈 DOCX를 만듭니다. 새 문서는 별도 작업
  복사본으로 열리며 기존 파일을 덮어쓰지 않습니다.

- #91 DOCX 편집기에 표 구조 편집 도구를 추가하고 표 작업을 저장 및 실행 취소에
  연결합니다. 단순 표 안에서 Tab과 Shift+Tab으로 셀을 이동합니다.

- #91 본문에서 표를 삽입·삭제하고 단순 표의 행·열을 추가·삭제·이동하는 계약을
  추가합니다. 기존 셀의 내용과 서식, 행 속성, 열 너비를 보존합니다.

- #93 저장된 DOCX를 편집 도구 없이 브라우저에서 인쇄하거나 PDF로 저장합니다.
  구역별 용지·여백, 본문·표와 지원되는 그림을 출력합니다.

- #98 연속 입력에서 같은 실행 취소 상태를 글자마다 복사하지 않습니다.
  짧은 화면에서는 검색·바꾸기 패널을 스크롤할 수 있습니다.

- #97 단순 인라인 수식을 Word 수식으로 삽입·교체·삭제합니다. 고급 개체를
  유형별로 표시하며 지원하지 않는 구조는 읽기 전용으로 보존합니다.

- #96 본문 문단에 DOCX 주석을 추가하고 저장된 주석과 대상 문단을 검토합니다.
  원본 변경 추적은 읽기 전용 상태로 표시합니다.

- #96 AI 변경 위치에 고유한 Word 문단 ID를 사용합니다. 구조 변경에서 같은
  문단을 확인할 수 없으면 추측한 위치를 표시하지 않습니다.

- #95 링크·북마크를 브라우저에서 삽입하고 본문 북마크로 이동합니다.
  안전한 외부 링크는 사용자가 클릭할 때 별도 탭에서 엽니다.

- #95 본문에 북마크와 문서 내부·HTTP(S) 링크를 삽입하는 계약을 추가합니다.
  이름·대상·주소와 관계 ID를 검증하고 외부 주소를 자동으로 읽지 않습니다.

- #94 본문 제목의 개요 수준을 읽어 현재 제목과 제목 간 이동을 제공합니다.

- #94 DOCX 찾기에 검색 범위, 대소문자·단어 단위, 이전·다음 이동과 한 건·전체
  바꾸기를 추가합니다. 전체 바꾸기는 한 번에 저장하고 실행 취소합니다.

- #92 편집기에서 그림 파일을 선택하고 대체 텍스트를 지정합니다. 저장한
  인라인 PNG·JPEG를 인증된 문서 안에서 미리 볼 수 있습니다.

- #92 검증한 PNG·JPEG를 인라인 DOCX 그림으로 삽입하는 계약을 추가합니다.
  그림의 비율과 대체 텍스트, 기존 media 및 관계를 보존합니다.

- #111 문서 보존·리비전·인증·MCP와 편집기 경계를 우선하는 한국어 CodeRabbit
  자동 리뷰 설정과 경로별 검토 기준을 추가했습니다.



- #90 본문 문단 앞 페이지 나눔 설정과 해제, 문단 뒤 다음 페이지 또는
  연속 구역 나눔을 추가합니다. 기존 용지 설정과 머리글 참조를 보존합니다.

- #90 홀수·짝수 페이지의 머리글과 바닥글 구분을 문서 전체 설정으로 저장합니다.
  미리보기에서 명시적 페이지 나눔에 따라 해당 내용을 표시합니다.

- #90 구역별 쪽 번호 시작값, 이전 구역에서 계속, 번호 형식을 편집하고 저장합니다.
  원래 장 번호 설정과 다른 구역의 번호 속성은 유지합니다.

- #90 구역별 머리글·바닥글 작성 도구와 첫 페이지 구분, 이전 연결 복원과
  본문 미리보기를 연결합니다. 서로 다른 variant의 초안도 함께 저장·undo합니다.

- #90 구역별 단순 텍스트 머리글·바닥글 part 생성, 이전 구역 연결과 첫 페이지
  구분 계약을 추가합니다. 공유 원본을 보존하고 보호된 내용의 평탄화를 거부합니다.

- #90 구역 선택 도구와 용지·여백·방향·단·나누기 유형의 초안 편집을 연결합니다.
  구역마다 용지를 구분해 미리 보고 본문과 같은 저장·undo 이력을 사용합니다.

- #90 구역별 용지·방향·여백·단·나누기 유형의 snapshot과 수정 계약을 추가합니다.
  기존 구역 경계·머리글 참조·custom 속성을 유지하며 충돌·범위를 검증합니다.

- #87 DOCX 문단·글자 스타일 적용과 정의 생성·수정·상속 편집을 추가합니다.
  정의와 선택 범위를 같은 batch로 저장하며 순환 상속을 거부합니다. 원래
  스타일 part 경로·관계·메타데이터를 유지하고 정의 초안도 이력에 포함합니다.

- #87 DOCX 문단 정렬·좌우 및 첫 줄/내어쓰기·앞뒤 간격·줄 간격과 읽기·
  텍스트 방향을 편집합니다. 충돌하는 OOXML 길이 속성만 교체하고 탭·언어·
  나머지 속성을 유지하며 범위 끝의 다음 문단을 변경하지 않습니다.

- #87 DOCX 선택 범위와 이후 입력의 글꼴·크기·굵게·기울임·밑줄·취소선·
  색·강조·첨자와 직접 서식 지우기를 추가합니다. 혼합 선택을 표시하고 필요한
  run만 분리하며 선택 밖 서식, 언어별 글꼴과 문서 스타일 참조를 보존합니다.

- #86 DOCX 도구 모음과 Ctrl/Cmd+Z, 다시 실행 단축키를 서버 이력에 연결합니다.
  미저장 입력은 750 ms의 연속 입력 단위와 독립 구조·붙여넣기 단위로 취소하며,
  초안 이력과 확정 이력의 중복 적용을 막고 복원 뒤 선택 위치를 갱신합니다.

- #86 DOCX의 사람·AI 확정 batch를 같은 순서의 서버 이력으로 보관합니다.
  전체 패키지를 복원하는 실행 취소·다시 실행 API와 MCP 도구, 최대 10개·64 MiB
  이력 한도와 재시작 시 hash 기반 pending 복구를 추가합니다. 외부 변경,
  stale revision, 손상된 snapshot과 저장 실패는 문서·리비전을 바꾸지 않습니다.

- #84 DOCX corpus 출처·SHA-256 검사, 서로 다른 내용의 반복 저장 보존 회귀와
  native 검사용 복사본 내보내기를 추가했습니다. 기능별 지원표와 native 품질
  승격 기준을 기록하고 Word 읽기 검증과 미검증 범위를 구분했습니다.

- #85 입력기 조합 중 저장 요청을 조합 완료 후 한 번으로 모아 처리합니다.
  조합 취소 시 역방향 범위를 복원하고 원격 충돌·연결 실패·문서 종료 시
  대기 중 저장을 취소하며 최종 입력을 초안에 유지합니다.

- #85 붙여넣기에 텍스트만·기본 글자 서식 유지 선택을 추가합니다. 굵게·기울임·
  글꼴·크기·색을 검증된 typed 서식으로 저장하며 외부 HTML의 실행·URL·개체를
  제외합니다. 이미지뿐인 clipboard와 잘라내기 기록 실패가 선택 텍스트를
  삭제하지 않도록 보호합니다.

- #85 지원 문단을 하나의 본문 편집 영역으로 연결해 run·문단·표 셀에 걸친
  연속 선택과 범위 교체를 제공합니다. 선택 밖 run 서식을 유지하고 셀 경계를
  합치지 않으며 Unicode grapheme 단위 삭제와 저장 뒤 선택 복원을 추가합니다.

- #83 본문과 단순 표의 지원 문단에서 Enter 분리, 경계 Backspace·Delete 병합,
  여러 문단 붙여넣기, 문단 이동과 Shift+Enter 줄바꿈을 추가했습니다. 구조 초안을
  한 번에 저장하고 source run 서식과 고유 문단 ID를 보존합니다. 구역·명시적
  페이지 경계와 미지원 구조는 보호하며 구조 초안의 이전 AI 위치는 숨깁니다.


- #101 독립적인 설명·검토·검증·되돌리기를 기준으로 커밋을 나누는 질문과 예시,
  기반·의존 기능의 순서 및 PR 제출 전 점검을 기여 가이드와 PR 양식에 명시합니다.
  전체 이슈 구현 요청을 부분 PR 제출로 끝내지 않고 일반 PR에서 계속 보완하는
  진행 원칙과 실제 완료·검증·병합 승인 구분을 추가합니다.

- #79 파일 선택창에서 여러 XLSX·DOCX를 작업 복사본으로 엽니다. 경로 열기는
  고급 옵션으로 옮기고 언어·최근 파일을 함께 배치합니다. 원본 유지와 수정본
  다운로드를 안내하며 크기 제한과 작업 복사본 종료 정리를 적용합니다.

- #78 XLSX·DOCX에 저장·다운로드와 Ctrl/Cmd+S를 추가합니다. 진행 중 편집을
  먼저 저장하고 문서 ID·리비전·외부 변경을 확인한 원본 형식 파일을 내려받습니다.
  실패·충돌 시 다운로드를 차단하며 확인만으로 리비전이나 이력을 늘리지 않습니다.

- #76 편집 툴바를 제외한 상태·오류·경고·검색·활동·파일 작업 UI에 한국어,
  영어, 일본어, 중국어 간체를 지원합니다. 브라우저 언어 감지와 설정 저장,
  열린 편집기 간 즉시 언어 동기화, 별도 원문 진단을 제공합니다.

- #74 로컬 XLSX·DOCX를 최대 8개 함께 열고 탭으로 전환하는 작업 공간을 추가합니다.
  파일별 초안·선택 위치·저장 상태를 유지하며 최근 파일, 경로 중복 방지,
  저장 후 닫기와 초안 폐기 확인을 제공합니다. 문서별 API·SSE 대상을 고정하고
  여러 XLSX가 열려 있으면 MCP 쓰기에 명시적인 `documentId`를 요구합니다.

- #72 XLSX·DOCX 검색 버튼과 Ctrl/Cmd+F, 검색 위치 이동을 추가합니다. XLSX는
  모든 시트의 저장값을 서버에서 검색하고 DOCX는 본문·표의 미저장 입력을 포함합니다.
  결과는 최대 200개이며 XLSX 탐색 한도 도달 시 일부 결과임을 안내합니다.

- #70 macOS MCP 설치에 XLSX 편집과 DOCX 생성·편집을 안내하는
  `document-workspace` 스킬을 포함합니다.

- #65 DOCX에서 기본·문단·글자 스타일 일부를 상속하고, 문단 경계에 저장된 명시적
  페이지 나눔을 별도 용지로 표시합니다.

- #45: 실제 저장 응답·수신 event 기반의 session 활동 패널을 추가했습니다.
  저장 중·저장됨·확인 필요를 구분하며 문서 내용이나 token은 기록하지 않습니다.

- #43: 실제 선택 범위의 굵게·기울임·글자 크기를 서버에 저장하는 자체 XLSX
  도구 모음과 선택 범위 표시, 기본 ribbon의 고급 도구 전환을 추가했습니다.

- XLSX browser·MCP App과 DOCX의 앱 UI를 Solid 컴포넌트와 반응형 표시 상태로
  전환하고, DOCX 초안 폐기 확인을 자체 UI로 제공합니다.
- 실제 Solid DOM에서 engine container·입력 focus 보존, 저장·충돌·복구와 root
  해제를 검사하는 컴포넌트 테스트를 추가했습니다.

- DOCX 문단·단순 표 생성, 본문·표 run 텍스트의 리비전 기반 batch 편집, 원본
  OOXML 표적 보존, MCP 도구 4개와 인증된 브라우저 편집기를 추가했습니다.
  미지원 구조는 보호하며 Word native 호환성은 아직 검증하지 않았습니다.
- DOCX archive·XML 제한, 외부 writer 감지, 덮어쓰기 없는 생성, 원자적 저장,
  충돌 시 미저장 입력 유지와 합성 호환성 corpus를 추가했습니다.

- browser Alert의 접기, 펼치기, 닫기 control과 MCP의 browser-first workbook 편집
  workflow를 추가했습니다.
- macOS에서 실행 파일, Claude Desktop과 Codex MCP 설정, `document-xlsx` skill을
  함께 관리하는 `mcp install`, `status`, `uninstall` 명령을 추가했습니다.
- 파일 없이 시작한 stdio MCP에서 절대 `.xlsx` 경로를 동적으로 열고 전환하는
  `open_workbook` 입력과 필요한 범위만 읽는 `document-xlsx` prompt를 추가했습니다.
- macOS, Linux, Windows의 AMD64와 ARM64용 미서명 비공개 알파 archive, SHA-256
  manifest, 내용 검사, 현재 platform 실행 smoke test를 추가했습니다.
- 공개 Release 권한이 없는 수동 private-alpha workflow, CLI build metadata,
  위치 인자 경로, cell을 출력하지 않는 preflight를 추가했습니다.
- checksum, 미서명 앱, browser, Claude Desktop, Codex, 승인, backup을 다루는
  비공개 알파 안내를 추가했습니다.
- safety, compatibility, 전체 suite, packaging, release performance를 한 번에
  기록하는 Excel 실행 전 검증과 필수 전체 regression CI를 추가했습니다.
- 25,000셀 CI와 400,000셀 release 성능 profile, latency 및 Go heap budget,
  Markdown 증거를 추가했습니다.
- X-07과 X-08용 Microsoft Excel fixture, 편집, 수식, reopen 절차와 증거 양식을
  추가했습니다.
- X-01부터 X-06까지 재현 가능한 safety 검증과 CI, mixed batch와 동시 writer의
  memory·disk·history·event 원자성 테스트를 추가했습니다.
- cached 및 unavailable 수식 값을 구분하고 browser 결과를 preview로 표시하며
  native 전체 재계산을 요청하는 수식 정책을 추가했습니다.
- source hash, 협조 writer lock, 외부 변경 conflict, event checkpoint, replay
  resync, slow subscriber disconnect, blind retry 없는 browser recovery를
  추가했습니다.
- persistence failure injection, HTTP/SSE, frontend recovery, history 10개와
  snapshot 32 MiB 제한, restart, 단조 증가 revision, unavailable UI 테스트를
  추가했습니다.
- browser와 MCP 인증 negative matrix, Origin 무변경, 인증 정보 및 문서 비노출,
  로컬 credential 전달 경계를 추가했습니다.
- clean checkout package CI, bundle validation, 공백 절대 경로 실행 smoke test를
  추가했습니다.
- keyboard 편집, 원자적 사각 붙여넣기, server history UI, 접근 가능한 상태,
  reduced motion, safe area, 반응형 화면 증거를 추가했습니다.
- 저장 실패가 기준 memory, revision, undo/redo를 훼손하지 않도록 모든 XLSX 변경과
  history 복원을 candidate에서 처리합니다.
- XLSX 안정 release checklist, DOCX 보존 실험, 공통 제품 명세, 형식별 지원표를
  추가했습니다.
- 교체 가능한 SpreadsheetEngine 뒤의 Univer OSS UI와 Vite, Tailwind CSS build를
  React application 의존성 없이 추가했습니다.
- 사람 cell 저장, AI commit 동기화, 인증 UI asset integration test, Univer와
  ONLYOFFICE 및 자체 엔진 비교 ADR을 추가했습니다.
- 통합 문서, secret, dependency, build artifact ignore 규칙, monorepo command,
  Univer Pro 차단, issue 및 Pull Request workflow를 추가했습니다.
- 공식 MCP Go SDK와 Excelize 기반 service, get_workbook, read_range,
  apply_operations, revision conflict, 원자적 file replacement를 추가했습니다.
- 인증 browser와 MCP endpoint, 직접 cell 편집, AI cursor·typing·commit SSE,
  사람 및 AI range presence, Last-Event-ID replay를 추가했습니다.
- 사각 붙여넣기, 기본 서식, merge, style과 merged-cell rendering, browser와 MCP
  credential 분리, workbook과 batch limit를 추가했습니다.
- MCP App resource, open_workbook, app 전용 협업 도구, browser 및 self-contained
  bundle, stdio transport와 실제 subprocess test를 추가했습니다.
- Claude Desktop과 Codex 설정, host matrix, architecture skill, browser boundary
  validation을 추가했습니다.
- Claude와 Codex의 stdio 읽기, 승인, 쓰기, conflict, reopen 기록과 native MCP App이
  없는 host의 browser fallback을 추가했습니다.
- Codex Browser panel의 실행, 사람 편집, AI presence와 commit, reopen을
  인증했습니다.
- browser와 MCP 편집이 같은 revision, ordered event, XLSX를 공유하는 integration
  test를 추가했습니다.
- 수식, style, merge, validation, conditional formatting, drawing, link, name,
  comment 보존 corpus와 xlsx-compatibility-qa skill을 추가했습니다.
- 전 경계의 행·열 삽입 및 삭제, server 기준 bounded undo/redo, viewport range
  loading, compatibility notice를 추가했습니다.
- list 및 숫자·text·formula·rank·average·date·color scale·data bar·unique·duplicate
  규칙의 OSS Univer mapping과 LibreOffice fixture를 추가했습니다.
- image rendering, bar·line·area·pie·doughnut chart preview, object API를 Univer
  Pro 없이 추가했습니다.
- image와 chart의 이동·크기·삭제, 기존 chart title AI 변경, 사람 drawing transform
  저장과 원격 object 변경 reload를 추가했습니다.

### 변경

- #76 XLSX 실행 취소·다시 실행을 일관된 곡선 화살표 아이콘으로 표시하고
  툴팁·접근성 이름·단축키 안내와 비활성 상태를 유지합니다. 서버 이력과
  연결된 기존 명령을 그대로 사용합니다.

- DOCX 포커스를 본문과 표 경계에 맞추고, 두 편집기의 공통 활동을 타임라인으로 정리했습니다.

- DOCX에 저장·다시 불러오기·보기 배율과 현재 글자 서식을 표시하는 도구 모음을 추가했습니다.

- DOCX 미리보기에 단일 섹션의 원본 용지·여백과 직접 지정 문단·글자 서식을 반영합니다.

- 편집기 상호작용과 실제 브라우저 검증 기준을 AGENTS.md 및 저장소 전용 스킬에 기록했습니다.

- 문서 편집기의 버튼·텍스트 커서와 호버·포커스 피드백을 정리했습니다.
  활동 닫기의 키보드 동작과 AI 강조의 모션 감소·유휴 상태를 반영합니다.

- XLSX 도구 모음을 수식 입력줄 위에 한 줄로 기본 표시합니다. 실행 취소와
  다시 실행은 문서를 다시 열어도 서버 이력에 따라 활성화됩니다.

- XLSX 수식 입력줄을 기본 표시하고 도구와 호환성 안내를 간결하게 정리합니다.
  활동에서 기록된 시트와 셀 범위로 이동할 수 있습니다.

- 문서 편집 헤더를 간결하게 정리하고 DOCX 문단 폭 입력과 키보드 이동,
  조건부 저장 버튼, AI 변경 위치와 이전·이후 미리보기를 제공합니다.

- #47: Univer OSS의 한국어 locale와 앱 호환성·연결·충돌 메시지를 통일했습니다.
  문서 내용과 수식은 번역하지 않고 알 수 없는 원인은 그대로 표시합니다.

- #44: DOCX를 용지·문단·표 중심의 읽기 배치로 바꾸고 입력 테두리를 focus
  중심으로 줄였습니다. 원본 run 편집과 초안·cursor 유지 계약은 보존합니다.

- #42: XLSX와 DOCX의 header·문서명·상태·버튼을 흰색과 indigo 강조색의 공통
  디자인으로 통일했습니다.

- 독립 기능·수정별 커밋과 해당 검증·문서 동반 원칙을 AGENTS.md에 명시하고,
  불가분 변경은 하나로 유지해 형식적인 파일 단위 분리를 피합니다.

- 소유자가 읽는 문서, 변경 이력, GitHub issue 및 Pull Request 양식, 제품 문구의
  기본 언어를 한국어로 바꿨습니다. AI 전용 지침, 코드 주석, commit message,
  명령어, API 식별자는 영어를 유지합니다.
- native Excel 증거, 초대 tester 반복 사용, 배포 조건, 소유자 결정 전까지
  프로젝트를 비공개 로컬 알파로 제한했습니다.
- 자동 release trigger, contents write, 알려진 GitHub Release publisher를
  private-alpha 동안 차단합니다.
- 모든 변경에 issue 번호 branch와 연결 Pull Request를 의무화했습니다.
- 제품과 저장소를 document-workspace로 바꾸고 XLSX를 다중 형식 플랫폼의 첫
  adapter로 재정의했습니다.
- monorepo를 apps, cmd, internal/formats, internal/transport, testdata/xlsx,
  legacy로 정리하며 기존 계약을 유지했습니다.
- MCP App URI, Go module, executable, frontend package, local skill을 새 namespace로
  바꿨습니다.
- Go를 기준 service로 정의하고 Univer Pro 평가를 repository 소유 OSS 전략으로
  교체했습니다.
- Univer preset을 명시적 OSS plugin으로 바꾸고 MCP App bundle을 12.7 MB에서
  약 7.8 MB로 줄였습니다.
- npm에서 pnpm workspace와 정확한 Univer version catalog로 이전했습니다.
- MCP App version, server instruction, tool safety annotation, OpenAI alias를
  추가했습니다.
- browser orchestration, HTTP 및 MCP App transport, UI, workbook mapping, Univer
  command mapping을 TypeScript module로 나눴습니다.
- Go HTTP adapter와 workbook domain을 책임별 파일로 나누면서 route, tool, event,
  revision, 저장 계약을 유지했습니다.
- operation과 event를 required field가 분명한 discriminated union으로 바꿨습니다.
- host의 manual, automatic, one-time, persistent 승인을 문서화하고 image 및 chart
  write-back 한계를 안내에 반영했습니다.

- CI의 main 푸시 중복 검사를 제거하고 PR 변경 범위로 실행할 검사를 선택합니다.
  문서 전용 변경은 무거운 검사를 생략하고 DOCX 전용 변경은 XLSX 전용 회귀·안전성·
  성능 gate 대신 DOCX 검증을 실행합니다. 수동 실행은 전체 검증을 유지합니다.

### 수정

- #86 DOCX 저장이나 실행 취소로 고유 ID가 있는 문단의 위치가 바뀌어도 선택을 같은 문단에 복원합니다.

- #131 DOCX의 추가 도구를 서식·삽입·페이지·복구로 묶고 필요할 때 열 수 있게 했습니다.

- #98 작은 화면에서 DOCX 도구 모음을 별도로 스크롤할 수 있게 하여
  본문을 가리거나 편집 영역 높이를 모두 차지하는 문제를 수정했습니다.

- #80 DOCX의 일반 빈 문단·빈 run에도 커서와 입력칸을 제공합니다. 문단 속성과
  주변 OOXML을 보존하며 보호된 필드·변경 추적 구조는 편집하지 않습니다.

- #69 XLSX 값·수식의 실행 취소와 다시 실행 때 변경된 셀만 갱신해 주변 텍스트가
  깜빡이는 현상을 줄였습니다.

- #68 Delete 또는 Backspace로 XLSX 셀 내용을 지우면 저장 이력에 반영되어
  실행 취소와 다시 실행을 사용할 수 있습니다.

- #66 XLSX 실행 취소와 다시 실행 때 편집기 전체를 다시 열지 않고 시트를 갱신하며
  선택했던 범위와 가로·세로 스크롤 위치를 유지합니다.

- #64 XLSX 글꼴과 크기 메뉴를 키보드로 열고 닫을 수 있게 하고, 좁은 화면의
  서식 더보기 버튼에 접근성 이름을 붙였습니다.

- #63 XLSX 글꼴·크기 입력과 드롭다운 화살표를 하나의 포커스 테두리로
  표시하고 DOCX 보기 도구와 공통 버튼의 호버·누름·포커스 표시를 정리했습니다.

- XLSX 샘플 금액의 채우기와 글자 색상을 바로잡아 본문 대비를 복원했습니다.

- 같은 호스트의 다른 로컬 포트에 열린 DOCX와 XLSX가 브라우저 인증을 서로
  덮어쓰던 문제를 수정했습니다.

- #48: 좁은 화면의 활동 패널·touch target·안전 영역과 focus 복귀를 검증하고,
  DOCX 미지원 문자·요청 크기를 저장 전에 안내해 초안을 유지합니다.

- #45: 자기 저장의 SSE와 응답이 겹쳐 활동이 중복되던 경우를 방지했습니다.

- #43: 자체 도구로 focus를 옮겨도 해당 workbook의 선택과 서식을 참조하고,
  초기 렌더링 뒤 같은 engine에서 고급 ribbon을 전환하도록 보완했습니다.

- #46: 셀을 가리는 고정 경고를 inline 안내로 바꾸고 복구·초안 폐기 확인의
  keyboard focus 이동·취소 복귀와 Escape 동작을 추가했습니다.

- 저장이 확인된 실행 취소·다시 실행 후 재열기에서 불필요한 미저장 이탈 경고가
  나오지 않도록 복구 상태와 확정된 화면 이동을 구분합니다.

- 관련 없는 XLSX 편집 후에도 데이터 유효성 검사의 `showDropDown`,
  `showInputMessage`, `showErrorMessage` 명시 여부와 값을 보존합니다.
- XML 문자 참조로 저장된 데이터 유효성 검사 수식을 디코딩하고, 내용이 같은 저장은
  리비전과 undo 이력을 늘리지 않으며 유휴 browser가 원격 리비전을 동기화합니다.
- clean CI에서 Python MVP 회귀가 동작하도록 dependency를 고정했습니다.
- local edit, AI commit, history restore, reload, recovery 뒤 undo/redo availability를
  동기화했습니다.
- AI formatting과 merge 뒤 browser revision을 갱신해 stale write를 막았습니다.
- 초기 200행 대신 sheet dimension 기반 bounded range로 worksheet를 읽습니다.
- 크거나 미지원 object는 visible notice와 함께 생략해 시작을 막지 않습니다.

### 보안

- 프런트엔드 의존성 검사에서 발견한 MCP client SDK와 Zod의 알려진 취약점을
  수정 버전으로 갱신했습니다.

- credential을 fixed-length hash로 비교하고 framing, resource, capability,
  base URI, object, form 제한을 추가했습니다.
- expanded byte, part size와 수, encrypted·duplicate·unsafe ZIP entry를 검사하는
  XLSX preflight를 추가했습니다.
- Univer를 1.0.2로 올리고 알려진 dependency 취약점이 없음을 확인했습니다.
- 필요한 data와 blob image, font, worker만 허용하고 script는 same-origin으로
  제한했습니다.
- random token, strict cookie, Origin 검사, body와 range 제한, security header를
  추가했습니다.
- stdio browser fallback을 2분 single-use token, loopback, bounded token set으로
  보호했습니다.
- image byte를 검증하고 제한 안의 PNG, JPEG, GIF만 표시합니다.
- history manifest, image dimension, chart series, point, label을 memory load 전에
  제한합니다.

## 0.0.1 - 2026-09-28

### 추가

- 초기 Python XLSX viewer 및 editor 개념 증명을 추가했습니다.
- worksheet tab, 직접 cell 편집, list validation control, 제한된 expression 기반
  conditional formatting을 추가했습니다.
