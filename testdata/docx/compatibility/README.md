# DOCX 합성 호환성 corpus

`generate.py` v1은 Python 표준 라이브러리만 사용해 재현 가능한 비민감 DOCX를
생성합니다. 압축 해제 part payload와 대상 밖 XML 보존을 검증하는 자료입니다.
manifest에 생성 정보와 SHA-256을 기록합니다.

```sh
python3 testdata/docx/compatibility/generate.py
make docx-compatibility-test
```

- `generated-basic.docx`: 한글, 반복 text, 공백, 혼합 run 서식, 단순 표
- `generated-preservation.docx`: 위 내용과 머리글, 외부 링크, field, 변경 추적,
  별도 image와 custom XML part. 별도 image part의 보존 검사는 이미지의 시각 배치
  검증을 뜻하지 않습니다.

자동 검사는 문서를 복사해 본문과 표 run을 수정하고 다시 연 뒤 관련 없는 part의
payload hash와 대상 밖 XML byte를 비교합니다. 위험 archive, 손상 XML, 저장 실패,
리비전 충돌은 adapter 테스트가 별도로 생성합니다.

Microsoft Word와 LibreOffice에서 생산한 fixture는 아직 없습니다. 합성 corpus의
Word 읽기와 LibreOffice 자동 재저장·재열기 결과는 품질 검증 안내에 기록했습니다.
실제 문서는 복사본으로 사용합니다.

manifest의 SHA-256·출처 누락과 미등록 DOCX를 CI에서 거부합니다. 서로 다른
내용의 반복 저장과 원본 비변경을 검사하며, native 검증용 출력은
[품질 검증 안내](../../../docs/docx-quality.md)의 전용 절대 경로를 사용합니다.
