import { createSignal, For, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { ParagraphMoveOperation, Paragraph } from './contracts';

export function DocumentParagraphMoveToolbar(props:{
  paragraphs: Paragraph[];
  kind: 'bookmark'|'hyperlink'|'note';
  pending: boolean;
  disabled: boolean;
  onApply:(operation:ParagraphMoveOperation)=>boolean;
}) {
  const [id,setID]=createSignal(''),[error,setError]=createSignal('');
  const selected=()=>props.paragraphs.find(paragraph=>paragraph.id===id());
  const disabled=()=>props.disabled||props.pending;
  const label={bookmark:'북마크',hyperlink:'링크',note:'각주·미주'}[props.kind];
  const canMove=(direction:'up'|'down')=>{
    const paragraph=selected();
    if (!paragraph) return false;
    switch(props.kind){
      case 'bookmark': return direction==='up'?paragraph.moveBookmarkUp:paragraph.moveBookmarkDown;
      case 'hyperlink': return direction==='up'?paragraph.moveHyperlinkUp:paragraph.moveHyperlinkDown;
      case 'note': return direction==='up'?paragraph.moveNoteUp:paragraph.moveNoteDown;
    }
  };
  const move=(direction:'up'|'down')=>{
    const type = `move_${props.kind}_${direction}` as const;
    const target=selected();
    if (!target||disabled()||!canMove(direction)) return;
    setError(props.onApply({type,targetId:target.id})?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
  };
  return <Show when={props.paragraphs.length}>
    <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
      <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">{`${label}가 있는 문단 이동`}</summary>
      <div class="flex min-w-0 flex-wrap items-center gap-2 pb-2">
        <select aria-label={`이동할 ${label} 문단`} class="editor-select min-h-11 max-w-64" disabled={disabled()}
          value={selected()?.id??''} onChange={event=>{setID(event.currentTarget.value);setError('');}}>
          <option value="">문단 선택</option>
          <For each={props.paragraphs}>{(paragraph,index)=>
            <option value={paragraph.id}>{index()+1}. {paragraph.runs.map(run=>run.text).join('').trim().slice(0,60)||'빈 문단'}</option>
          }</For>
        </select>
        <button type="button" class={buttonClass} disabled={disabled()||!canMove('up')}
          onClick={()=>move('up')}>앞 문단 위로 이동</button>
        <button type="button" class={buttonClass} disabled={disabled()||!canMove('down')}
          onClick={()=>move('down')}>뒤 문단 아래로 이동</button>
        <Show when={props.pending}><p class="basis-full text-slate-600">이동을 저장하거나 실행 취소하세요.</p></Show>
        <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
      </div>
    </details>
  </Show>;
}
