import { createEffect, createMemo, createSignal, For, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { MathOperation, Paragraph, Run, Snapshot } from './contracts';

const labels:Record<NonNullable<Run['object']>['kind'],string> = {
  math:'수식',shape:'도형',textbox:'텍스트 상자',chart:'차트',smartart:'SmartArt',
  control:'콘텐츠 컨트롤',embedded:'내장 개체',drawing:'그리기 개체',
};

export function ObjectPreview(props:{object:NonNullable<Run['object']>}) {
  return <span data-docx-opaque contentEditable={false} aria-label={labels[props.object.kind]}
    class="inline-block max-w-full break-words rounded border border-slate-300 bg-stone-50 px-1.5 align-middle text-sm">
    <span class="mr-1 text-xs text-slate-600">{labels[props.object.kind]}{props.object.editable?'':' · 읽기 전용'}</span>
    <Show when={props.object.text}>{text=><span classList={{'font-serif':props.object.kind==='math'}}>{text()}</span>}</Show>
  </span>;
}

export function DocumentMathToolbar(props:{
  snapshot?:Snapshot;target?:Paragraph;pending:boolean;disabled:boolean;onApply:(operation:MathOperation)=>boolean;
}) {
  const [action,setAction]=createSignal<MathOperation['math']['action']>('insert');
  const [id,setID]=createSignal(''),[expression,setExpression]=createSignal(''),[error,setError]=createSignal('');
  const objects=createMemo(()=>{
    const paragraphs=props.snapshot?.blocks.flatMap(block=>block.type==='paragraph'?[block.paragraph]:block.rows.flatMap(row=>row.cells.flatMap(cell=>cell.paragraphs)))??[];
    return paragraphs.flatMap(paragraph=>paragraph.runs.filter(run=>run.object?.kind==='math'&&run.object.editable)).slice(0,200);
  });
  const selected=()=>objects().find(item=>item.id===id())??objects()[0];
  createEffect(()=>{const kind=action(),math=selected();if(kind==='replace')setExpression(math?.object?.text??'');else setExpression('');setError('');});
  const disabled=()=>props.pending||props.disabled;
  const targetID=()=>action()==='insert'?props.target?.id:selected()?.id;
  const apply=()=>{
    const id=targetID();
    if(!id)return;
    if(action()!=='delete'&&!expression().trim()){setError('수식 텍스트를 입력하세요.');return;}
    const accepted=props.onApply({type:'update_math',targetId:id,math:{action:action(),...(action()==='delete'?{}:{expression:expression()})}});
    setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
    if(accepted)setExpression('');
  };
  return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
    <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">수식</summary>
    <div class="flex max-h-[35dvh] min-w-0 flex-wrap items-center gap-2 overflow-y-auto pb-2">
      <select aria-label="수식 작업" class="editor-select min-h-11" disabled={disabled()} value={action()}
        onChange={event=>setAction(event.currentTarget.value as MathOperation['math']['action'])}>
        <option value="insert">단순 수식 삽입</option><option value="replace">단순 수식 바꾸기</option><option value="delete">단순 수식 삭제</option>
      </select>
      <Show when={action()!=='insert'}><select aria-label="편집할 수식" class="editor-select min-h-11 max-w-48"
        disabled={disabled()} value={selected()?.id??''} onChange={event=>setID(event.currentTarget.value)}>
        <Show when={!objects().length}><option value="">편집할 수식 없음</option></Show>
        <For each={objects()}>{item=><option value={item.id}>{item.object?.text||'수식'}</option>}</For>
      </select></Show>
      <Show when={action()!=='delete'}><input type="text" maxlength="256" aria-label="수식 텍스트" placeholder="x² + y²"
        class="min-h-11 min-w-24 rounded border border-slate-300 px-2" disabled={disabled()} value={expression()}
        onInput={event=>setExpression(event.currentTarget.value)}/></Show>
      <button type="button" class={buttonClass} disabled={disabled()||!targetID()} onClick={apply}>수식 적용</button>
      <p class="basis-full text-slate-600">{props.pending?'저장하거나 실행 취소하세요.':'단순 인라인 수식을 지원합니다. 분수·행렬 등 원본 구조 수식은 읽기 전용으로 보존합니다.'}</p>
      <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
    </div>
  </details>;
}
