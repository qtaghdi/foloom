import { createSignal, For, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { BoundaryMode, Paragraph } from './contracts';

export function DocumentBoundaryToolbar(props: {
  target?: { paragraph: Paragraph; canSplit: boolean };
  trailingBreaks: Paragraph[];
  sectionBreaks: {paragraph:Paragraph;earlier?:boolean;later?:boolean}[];
  pending: boolean;
  disabled: boolean;
  onApply: (id: string, mode: BoundaryMode) => boolean;
}) {
  const [error, setError] = createSignal('');
  const [mode, setMode] = createSignal<BoundaryMode>('page_before');
  const [selectedBreak,setSelectedBreak] = createSignal('');
  const usesStoredBreak = () => mode()==='no_page_after'||mode()==='remove_section'||
    mode()==='move_section_earlier'||mode()==='move_section_later';
  const breaks = () => mode()==='no_page_after'?props.trailingBreaks:props.sectionBreaks
    .filter(item=>mode()==='move_section_earlier'?item.earlier:mode()==='move_section_later'?item.later:true)
    .map(item=>item.paragraph);
  const paragraph = () => usesStoredBreak() ? breaks().find(item=>item.id===selectedBreak()) : props.target?.paragraph;
  const available = () => {
    const target = paragraph();
    if (!target) return false;
    if (usesStoredBreak()) return true;
    if (mode() === 'page_before' || mode() === 'no_page_before') {
      return Boolean(target.structureEditable || target.pageBreakBefore);
    }
    return Boolean(target.structureEditable && props.target?.canSplit);
  };
  return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
    <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">페이지·구역 나눔</summary>
    <div class="flex flex-wrap items-center gap-2 pb-2">
      <select aria-label="나눔 유형" class="editor-select min-h-11"
        disabled={props.disabled || props.pending} value={mode()} onChange={event => { setMode(event.currentTarget.value as BoundaryMode); setSelectedBreak(''); setError(''); }}>
        <option value="page_before">문단 앞에 페이지 나눔</option>
        <option value="no_page_before">문단 앞 페이지 나눔 해제</option>
        <option value="no_page_after">문단 끝 페이지 나눔 해제</option>
        <option value="section_next_page">문단 뒤에 다음 페이지 구역 나눔</option>
        <option value="section_even_page">문단 뒤에 다음 짝수 페이지 구역 나눔</option>
        <option value="section_odd_page">문단 뒤에 다음 홀수 페이지 구역 나눔</option>
        <option value="section_continuous">문단 뒤에 연속 구역 나눔</option>
        <option value="remove_section">구역 나눔 제거</option>
        <option value="move_section_earlier">구역 나눔을 앞 문단으로 이동</option>
        <option value="move_section_later">구역 나눔을 뒤 문단으로 이동</option>
      </select>
      <Show when={usesStoredBreak()}>
        <select class="editor-select min-h-11 max-w-64" aria-label={mode()==='no_page_after'?'해제할 페이지 나눔':mode()==='remove_section'?'제거할 구역 나눔':'이동할 구역 나눔'} value={paragraph()?.id ?? ''}
          disabled={props.disabled || props.pending} onChange={event=>{setSelectedBreak(event.currentTarget.value);setError('');}}>
          <option value="">문단 선택</option>
          <For each={breaks()}>{(item,index)=>
            <option value={item.id}>{index()+1}. {item.runs.map(run=>run.text).join('').trim().slice(0,60)||'빈 문단'}</option>
          }</For>
        </select>
        <Show when={mode()==='remove_section'}><p class="basis-full text-slate-600">합친 구역에는 뒤 구역의 설정을 사용합니다.</p></Show>
      </Show>
      <button type="button" class={buttonClass} disabled={props.disabled || props.pending || !available()}
        onClick={() => { const target = paragraph(); if (target) setError(props.onApply(target.id, mode()) ? '' : '다른 초안을 먼저 저장하거나 실행 취소하세요.'); }}>
        나눔 적용
      </button>
      <p class="basis-full text-slate-600">{props.pending ? '나눔을 저장하거나 실행 취소하세요.' : paragraph()
        ? `대상 문단: ${paragraph()?.runs.map(run => run.text).join('').trim().slice(0, 90) || '빈 문단'}`
        : usesStoredBreak() ? '나눔이 있는 문단을 선택하세요.' : '본문 문단에 커서를 놓고 나눔을 선택하세요.'}</p>
      <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
    </div>
  </details>;
}
