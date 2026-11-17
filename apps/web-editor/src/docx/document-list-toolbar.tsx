import { For, createSignal } from 'solid-js';
import type { ListDefinition, Paragraph, ParagraphList, Snapshot } from './contracts';
import { buttonClass } from '../ui/editor-chrome';

const numberFormats={decimal:'1, 2, 3',upperRoman:'I, II, III',lowerRoman:'i, ii, iii'} as const;
type NumberFormat=keyof typeof numberFormats;

export function DocumentListToolbar(props:{
  paragraphs:Paragraph[]; lists:NonNullable<Snapshot['lists']>; disabled:boolean;
  onList:(list:ParagraphList|Omit<ListDefinition,'key'>)=>void;
  onLevel:(delta:number)=>void;
}) {
 const [start,setStart]=createSignal(1);
 const [format,setFormat]=createSignal<NumberFormat>('decimal');
 const maxStart=()=>format()==='decimal'?9999:3999;
 const available=()=>!props.disabled&&props.paragraphs.length>0;
 const canLevel=(delta:number)=>available()&&props.paragraphs.every(p=>p.list&&(p.list.id||p.list.key)&&(p.list.level??0)+delta>=0&&(p.list.level??0)+delta<=8);
 return <div role="group" aria-label="목록 서식" class="flex shrink-0 items-center gap-2 overflow-x-auto border-b border-slate-200 bg-white px-4 py-1 text-xs">
  <button type="button" class={buttonClass} disabled={!available()} onClick={()=>props.onList({kind:'bullet'})}>새 글머리 목록</button>
  <button type="button" class={buttonClass} disabled={!available()} onClick={()=>props.onList({kind:format(),start:start()})}>새 번호 목록</button>
  <label class="flex shrink-0 items-center gap-1">번호 형식
   <select aria-label="목록 번호 형식" class="editor-select min-h-11 rounded px-2" disabled={!available()} value={format()}
    onChange={event=>{setFormat(event.currentTarget.value as NumberFormat);setStart(Math.min(start(),maxStart()));}}>
    <For each={Object.entries(numberFormats)}>{([kind,label])=><option value={kind}>{label}</option>}</For>
   </select>
  </label>
  <label class="flex shrink-0 items-center gap-1">시작 번호
   <input aria-label="목록 시작 번호" class="editor-select min-h-11 w-20 rounded px-2"
    type="number" min="1" max={maxStart()} value={start()} onChange={event=>{
     const n=event.currentTarget.valueAsNumber;
     if(Number.isInteger(n)&&n>=1&&n<=maxStart())setStart(n);
    }}/>
  </label>
  <select aria-label="기존 목록 이어쓰기" class="editor-select min-h-11 shrink-0 rounded px-2" disabled={!available()||!props.lists.length} value="" onChange={e=>{const id=Number(e.currentTarget.value);if(id>0)props.onList({id,level:0});e.currentTarget.value='';}}>
    <option value="">기존 목록 이어쓰기</option><For each={props.lists}>{list=><option value={list.id}>목록 {list.id} ({list.levels[0]?.kind??'기존 서식'})</option>}</For>
  </select>
  <button type="button" class={buttonClass} disabled={!canLevel(-1)} onClick={()=>props.onLevel(-1)}>목록 수준 올리기</button>
  <button type="button" class={buttonClass} disabled={!canLevel(1)} onClick={()=>props.onLevel(1)}>목록 수준 내리기</button>
  <button type="button" class={buttonClass} disabled={!available()} onClick={()=>props.onList({id:0,level:0})}>목록 해제</button>
 </div>;
}
