import { createEffect, createMemo, createSignal, For, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { RectangleOperation, MathOperation, Paragraph, Run, Snapshot } from './contracts';

const labels:Record<NonNullable<Run['object']>['kind'],string> = {
  math:'수식',shape:'도형',textbox:'텍스트 상자',toc:'목차',figure_list:'그림 목록',table_list:'표 목록',chart:'차트',smartart:'SmartArt',
  control:'콘텐츠 컨트롤',embedded:'내장 개체',drawing:'그리기 개체',footnote:'각주',endnote:'미주',tracked_insert:'추적 삽입',tracked_delete:'추적 삭제',tracked_format:'추적 서식 변경',
};

export function ObjectPreview(props:{object:NonNullable<Run['object']>}) {
  return <Show when={props.object.rectangle} fallback={<span data-docx-opaque contentEditable={false} aria-label={labels[props.object.kind]}
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
    <Show when={props.object.text}>{text=><span classList={{'font-serif':props.object.kind==='math','block whitespace-pre-wrap':['toc','figure_list','table_list'].includes(props.object.kind)}}>{text()}</span>}</Show>
    <Show when={props.object.smartArt}>{diagram=>
      <>
        <span role="list" aria-label="SmartArt 항목" class="block">
          <For each={diagram().items}>{item=><span role="listitem" class="block">{item}</span>}</For>
        </span>
        <Show when={diagram().truncated}><span class="block text-xs text-slate-600">일부 항목이나 문구만 표시됩니다.</span></Show>
      </>
    }</Show>
  </span>}>{rectangle=>
    <span data-docx-rectangle data-docx-opaque contentEditable={false} role="img" aria-label="인라인 사각형"
      class="inline-block max-w-full align-middle" style={{
        width:`${rectangle().widthEmu/9525}px`,'aspect-ratio':`${rectangle().widthEmu}/${rectangle().heightEmu}`,
        'background-color':`#${rectangle().fill}`,border:`1pt solid #${rectangle().stroke}`,
      }}/>
  }</Show>;
}

export function DocumentShapeToolbar(props:{
  snapshot?:Snapshot;target?:Paragraph;pending:boolean;disabled:boolean;onApply:(operation:RectangleOperation)=>boolean;
}) {
  const [id,setID]=createSignal(''),[error,setError]=createSignal('');
  const rectangles=createMemo(()=>props.snapshot?.blocks.flatMap(block=>block.type==='paragraph'
    ?block.paragraph.runs.filter(run=>run.object?.editable&&run.object.rectangle):[]).slice(0,200)??[]);
  const selected=()=>rectangles().find(run=>run.id===id())??rectangles()[0];
  const disabled=()=>props.disabled||props.pending;
  return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
    <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">도형</summary>
    <div class="flex min-w-0 flex-wrap items-center gap-2 pb-2">
      <button type="button" class={buttonClass} disabled={disabled()||!props.target?.structureEditable}
        onClick={()=>{
          const target=props.target;
          if(!target?.structureEditable)return;
          const accepted=props.onApply({type:'insert_rectangle',targetId:target.id});
          setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
        }}>인라인 사각형 삽입</button>
      <Show when={rectangles().length}>
        <select aria-label="편집할 사각형" class="editor-select min-h-11" value={selected()?.id??''}
          disabled={disabled()} onChange={event=>{setID(event.currentTarget.value);setError('');}}>
          <For each={rectangles()}>{(run,index)=><option value={run.id}>사각형 {index()+1}</option>}</For>
        </select>
        <Show when={selected()} keyed>{run=>
          <>
          <form class="flex min-w-0 flex-wrap items-end gap-2" onSubmit={event=>{
            event.preventDefault();
            if(disabled())return;
            const rectangle=run.object?.rectangle;
            if(!rectangle)return;
            const inputs=event.currentTarget.elements;
            const readColor=(name:string)=>(inputs.namedItem(name) as HTMLInputElement).value.slice(1).toUpperCase();
            const fill=readColor('fill'),stroke=readColor('stroke');
            if(fill===rectangle.fill.toUpperCase()&&stroke===rectangle.stroke.toUpperCase())return;
            const accepted=props.onApply({type:'update_rectangle_colors',targetId:run.id,rectangleColors:{fill,stroke}});
            setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
          }}>
            <label class="flex flex-col gap-1">채우기 색
              <input name="fill" type="color" value={`#${run.object!.rectangle!.fill}`} disabled={disabled()}
                class="min-h-11 w-14 rounded border border-slate-300"/>
            </label>
            <label class="flex flex-col gap-1">테두리 색
              <input name="stroke" type="color" value={`#${run.object!.rectangle!.stroke}`} disabled={disabled()}
                class="min-h-11 w-14 rounded border border-slate-300"/>
            </label>
            <button type="submit" class={buttonClass} disabled={disabled()}>사각형 색 적용</button>
            <button type="button" class={buttonClass} disabled={disabled()} onClick={()=>{
              const accepted=props.onApply({type:'delete_rectangle',targetId:run.id});
              setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            }}>사각형 삭제</button>
          </form>
          <Show when={run.object?.rectangleResizeEditable && run.object.rectangle}>{rectangle=>
            <form class="flex min-w-0 flex-wrap items-end gap-2" onSubmit={event=>{
              event.preventDefault();
              if(disabled())return;
              const inputs=event.currentTarget.elements;
              const width=(inputs.namedItem('width') as HTMLInputElement).valueAsNumber;
              const height=(inputs.namedItem('height') as HTMLInputElement).valueAsNumber;
              const current=rectangle();
              const widthEmu=width===Number((current.widthEmu/12700).toFixed(2))?current.widthEmu:Math.round(width*12700);
              const heightEmu=height===Number((current.heightEmu/12700).toFixed(2))?current.heightEmu:Math.round(height*12700);
              if(![widthEmu,heightEmu].every(size=>Number.isFinite(size)&&size>=12700&&size<=18288000)||
                widthEmu===current.widthEmu&&heightEmu===current.heightEmu)return;
              const accepted=props.onApply({type:'resize_rectangle',targetId:run.id,rectangleSize:{widthEmu,heightEmu}});
              setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            }}>
              <label class="flex flex-col gap-1">사각형 너비 (pt)
                <input name="width" type="number" min="1" max="1440" step="any" required disabled={disabled()}
                  value={(rectangle().widthEmu/12700).toFixed(2)} class="min-h-11 w-24 rounded border border-slate-300 px-2"/>
              </label>
              <label class="flex flex-col gap-1">사각형 높이 (pt)
                <input name="height" type="number" min="1" max="1440" step="any" required disabled={disabled()}
                  value={(rectangle().heightEmu/12700).toFixed(2)} class="min-h-11 w-24 rounded border border-slate-300 px-2"/>
              </label>
              <button type="submit" class={buttonClass} disabled={disabled()}>사각형 크기 적용</button>
            </form>
          }</Show>
          </>
        }</Show>
      </Show>
      <Show when={props.pending}><p class="basis-full text-slate-600">저장하거나 실행 취소하세요.</p></Show>
      <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
    </div>
  </details>;
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
