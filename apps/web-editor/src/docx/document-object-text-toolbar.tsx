import { createSignal, For, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { ObjectTextOperation, Paragraph, Snapshot } from './contracts';
import { utf8Length, validChartValue } from './validation';
import { DocumentChartForm } from './document-chart-form';

const objectLabels={
  control:{title:'콘텐츠 컨트롤 편집',select:'편집할 콘텐츠 컨트롤',name:'컨트롤',text:'콘텐츠 컨트롤 텍스트'},
  textbox:{title:'텍스트 상자 편집',select:'편집할 텍스트 상자',name:'텍스트 상자',text:'텍스트 상자 내용'},
  chart:{title:'차트 편집',select:'편집할 차트',name:'차트',text:'차트 제목'},
};

export function DocumentObjectTextToolbar(props:{
  snapshot?:Snapshot;
  target?:Paragraph;
  kind:'control'|'textbox'|'chart';
  pending:boolean;
  disabled:boolean;
  onApply:(operation:ObjectTextOperation)=>boolean;
}) {
  const [id,setID]=createSignal(''),[error,setError]=createSignal('');
  const [point,setPoint]=createSignal(0);
  const [chartFormOpened,setChartFormOpened]=createSignal(false);
  const labels=()=>objectLabels[props.kind];
  const objects=()=>props.snapshot?.blocks.flatMap(block=>block.type==='paragraph'
    ? block.paragraph.runs.filter(run=>run.object?.kind===props.kind&&
      (run.object.editable||props.kind==='chart'&&run.object.deleteEditable))
    : []).slice(0,200)??[];
  const selected=()=>objects().find(run=>run.id===id());
  const disabled=()=>props.pending||props.disabled;
  const maxBytes=()=>props.kind==='chart'?256:1024;
  const insertable=()=>props.kind==='textbox'||props.kind==='control';
  const insertionLabel=()=>props.kind==='control'?'콘텐츠 컨트롤':'텍스트 상자';
  return <Show when={objects().length||insertable()||props.kind==='chart'}>
    <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6"
      onToggle={event=>{if(props.kind==='chart'&&event.currentTarget.open)setChartFormOpened(true);}}>
      <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">{labels().title}</summary>
      <div class="flex min-w-0 flex-wrap items-end gap-2 pb-2">
        <Show when={props.kind==='chart'&&chartFormOpened()}>
          <DocumentChartForm target={props.target} disabled={disabled()} onApply={props.onApply}/>
        </Show>
        <Show when={insertable()}>
          <form class="flex basis-full flex-wrap items-end gap-2" onSubmit={event=>{
            event.preventDefault();
            const target=props.target;
            if(disabled()||!target?.structureEditable)return;
            const input=event.currentTarget.elements.namedItem('newObjectText') as HTMLInputElement;
            const text=input.value;
            if(!text.trim()){setError(`${insertionLabel()} 내용을 입력하세요.`);return;}
            if(utf8Length(text)>1024){setError('텍스트를 UTF-8 1 KiB 이하로 줄이세요.');return;}
            const type=props.kind==='control'?'insert_control_text':'insert_textbox';
            const accepted=props.onApply({type,targetId:target.id,expectedText:'',text});
            setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            if(accepted)input.value='';
          }}>
            <label class="flex min-w-0 flex-col gap-1">새 {insertionLabel()} 내용
              <input name="newObjectText" type="text" maxlength="1024" class="min-h-11 rounded border border-slate-300 px-2"
                disabled={disabled()||!props.target?.structureEditable}/>
            </label>
            <button type="submit" class={buttonClass} disabled={disabled()||!props.target?.structureEditable}>{insertionLabel()} 삽입</button>
          </form>
        </Show>
        <Show when={objects().length}>
        <select aria-label={labels().select} class="editor-select min-h-11 max-w-56"
          value={selected()?.id??''} disabled={disabled()} onChange={event=>{setID(event.currentTarget.value);setPoint(0);setError('');}}>
          <option value="">{labels().name} 선택</option>
          <For each={objects()}>{(run,index)=>
            <option value={run.id}>{index()+1}. {run.object?.label||run.object?.text?.slice(0,60)||`빈 ${labels().name}`}</option>
          }</For>
        </select>
        <Show when={selected()} keyed>{run=>
          <>
          <Show when={run.object?.editable}>
          <form class="flex flex-wrap items-end gap-2" onSubmit={event=>{
            event.preventDefault();
            if (disabled()) return;
            const input=event.currentTarget.elements.namedItem('objectText') as HTMLInputElement;
            const text=input.value,expectedText=run.object?.text??'';
            if(text===expectedText)return;
            if(props.kind==='chart'&&!text.trim()){setError('차트 제목을 입력하세요.');return;}
            if(utf8Length(text)>maxBytes()){setError(`텍스트를 UTF-8 ${maxBytes()}바이트 이하로 줄이세요.`);return;}
            const type=props.kind==='chart'?'update_chart_title':props.kind==='control'?'update_control_text':'update_textbox_text';
            const accepted=props.onApply({type,targetId:run.id,expectedText,text});
            setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
          }}>
            <label class="flex min-w-0 flex-col gap-1">{labels().text}
              <input name="objectText" type="text" maxlength={maxBytes()} value={run.object?.text??''} disabled={disabled()}
                class="min-h-11 rounded border border-slate-300 px-2"/>
            </label>
            <button type="submit" class={buttonClass} disabled={disabled()}>{props.kind==='chart'?'차트 제목 적용':`${labels().name} 내용 적용`}</button>
          </form>
          </Show>
          <Show when={run.object?.chartFrame}>{frame=>
            <form class="flex flex-wrap items-end gap-2" onSubmit={event=>{
              event.preventDefault();
              if(disabled())return;
              const input=event.currentTarget.elements.namedItem('chartWidth') as HTMLInputElement;
              if(input.value===(frame().widthEmu/12700).toFixed(2))return;
              const chartWidthEmu=Math.round(input.valueAsNumber*12700);
              if(!Number.isFinite(chartWidthEmu)||chartWidthEmu<9525||chartWidthEmu>18288000||
                chartWidthEmu===frame().widthEmu)return;
              const accepted=props.onApply({type:'resize_chart',targetId:run.id,chartWidthEmu});
              setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            }}>
              <label class="flex min-w-0 flex-col gap-1">차트 너비 (pt)
                <input name="chartWidth" type="number" min=".75" max="1440" step="any" required
                  value={(frame().widthEmu/12700).toFixed(2)} disabled={disabled()}
                  class="min-h-11 w-24 rounded border border-slate-300 px-2"/>
              </label>
              <span class="self-center text-slate-600">현재 높이 {(frame().heightEmu/12700).toFixed(2)} pt</span>
              <button type="submit" class={buttonClass} disabled={disabled()}>차트 비율 유지 크기 변경</button>
            </form>
          }</Show>
          <Show when={run.object?.chartValues}>{values=>
            <form class="flex flex-wrap items-end gap-2" onSubmit={event=>{
              event.preventDefault();
              if(disabled())return;
              const input=event.currentTarget.elements.namedItem('chartValue') as HTMLInputElement;
              const value=input.value.trim(),index=point(),expectedValue=values()[index];
              if(expectedValue===undefined||value===expectedValue)return;
              if(!validChartValue(value)){
                setError('절댓값이 10¹⁵ 이하인 숫자를 입력하세요.');
                return;
              }
              const accepted=props.onApply({type:'update_chart_value',targetId:run.id,chartValue:{point:index,expectedValue,value}});
              setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            }}>
              <label class="flex min-w-0 flex-col gap-1">편집할 차트 데이터
                <select aria-label="편집할 차트 데이터" class="editor-select min-h-11 max-w-56"
                  value={point()} disabled={disabled()} onChange={event=>{setPoint(Number(event.currentTarget.value));setError('');}}>
                  <For each={values()}>{(value,index)=><option value={index()}>값 {index()+1} · {value}</option>}</For>
                </select>
              </label>
              <Show when={point()+1} keyed>{index=>
                <label class="flex min-w-0 flex-col gap-1">새 차트 데이터 값
                  <input name="chartValue" type="text" inputmode="decimal" maxlength="64" required
                    value={values()[index-1]} disabled={disabled()} class="min-h-11 w-28 rounded border border-slate-300 px-2"/>
                </label>
              }</Show>
              <button type="submit" class={buttonClass} disabled={disabled()}>차트 데이터 값 적용</button>
            </form>
          }</Show>
          <Show when={props.kind==='chart'&&(run.object?.moveChartUp||run.object?.moveChartDown)}>
            <For each={([
              {type:'move_chart_up',enabled:run.object?.moveChartUp,label:'차트 위로 이동'},
              {type:'move_chart_down',enabled:run.object?.moveChartDown,label:'차트 아래로 이동'},
            ] as const)}>{action=>
              <button type="button" class={buttonClass} disabled={disabled()||!action.enabled} onClick={()=>{
                const accepted=props.onApply({type:action.type,targetId:run.id});
                setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
              }}>{action.label}</button>
            }</For>
          </Show>
          <Show when={props.kind==='chart'&&run.object?.deleteEditable}>
            <button type="button" class={buttonClass} disabled={disabled()} onClick={()=>{
              const accepted=props.onApply({type:'delete_chart',targetId:run.id});
              setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            }}>선택한 차트 제거</button>
          </Show>
          </>
        }</Show>
        </Show>
        <Show when={props.pending}><p class="basis-full text-slate-600">{labels().name} 내용을 저장하거나 실행 취소하세요.</p></Show>
        <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
      </div>
    </details>
  </Show>;
}
