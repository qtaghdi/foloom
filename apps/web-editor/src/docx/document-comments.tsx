import { createSignal, For, Show } from 'solid-js';
import { buttonClass, quietButtonClass } from '../ui/editor-chrome';
import type { CommentOperation, Paragraph, Snapshot } from './contracts';

export function DocumentComments(props:{
  snapshot?:Snapshot;target?:Paragraph;disabled:boolean;pending:boolean;
  onApply:(operation:CommentOperation)=>boolean;onLocate:(id:string)=>void;
}) {
  const [open,setOpen]=createSignal(false),[text,setText]=createSignal(''),[error,setError]=createSignal('');
  const targetText=()=>props.target?.structureEditable?props.target.runs.map(run=>run.text).join('').trim():'';
  const disabled=()=>props.disabled||props.pending;
  const add=()=>{
    if(!props.target||!targetText()||!text().trim()||disabled())return;
    if(props.onApply({type:'add_comment',targetId:props.target.id,comment:{text:text()}})){
      setText('');setError('');
    }else setError('다른 초안을 먼저 저장하거나 실행 취소하세요.');
  };
  return <>
    <button type="button" class={quietButtonClass} aria-expanded={open()} aria-controls={open()?'docx-comments':undefined}
      onClick={()=>setOpen(value=>!value)}>검토</button>
    <Show when={open()}><section id="docx-comments" aria-label="문서 주석"
      class="fixed right-3 top-16 z-50 max-h-[min(70dvh,34rem)] w-[min(24rem,calc(100vw-1.5rem))] overflow-y-auto rounded-lg border border-slate-300 bg-white p-3 text-slate-900 shadow-lg">
      <div class="flex items-center justify-between"><strong>문서 주석</strong><button type="button" class={quietButtonClass} onClick={()=>setOpen(false)}>닫기</button></div>
      <Show when={props.snapshot?.trackedChanges}><p class="rounded bg-amber-50 p-2 text-sm text-amber-950">원본 변경 추적은 읽기 전용이며 그대로 보존합니다.</p></Show>
      <p class="py-2 text-xs text-slate-600">{props.pending?'적용한 작업을 저장하거나 실행 취소하세요.':targetText()
        ? `주석 대상 문단: ${targetText().slice(0,80)}`:'내용이 있는 편집 가능한 본문 문단에 커서를 놓으세요.'}</p>
      <textarea aria-label="새 주석" rows="3" maxlength="1000" class="w-full rounded border border-slate-300 p-2 text-sm"
        disabled={disabled()} value={text()} onInput={event=>setText(event.currentTarget.value)}/>
      <button type="button" class={buttonClass} disabled={disabled()||!targetText()||!text().trim()} onClick={add}>주석 적용</button>
      <Show when={error()}><p role="alert" class="py-2 text-sm text-red-700">{error()}</p></Show>
      <ul class="mt-3 space-y-2" aria-label="저장된 주석">
        <For each={props.snapshot?.comments??[]}>{comment=><li class="rounded border border-slate-200 p-2">
          <p class="text-xs font-semibold">{comment.author||'작성자 미상'} <Show when={comment.date}>{date=><time dateTime={date()}>{date()}</time>}</Show></p>
          <p class="whitespace-pre-wrap break-words text-sm">{comment.text}</p>
          <Show when={comment.paragraphId}>{id=><button type="button" class={quietButtonClass} onClick={()=>props.onLocate(id())}>주석 위치로 이동</button>}</Show>
        </li>}</For>
      </ul>
    </section></Show>
  </>;
}
