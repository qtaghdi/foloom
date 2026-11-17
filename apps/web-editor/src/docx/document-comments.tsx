import { createSignal, For, Show } from 'solid-js';
import { buttonClass, quietButtonClass } from '../ui/editor-chrome';
import type { CommentOperation, CommentRange, Paragraph, RevisionReviewOperation, Snapshot } from './contracts';

export function DocumentComments(props:{
  snapshot?:Snapshot;target?:Paragraph;range?:CommentRange;disabled:boolean;pending:boolean;
  onApply:(operation:CommentOperation|RevisionReviewOperation)=>boolean;onLocate:(id:string)=>void;
}) {
  const [open,setOpen]=createSignal(false),[text,setText]=createSignal(''),[error,setError]=createSignal('');
  const [useRange,setUseRange]=createSignal(false);
  const targetText=()=>props.target?.structureEditable?props.target.runs.map(run=>run.text).join('').trim():'';
  const disabled=()=>props.disabled||props.pending;
  const changes=()=>props.snapshot?.blocks.flatMap(block=>block.type==='paragraph'
    ? block.paragraph.runs.filter(run=>run.object?.kind==='tracked_insert'||run.object?.kind==='tracked_delete')
    : []).slice(0,100)??[];
  const review=(type:RevisionReviewOperation['type'],targetId:string)=>{
    if(disabled())return;
    setError(props.onApply({type,targetId})?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
  };
  const add=()=>{
    if(!props.target||!targetText()||!text().trim()||disabled()||useRange()&&!props.range)return;
    if(props.onApply({type:'add_comment',targetId:props.target.id,comment:{text:text(),...(useRange()?props.range:undefined)}})){
      setText('');setError('');
    }else setError('다른 초안을 먼저 저장하거나 실행 취소하세요.');
  };
  return <>
    <button type="button" class={quietButtonClass} aria-expanded={open()} aria-controls={open()?'docx-comments':undefined}
      onClick={()=>setOpen(value=>!value)}>검토</button>
    <Show when={open()}><section id="docx-comments" aria-label="문서 주석"
      class="fixed right-3 top-16 z-50 max-h-[min(70dvh,34rem)] w-[min(24rem,calc(100vw-1.5rem))] overflow-y-auto rounded-lg border border-slate-300 bg-white p-3 text-slate-900 shadow-lg">
      <div class="flex items-center justify-between"><strong>문서 주석</strong><button type="button" class={quietButtonClass} onClick={()=>setOpen(false)}>닫기</button></div>
      <Show when={props.snapshot?.trackedChanges}>
        <section aria-label="변경 추적" class="my-2 border-b border-slate-200 pb-2">
          <h3 class="text-sm font-semibold">변경 추적</h3>
          <Show when={changes().length} fallback={<p class="py-2 text-sm text-slate-600">지원되는 일반 텍스트 변경이 없습니다.</p>}>
            <ul class="space-y-2">
              <For each={changes()}>{change=><li class="rounded border border-slate-200 p-2">
                <p class="text-xs font-semibold">
                  {change.object?.kind==='tracked_insert'?'삽입':'삭제'}
                  {change.object?.author?` · ${change.object.author}`:''}
                </p>
                <p class="whitespace-pre-wrap break-words text-sm">{change.object?.text}</p>
                <Show when={change.object?.editable} fallback={<p class="text-xs text-slate-600">읽기 전용</p>}>
                  <div class="mt-1 flex flex-wrap gap-2">
                    <button type="button" class={quietButtonClass} disabled={disabled()} onClick={()=>review('accept_revision',change.id)}>수락</button>
                    <button type="button" class={quietButtonClass} disabled={disabled()} onClick={()=>review('reject_revision',change.id)}>거절</button>
                  </div>
                </Show>
              </li>}</For>
            </ul>
          </Show>
        </section>
      </Show>
      <p class="py-2 text-xs text-slate-600">{props.pending?'적용한 작업을 저장하거나 실행 취소하세요.':targetText()
        ? `주석 대상 문단: ${targetText().slice(0,80)}`:'내용이 있는 편집 가능한 본문 문단에 커서를 놓으세요.'}</p>
      <label class="mb-2 flex min-h-11 items-center gap-2 text-sm">
        <input type="checkbox" checked={useRange()} onChange={event=>setUseRange(event.currentTarget.checked)} disabled={disabled()}/>
        선택한 텍스트에만 주석 달기
      </label>
      <Show when={useRange()&&!props.range}><p class="mb-2 text-xs text-slate-600">한 본문 문단 안에서 일반 텍스트를 선택하세요.</p></Show>
      <textarea aria-label="새 주석" rows="3" maxlength="1000" class="w-full rounded border border-slate-300 p-2 text-sm"
        disabled={disabled()} value={text()} onInput={event=>setText(event.currentTarget.value)}/>
      <button type="button" class={buttonClass} disabled={disabled()||!targetText()||!text().trim()||useRange()&&!props.range} onClick={add}>주석 적용</button>
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
