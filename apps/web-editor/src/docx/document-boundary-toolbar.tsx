import { createSignal, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { BoundaryMode, Paragraph } from './contracts';

export function DocumentBoundaryToolbar(props: {
  target?: { paragraph: Paragraph; canSplit: boolean };
  pending: boolean;
  disabled: boolean;
  onApply: (id: string, mode: BoundaryMode) => boolean;
}) {
  const [error, setError] = createSignal('');
  const [mode, setMode] = createSignal<BoundaryMode>('page_before');
  const available = () => {
    const paragraph = props.target?.paragraph;
    if (!paragraph) return false;
    if (mode() === 'page_before' || mode() === 'no_page_before') {
      return Boolean(paragraph.structureEditable || paragraph.pageBreakBefore);
    }
    return Boolean(paragraph.structureEditable && props.target?.canSplit);
  };
  return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
    <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">페이지·구역 나눔</summary>
    <div class="flex flex-wrap items-center gap-2 pb-2">
      <select aria-label="나눔 유형" class="editor-select min-h-11"
        disabled={props.disabled || props.pending} value={mode()} onChange={event => { setMode(event.currentTarget.value as BoundaryMode); setError(''); }}>
        <option value="page_before">문단 앞에 페이지 나눔</option>
        <option value="no_page_before">문단 앞 페이지 나눔 해제</option>
        <option value="section_next_page">문단 뒤에 다음 페이지 구역 나눔</option>
        <option value="section_continuous">문단 뒤에 연속 구역 나눔</option>
      </select>
      <button type="button" class={buttonClass} disabled={props.disabled || props.pending || !available()}
        onClick={() => { if (props.target) setError(props.onApply(props.target.paragraph.id, mode()) ? '' : '다른 초안을 먼저 저장하거나 실행 취소하세요.'); }}>
        나눔 적용
      </button>
      <p class="basis-full text-slate-600">{props.pending ? '나눔을 저장하거나 실행 취소하세요.' : props.target
        ? `대상 문단: ${props.target.paragraph.runs.map(run => run.text).join('').trim().slice(0, 90) || '빈 문단'}`
        : '본문 문단에 커서를 놓고 나눔을 선택하세요.'}</p>
      <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
    </div>
  </details>;
}
