# XLSX 호환성 corpus

이 디렉터리에는 document-workspace의 XLSX 저장 과정에서 기능 손실을 찾기 위한
비민감 통합 문서 fixture가 있습니다. 테스트는 항상 임시 디렉터리에 복사한 뒤
편집합니다.

## Fixture

### `feature-rich.xlsx`

- 출처: `go run ./testdata/xlsx/compatibility/generate`로 생성
- 생성기: Go 1.25, Excelize 2.11.0
- 색상 교정: 생성기에 6자리 RGB를 전달하고, 재생성한 스타일 정의만 교체했습니다.
- 기능: 수식, 외부 workbook 수식 reference, styled cell, merge, inline 및 range
  list validation, prompt와 error가 있는 whole-number validation, date 및 custom
  formula validation, 숫자와 text highlight, three-color scale, data bar, duplicate
  formatting, chart, embedded PNG, 외부 hyperlink, workbook defined name, cell comment
- 의도한 편집: `xlsx.Session`으로 관련 없는 `Z50` marker 변경

이 fixture는 Excelize 기준이며 Microsoft Excel, LibreOffice 등 다른 앱이 만든
파일의 완전 호환을 입증하지 않습니다. timestamp와 생성 metadata 때문에 재생성
파일의 byte가 달라질 수 있습니다.

```bash
go run ./testdata/xlsx/compatibility/generate
```

기능 목록을 의도적으로 바꿀 때만 재생성하고 같은 변경에서 통합 문서와 assertion을
함께 검토합니다. 설명되지 않은 회귀를 받아들이기 위해 fixture를 교체하지 않습니다.

### `libreoffice.xlsx`

- 출처: 2026-10-01 LibreOfficeDev 26.8.0.0.alpha0의 Calc Office Open XML
  filter로 열고 export
- 원본: `feature-rich.xlsx` 복사본
- 기능: 해당 producer가 유지한 수식, style, merge, validation, conditional
  formatting, drawing, link, name, comment subset
- 의도한 편집: `xlsx.Session`으로 관련 없는 `Z50` marker 변경

이 fixture는 실제 LibreOffice producer의 round-trip 안정성을 검사합니다.
Microsoft Excel fixture는 여전히 필요합니다. 실제 producer 버전과 출처 없이
생성 또는 third-party 파일을 Microsoft Excel 출력으로 표시하지 않습니다.

`validation_preservation_test.go`는 Excel 호환 OOXML boolean 표현인 `0`과 `1`을
사용해 `showDropDown`, `showInputMessage`, `showErrorMessage`의 명시 조합을 package
수준에서 구성합니다. 실제 `xlsx.Session`으로 관련 없는 셀을 편집한 뒤 속성의
명시 여부와 값, Excelize reopen을 검사합니다. 이 테스트는 Microsoft Excel이 만든
fixture나 native Excel reopen 증거를 대신하지 않습니다.

```bash
make compatibility-test
```
