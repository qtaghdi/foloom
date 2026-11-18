import { createSignal, For, Show } from 'solid-js';
import { buttonClass, quietButtonClass } from '../ui/editor-chrome';
import type { CommentOperation, CommentRange, Paragraph, RevisionReviewOperation, Snapshot, TrackTextOperation } from './contracts';
import { utf8Length } from './validation';

const paragraphAlignments={start:'시작',center:'가운데',end:'끝',justify:'양쪽'} as const;
type ParagraphAlignment=keyof typeof paragraphAlignments;

export function DocumentComments(props:{
  snapshot?:Snapshot;target?:Paragraph;range?:CommentRange;disabled:boolean;pending:boolean;
  onApply:(operation:CommentOperation|RevisionReviewOperation|TrackTextOperation)=>boolean;onLocate:(id:string)=>void;
}) {
  const [open,setOpen]=createSignal(false),[text,setText]=createSignal(''),[error,setError]=createSignal('');
  const [useRange,setUseRange]=createSignal(false);
  const targetText=()=>props.target?.structureEditable?props.target.runs.map(run=>run.text).join('').trim():'';
  const disabled=()=>props.disabled||props.pending;
  const changes=()=>props.snapshot?.blocks.flatMap(block=>{
    if(block.type!=='paragraph')return [];
    const paragraph=block.paragraph;
    const runs=paragraph.runs.filter(run=>run.object?.kind==='tracked_insert'||run.object?.kind==='tracked_delete'||run.object?.kind==='tracked_format');
    return [...(paragraph.alignmentRevision?[{id:paragraph.id,object:paragraph.alignmentRevision}]:[]),...runs];
  }).slice(0,100)??[];
  const [alignment,setAlignment]=createSignal<ParagraphAlignment>('center');
  const currentAlignment=()=>props.target?.layout?.align??'start';
  const [replyTo,setReplyTo]=createSignal('');
  const comments=()=>props.snapshot?.comments??[];
  const threads=()=>comments().filter(comment=>!comment.parentId||!comments().some(parent=>parent.id===comment.parentId));
  const replies=(parentId:string)=>comments().filter(comment=>comment.parentId===parentId);
  const [trackId,setTrackId]=createSignal('');
  const availableRuns=()=>props.target?.runs.filter(run=>run.trackable).slice(0,200)??[];
  const trackRun=()=>availableRuns().find(run=>run.id===trackId());
  const applyReview=(operation:CommentOperation|RevisionReviewOperation|TrackTextOperation)=>{
    if(disabled())return;
    const accepted=props.onApply(operation);
    setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
    if(accepted)setOpen(false);
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
      <Show when={availableRuns().length}>
        <details class="my-2 border-b border-slate-200 pb-2">
          <summary class="min-h-11 cursor-pointer content-center text-sm font-semibold focus-visible:outline-2 focus-visible:outline-teal-700">텍스트·서식 변경 추적</summary>
          <select aria-label="추적할 텍스트" value={trackRun()?.id??''} disabled={disabled()}
            class="editor-select min-h-11 max-w-full" onChange={event=>{setTrackId(event.currentTarget.value);setError('');}}>
            <option value="">텍스트 선택</option>
            <For each={availableRuns()}>{run=><option value={run.id}>{run.text.slice(0,80)}</option>}</For>
          </select>
          <Show when={trackRun()} keyed>{run=>
            <form class="my-2 grid gap-2" onSubmit={event=>{
              event.preventDefault();
              if(disabled())return;
              const field=event.currentTarget.elements.namedItem('replacement') as HTMLInputElement;
              const replacement=field.value;
              if(replacement===run.text)return;
              if(utf8Length(replacement)>1024){setError('텍스트를 UTF-8 1 KiB 이하로 줄이세요.');return;}
              applyReview({type:'track_run_text',targetId:run.id,expectedText:run.text,text:replacement});
            }}>
              <label class="grid gap-1 text-sm">새 텍스트
                <input type="text" name="replacement" maxlength="1024" value={run.text} disabled={disabled()}
                  class="min-h-11 rounded border border-slate-300 px-2"/>
              </label>
              <p class="text-xs text-slate-600">텍스트를 비우면 삭제로 기록합니다.</p>
              <button type="submit" class={buttonClass} disabled={disabled()}>변경 추적 적용</button>
              <Show when={run.boldTrackable}>
                <button type="button" class={quietButtonClass} disabled={disabled()} onClick={()=>
                  applyReview({type:'track_run_bold',targetId:run.id,expectedText:run.text,bold:!run.bold})}>
                  {run.bold?'굵게 해제 기록':'굵게 적용 기록'}
                </button>
              </Show>
            </form>
          }</Show>
        </details>
      </Show>
      <Show when={props.target?.alignmentTrackable&&props.target} keyed>{paragraph=>
        <form class="my-2 flex flex-wrap items-end gap-2 border-b border-slate-200 pb-2" onSubmit={event=>{
          event.preventDefault();
          if(alignment()!==currentAlignment()){
            applyReview({type:'track_paragraph_alignment',targetId:paragraph.id,alignment:alignment()});
          }
        }}>
          <label class="grid gap-1 text-sm">추적할 문단 정렬
            <select class="editor-select min-h-11" value={alignment()} disabled={disabled()}
              onChange={event=>setAlignment(event.currentTarget.value as ParagraphAlignment)}>
              <For each={Object.entries(paragraphAlignments)}>{([value,name])=><option value={value}>{name}</option>}</For>
            </select>
          </label>
          <button type="submit" class={buttonClass} disabled={disabled()||alignment()===currentAlignment()}>문단 정렬 변경 기록</button>
        </form>
      }</Show>
      <Show when={props.snapshot?.trackedChanges}>
        <section aria-label="변경 추적" class="my-2 border-b border-slate-200 pb-2">
          <h3 class="text-sm font-semibold">변경 추적</h3>
          <Show when={changes().length} fallback={<p class="py-2 text-sm text-slate-600">검토할 수 있는 변경이 없습니다.</p>}>
            <ul class="space-y-2">
              <For each={changes()}>{change=><li class="rounded border border-slate-200 p-2">
                <p class="text-xs font-semibold">
                  {change.object?.label??(change.object?.kind==='tracked_format'?'서식 변경':change.object?.kind==='tracked_insert'?'삽입':'삭제')}
                  {change.object?.author?` · ${change.object.author}`:''}
                  <Show when={change.object?.date}>{date=><time dateTime={date()}> · {new Date(date()).toLocaleString()}</time>}</Show>
                </p>
                <p class="whitespace-pre-wrap break-words text-sm">{change.object?.text}</p>
                <Show when={change.object?.editable} fallback={<p class="text-xs text-slate-600">읽기 전용</p>}>
                  <div class="mt-1 flex flex-wrap gap-2">
                    <button type="button" class={quietButtonClass} disabled={disabled()} onClick={()=>applyReview({type:'accept_revision',targetId:change.id})}>수락</button>
                    <button type="button" class={quietButtonClass} disabled={disabled()} onClick={()=>applyReview({type:'reject_revision',targetId:change.id})}>거절</button>
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
        <For each={threads()}>{comment=><li class="rounded border border-slate-200 p-2">
          <p class="text-xs font-semibold">{comment.author||'작성자 미상'} <Show when={comment.date}>{date=><time dateTime={date()}>{date()}</time>}</Show></p>
          <p class="whitespace-pre-wrap break-words text-sm">{comment.text}</p>
          <Show when={comment.resolutionEditable}>
            <p class="text-xs text-slate-600">{comment.resolved?'해결됨':'열려 있음'}</p>
            <button type="button" class={quietButtonClass} disabled={disabled()} onClick={()=>
              applyReview({type:comment.resolved?'reopen_comment':'resolve_comment',targetId:comment.id})}>
              {comment.resolved?'주석 다시 열기':'주석 해결'}
            </button>
          </Show>
          <Show when={comment.paragraphId}>{id=><button type="button" class={quietButtonClass} onClick={()=>props.onLocate(id())}>주석 위치로 이동</button>}</Show>
          <Show when={comment.resolutionEditable}>
            <button type="button" class={quietButtonClass} disabled={disabled()} onClick={()=>setReplyTo(id=>id===comment.id?'':comment.id)}>답글 쓰기</button>
          </Show>
          <Show when={replyTo()===comment.id}>
            <form class="mt-2 grid gap-2" onSubmit={event=>{
              event.preventDefault();
              if(disabled())return;
              const input=event.currentTarget.elements.namedItem('replyText') as HTMLTextAreaElement;
              if(!input.value.trim()){setError('답글 내용을 입력하세요.');return;}
              if(utf8Length(input.value)>4000){setError('답글을 UTF-8 4,000바이트 이하로 줄이세요.');return;}
              const accepted=props.onApply({type:'reply_comment',targetId:comment.id,comment:{text:input.value}});
              setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
              if(accepted){setReplyTo('');setOpen(false);}
            }}>
              <textarea name="replyText" aria-label="새 답글" rows="2" maxlength="1000" disabled={disabled()}
                class="w-full rounded border border-slate-300 p-2 text-sm"/>
              <button type="submit" class={buttonClass} disabled={disabled()}>답글 적용</button>
            </form>
          </Show>
          <Show when={replies(comment.id).length}>
            <ul aria-label="답글" class="mt-2 space-y-2 border-l border-slate-200 pl-3">
              <For each={replies(comment.id)}>{reply=><li>
                <p class="text-xs font-semibold">{reply.author||'작성자 미상'} <Show when={reply.date}>{date=><time dateTime={date()}>{date()}</time>}</Show></p>
                <p class="whitespace-pre-wrap break-words text-sm">{reply.text}</p>
              </li>}</For>
            </ul>
          </Show>
        </li>}</For>
      </ul>
    </section></Show>
  </>;
}
