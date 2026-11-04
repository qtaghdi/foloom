import type { Operation } from './contracts';

export function utf8Length(text: string): number {
  let bytes = 0;
  for (const character of text) {
    const code = character.codePointAt(0)!;
    bytes += code <= 0x7f ? 1 : code <= 0x7ff ? 2 : code <= 0xffff ? 3 : 4;
  }
  return bytes;
}
export function validateRunText(text: string): string {
  if (/[\u0000-\u001f\ufffe\uffff\ud800-\udfff]/u.test(text)) return '이 입력에는 줄바꿈·탭 또는 지원하지 않는 문자가 있습니다. 내용을 복사한 뒤 한 줄 텍스트로 수정하세요.';
  if (utf8Length(text) > 64 * 1024) return '한 입력은 UTF-8 64 KiB 이하여야 합니다. 내용을 줄인 뒤 저장하세요.';
  return '';
}
export function validateDrafts(sessionId: string, revision: number, operations: Operation[]): string {
  if (operations.length > 1000) return '한 번에 1,000개 입력까지 저장할 수 있습니다. 수정 범위를 줄이세요.';
  let textBytes = 0;
  for (const operation of operations) {
    const error = validateRunText(operation.text); if (error) return error;
    textBytes += utf8Length(operation.text) + utf8Length(operation.expectedText);
    if (textBytes > 1024 * 1024) return '한 번에 저장할 수 있는 범위를 넘었습니다. 변경을 나누어 저장하세요.';
  }
  if (utf8Length(JSON.stringify({ sessionId, baseRevision: revision, operations })) > 1024 * 1024) return '저장 요청이 1 MiB를 넘습니다. 변경을 나누어 저장하세요.';
  return '';
}
