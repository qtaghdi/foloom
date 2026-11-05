import { knownMessage, localizeMessage, source } from '../i18n';
import type { FeatureWarning } from '../contracts';

const featureMessages: Record<string, string> = {
  'formula calculation': '수식은 원본에 저장합니다. 서버 값은 기존 계산 결과이거나 없을 수 있으며, 화면의 계산 결과는 Excel 등에서 다시 계산하기 전까지 미리보기입니다.',
  charts: '지원하는 차트의 미리보기·이동·크기·삭제와 AI의 기존 차트 제목 변경을 제공합니다. 차트의 모든 편집 기능을 재현하지는 않습니다.',
  images: '기존 이미지를 표시하고 이동·크기·삭제를 저장합니다. 새 이미지 삽입은 아직 지원하지 않습니다.',
  'conditional formatting': '조건부 서식은 원본에 보존하며 일반 셀·텍스트·순위·색상·데이터 막대 규칙을 표시합니다. 일부 고급 규칙은 화면에 나타나지 않을 수 있습니다.',
  'data validation': '데이터 유효성 검사를 보존하고 일반 목록·숫자·날짜·사용자 수식 규칙을 지원합니다. 미지원 규칙의 화면 동작은 원본 앱과 다를 수 있습니다.',
  'external links': '외부 연결은 원본에 보존하며 브라우저에서 열거나 평가하지 않습니다.',
  macros: '보존 가능한 macro를 유지하지만 실행하지 않습니다.',
  'sheet-objects': '미리보기 제한으로 일부 이미지나 차트가 표시되지 않았습니다. 원본 객체는 문서에 유지됩니다.',
};

export function featureMessage(warning: FeatureWarning): string {
  return featureMessages[warning.feature] ?? source('unknownWarning');
}

export function problemMessage(reason: string): string {
  if (/event history is incomplete|server restarted/i.test(reason)) return '실시간 변경 내역이 끊겼거나 서버가 다시 시작되었습니다.';
  if (/live connection was interrupted|connection lost|network unavailable|failed to fetch|offline/i.test(reason)) return '문서 연결이 끊겼습니다. 연결 상태를 확인하세요.';
  if (/revision conflict|changed outside|external change|newer workbook|writer lock|writer.*busy/i.test(reason)) return '다른 작업이 먼저 저장되었거나 외부 앱이 문서를 변경했습니다.';
  if (/changed while loading|changed while local edits were pending/i.test(reason)) return '문서를 읽거나 입력을 저장하는 동안 다른 변경이 확인되었습니다.';
  if (/document.*closed|unknown.*document|documentId|multiple workbooks/i.test(reason)) return source('targetError');
  if (/unauthorized|forbidden|origin rejected|authentication/i.test(reason)) return source('accessError');
  if (/path|permission denied|no such file|access.*denied/i.test(reason)) return source('pathError');
  return knownMessage(reason) ? reason : source('theDocumentCouldNotBeProcessed');
}

export function warningMessage(message: string): string {
  return knownMessage(message) ? localizeMessage(message) : localizeMessage(source('unknownWarning'));
}
