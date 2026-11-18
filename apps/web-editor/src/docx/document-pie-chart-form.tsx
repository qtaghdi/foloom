import { createSignal, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { ObjectTextOperation, Paragraph } from './contracts';
import { utf8Length, validChartValue } from './validation';

export function DocumentPieChartForm(props: {
  target?: Paragraph;
  disabled: boolean;
  onApply: (operation: ObjectTextOperation) => boolean;
}) {
  const [error, setError] = createSignal('');
  const disabled = () => props.disabled || !props.target?.structureEditable;
  return <form class="flex basis-full flex-wrap items-end gap-2" onSubmit={event => {
    event.preventDefault();
    const target = props.target;
    if (disabled() || !target) return;
    const fields = new FormData(event.currentTarget);
    const title = String(fields.get('title') ?? '').trim();
    const rows = String(fields.get('points') ?? '').split('\n').map(row => row.trim()).filter(Boolean);
    if (!title || utf8Length(title) > 256 || rows.length < 1 || rows.length > 8) {
      setError('제목은 UTF-8 256바이트까지 입력하고 항목은 1~8개를 적으세요.');
      return;
    }
    const points = rows.map(row => {
      const comma = row.lastIndexOf(',');
      return comma < 0 ? {label: '', value: ''} : {
        label: row.slice(0, comma).trim(),
        value: row.slice(comma + 1).trim(),
      };
    });
    if (points.some(point => !point.label || utf8Length(point.label) > 120 ||
      !validChartValue(point.value) || Number(point.value) < 0) || !points.some(point => Number(point.value) > 0)) {
      setError('각 줄에 항목과 0 이상의 값을 쉼표로 구분하세요. 양수 값이 하나 이상 필요합니다.');
      return;
    }
    const accepted = props.onApply({type: 'insert_chart', targetId: target.id, chartInsert: {title, points}});
    setError(accepted ? '' : '다른 초안을 먼저 저장하거나 실행 취소하세요.');
    if (accepted) event.currentTarget.reset();
  }}>
    <label class="flex min-w-0 flex-col gap-1">원형 차트 제목
      <input name="title" type="text" maxlength="256" required disabled={disabled()}
        class="min-h-11 rounded border border-slate-300 px-2"/>
    </label>
    <label class="flex min-w-0 flex-col gap-1">차트 항목과 값
      <textarea name="points" rows="3" maxlength="2048" required disabled={disabled()}
        placeholder={'사과, 3\n배, 5'} class="rounded border border-slate-300 p-2"/>
    </label>
    <button type="submit" class={buttonClass} disabled={disabled()}>원형 차트 삽입</button>
    <p class="basis-full text-slate-600">한 줄에 항목과 값을 쉼표로 구분해 최대 8개를 입력하세요.</p>
    <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
  </form>;
}
