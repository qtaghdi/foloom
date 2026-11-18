import { createSignal, For, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { ObjectTextOperation, Snapshot } from './contracts';
import { utf8Length } from './validation';

const objectLabels={
  control:{title:'콘텐츠 컨트롤 편집',select:'편집할 콘텐츠 컨트롤',name:'컨트롤',text:'콘텐츠 컨트롤 텍스트'},
  textbox:{title:'텍스트 상자 편집',select:'편집할 텍스트 상자',name:'텍스트 상자',text:'텍스트 상자 내용'},
};

export function DocumentObjectTextToolbar(props:{
  snapshot?:Snapshot;
  kind:'control'|'textbox';
  pending:boolean;
  disabled:boolean;
  onApply:(operation:ObjectTextOperation)=>boolean;
}) {
  const [id,setID]=createSignal(''),[error,setError]=createSignal('');
  const labels=()=>objectLabels[props.kind];
  const objects=()=>props.snapshot?.blocks.flatMap(block=>block.type==='paragraph'
    ? block.paragraph.runs.filter(run=>run.object?.kind===props.kind&&run.object.editable)
    : []).slice(0,200)??[];
  const selected=()=>objects().find(run=>run.id===id());
  const disabled=()=>props.pending||props.disabled;
  return <Show when={objects().length}>
    <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
      <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">{labels().title}</summary>
      <div class="flex min-w-0 flex-wrap items-end gap-2 pb-2">
        <select aria-label={labels().select} class="editor-select min-h-11 max-w-56"
          value={selected()?.id??''} disabled={disabled()} onChange={event=>{setID(event.currentTarget.value);setError('');}}>
          <option value="">{labels().name} 선택</option>
          <For each={objects()}>{(run,index)=>
            <option value={run.id}>{index()+1}. {run.object?.label||run.object?.text?.slice(0,60)||`빈 ${labels().name}`}</option>
          }</For>
        </select>
        <Show when={selected()} keyed>{run=>
          <form class="flex flex-wrap items-end gap-2" onSubmit={event=>{
            event.preventDefault();
            if (disabled()) return;
            const input=event.currentTarget.elements.namedItem('objectText') as HTMLInputElement;
            const text=input.value,expectedText=run.object?.text??'';
            if(text===expectedText)return;
            if(utf8Length(text)>1024){setError('텍스트를 UTF-8 1 KiB 이하로 줄이세요.');return;}
            const accepted=props.onApply({type:props.kind==='control'?'update_control_text':'update_textbox_text',targetId:run.id,expectedText,text});
            setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
          }}>
            <label class="flex min-w-0 flex-col gap-1">{labels().text}
              <input name="objectText" type="text" maxlength="1024" value={run.object?.text??''} disabled={disabled()}
                class="min-h-11 rounded border border-slate-300 px-2"/>
            </label>
            <button type="submit" class={buttonClass} disabled={disabled()}>{labels().name} 내용 적용</button>
          </form>
        }</Show>
        <Show when={props.pending}><p class="basis-full text-slate-600">{labels().name} 내용을 저장하거나 실행 취소하세요.</p></Show>
        <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
      </div>
    </details>
  </Show>;
}
