import { createEffect, createMemo, createSignal, For, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { MathOperation, Paragraph, Run, Snapshot } from './contracts';

const labels:Record<NonNullable<Run['object']>['kind'],string> = {
  math:'수식',shape:'도형',textbox:'텍스트 상자',chart:'차트',smartart:'SmartArt',
  control:'콘텐츠 컨트롤',embedded:'내장 개체',drawing:'그리기 개체',footnote:'각주',endnote:'미주',tracked_insert:'추적 삽입',tracked_delete:'추적 삭제',tracked_format:'추적 서식 변경',
};

export function ObjectPreview(props:{object:NonNullable<Run['object']>}) {
  return <span data-docx-opaque contentEditable={false} aria-label={labels[props.object.kind]}
    class="inline-block max-w-full break-words rounded border border-slate-300 bg-stone-50 px-1.5 align-middle text-sm">
    <span class="mr-1 text-xs text-slate-600">{labels[props.object.kind]}{props.object.editable?'':' · 읽기 전용'}</span>
    <Show when={props.object.fraction}>{fraction=>
      <span class="inline-grid max-w-full grid-rows-2 text-center align-middle font-serif"
        aria-label={`분자 ${fraction().numerator}, 분모 ${fraction().denominator}`}>
        <span class="border-b border-current px-1">{fraction().numerator}</span>
        <span class="px-1">{fraction().denominator}</span>
      </span>
    }</Show>
    <Show when={props.object.squareRoot}>{text=>
      <span class="inline-flex max-w-full items-start font-serif align-middle" aria-label={`제곱근 ${text()}`}>
        <span aria-hidden="true">√</span><span class="border-t border-current px-0.5">{text()}</span>
      </span>
    }</Show>
    <Show when={props.object.text}>{text=><span classList={{'font-serif':props.object.kind==='math'}}>{text()}</span>}</Show>
  </span>;
}

export function DocumentMathToolbar(props:{
  snapshot?:Snapshot;target?:Paragraph;pending:boolean;disabled:boolean;onApply:(operation:MathOperation)=>boolean;
}) {
  const [action,setAction]=createSignal<MathOperation['math']['action']>('insert');
  const [id,setID]=createSignal(''),[expression,setExpression]=createSignal(''),[error,setError]=createSignal('');
  const [numerator,setNumerator]=createSignal(''),[denominator,setDenominator]=createSignal('');
  const objects=createMemo(()=>{
    const paragraphs=props.snapshot?.blocks.flatMap(block=>block.type==='paragraph'?[block.paragraph]:block.rows.flatMap(row=>row.cells.flatMap(cell=>cell.paragraphs)))??[];
    return paragraphs.flatMap(paragraph=>paragraph.runs.filter(run=>
      run.object?.kind==='math'&&run.object.editable&&
      (action()==='replace_root'?Boolean(run.object.squareRoot):action()==='replace_fraction'?Boolean(run.object.fraction):
        !run.object.squareRoot&&!run.object.fraction))).slice(0,200);
  });
  const selected=()=>objects().find(item=>item.id===id())??objects()[0];
  createEffect(()=>{
    const kind=action(),math=selected()?.object;
    setExpression(kind==='replace' ? math?.text??'' : kind==='replace_root'?math?.squareRoot??'':'');
    setNumerator(kind==='replace_fraction' ? math?.fraction?.numerator??'' : '');
    setDenominator(kind==='replace_fraction' ? math?.fraction?.denominator??'' : '');
    setError('');
  });
  const disabled=()=>props.pending||props.disabled;
  const inserting=()=>action()==='insert'||action()==='insert_fraction'||action()==='insert_root';
  const editingRoot=()=>action()==='insert_root'||action()==='replace_root';
  const editingFraction=()=>action()==='insert_fraction'||action()==='replace_fraction';
  const targetID=()=>inserting()?props.target?.id:selected()?.id;
  const apply=()=>{
    const id=targetID();
    if(!id)return;
    const mode=action();
    let math: MathOperation['math'];
    if (mode==='insert_fraction'||mode==='replace_fraction') {
      if (!numerator().trim()||!denominator().trim()) {setError('분자와 분모를 입력하세요.');return;}
      math={action:mode,fraction:{numerator:numerator(),denominator:denominator()}};
    } else {
      if(mode!=='delete'&&!expression().trim()){setError('수식 텍스트를 입력하세요.');return;}
      math={action:mode,...(mode==='delete'?{}:{expression:expression()})};
    }
    const accepted=props.onApply({type:'update_math',targetId:id,math});
    setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
    if(accepted){setExpression('');setNumerator('');setDenominator('');}
  };
  return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
    <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">수식</summary>
    <div class="flex max-h-[35dvh] min-w-0 flex-wrap items-center gap-2 overflow-y-auto pb-2">
      <select aria-label="수식 작업" class="editor-select min-h-11" disabled={disabled()} value={action()}
        onChange={event=>setAction(event.currentTarget.value as MathOperation['math']['action'])}>
        <option value="insert">단순 수식 삽입</option><option value="insert_fraction">분수 삽입</option><option value="insert_root">제곱근 삽입</option><option value="replace_root">제곱근 바꾸기</option><option value="replace_fraction">분수 바꾸기</option><option value="replace">단순 수식 바꾸기</option><option value="delete">단순 수식 삭제</option>
      </select>
      <Show when={!inserting()}><select aria-label="편집할 수식" class="editor-select min-h-11 max-w-48"
        disabled={disabled()} value={selected()?.id??''} onChange={event=>setID(event.currentTarget.value)}>
        <Show when={!objects().length}><option value="">편집할 수식 없음</option></Show>
        <For each={objects()}>{item=><option value={item.id}>{item.object?.fraction ? item.object.fraction.numerator+' / '+item.object.fraction.denominator : item.object?.squareRoot||item.object?.text||'수식'}</option>}</For>
      </select></Show>
      <Show when={action()!=='delete'&&!editingFraction()}><input type="text" maxlength="256" aria-label={editingRoot()?'피제곱근':'수식 텍스트'} placeholder="x² + y²"
        class="min-h-11 min-w-24 rounded border border-slate-300 px-2" disabled={disabled()} value={expression()}
        onInput={event=>setExpression(event.currentTarget.value)}/></Show>
      <Show when={editingFraction()}>
        <input type="text" maxlength="256" aria-label="분자" placeholder="분자" class="min-h-11 w-24 rounded border border-slate-300 px-2"
          value={numerator()} disabled={disabled()} onInput={event=>setNumerator(event.currentTarget.value)}/>
        <input type="text" maxlength="256" aria-label="분모" placeholder="분모" class="min-h-11 w-24 rounded border border-slate-300 px-2"
          value={denominator()} disabled={disabled()} onInput={event=>setDenominator(event.currentTarget.value)}/>
      </Show>
      <button type="button" class={buttonClass} disabled={disabled()||!targetID()} onClick={apply}>수식 적용</button>
      <p class="basis-full text-slate-600">{props.pending?'저장하거나 실행 취소하세요.':'단순 수식, 분수, 제곱근을 삽입하거나 바꿀 수 있습니다.'}</p>
      <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
    </div>
  </details>;
}
