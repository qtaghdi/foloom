import { For, Show, createMemo, createSignal } from 'solid-js';
import type { ListSetup, Paragraph, ParagraphList, Snapshot } from './contracts';
import { buttonClass } from '../ui/editor-chrome';
import { maxListStart } from './lists';

const numberFormats={decimal:'1, 2, 3',upperRoman:'I, II, III',lowerRoman:'i, ii, iii',upperLetter:'A, B, C',lowerLetter:'a, b, c'} as const;
type NumberFormat=keyof typeof numberFormats;

export function DocumentListToolbar(props:{
  paragraphs:Paragraph[]; lists:NonNullable<Snapshot['lists']>; disabled:boolean;
  onList:(list:ParagraphList|ListSetup)=>void;
  onLevel:(delta:number)=>void;
}) {
 const [start,setStart]=createSignal(1);
 const [format,setFormat]=createSignal<NumberFormat>('decimal');
 const [restartStart,setRestartStart]=createSignal(1);
 const maxStart=()=>maxListStart(format());
 const restartSource=createMemo(()=>{
  const selected=props.paragraphs[0]?.list;
  const id=selected?.id,level=selected?.level??0;
  if (!id || props.paragraphs.some(p=>p.list?.id!==id||(p.list.level??0)!==level)) return;
  const levels=props.lists.find(list=>list.id===id)?.levels??[];
  const source=levels.find(item=>item.level===level);
  const limit=maxListStart(source?.kind),token=`%${level+1}`;
  if (!limit || !source?.text.includes(token)) return;
  if (level>0 && (/%[1-9]/.test(source.text.replaceAll(token,'')) ||
    levels.some(child=>child.level>level&&child.text.includes(token)))) return;
  return {id,level,limit};
 });
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
  <Show when={restartSource()}>{source=><div role="group" aria-label="기존 목록 번호 다시 시작" class="flex shrink-0 items-center gap-2 border-l border-slate-200 pl-2">
    <label class="flex items-center gap-1">다시 시작할 번호
      <input aria-label="다시 시작할 목록 번호" class="editor-select min-h-11 w-20 rounded px-2"
       type="number" min="1" max={source().limit} value={Number.isFinite(restartStart())?restartStart():''}
       disabled={!available()} onInput={event=>setRestartStart(event.currentTarget.valueAsNumber)}/>
    </label>
    <button type="button" class={buttonClass}
     disabled={!available()||!Number.isInteger(restartStart())||restartStart()<1||restartStart()>source().limit}
     onClick={()=>props.onList({sourceId:source().id,level:source().level,start:restartStart()})}>선택한 목록 번호 다시 시작</button>
  </div>}</Show>
  <button type="button" class={buttonClass} disabled={!canLevel(-1)} onClick={()=>props.onLevel(-1)}>목록 수준 올리기</button>
  <button type="button" class={buttonClass} disabled={!canLevel(1)} onClick={()=>props.onLevel(1)}>목록 수준 내리기</button>
  <button type="button" class={buttonClass} disabled={!available()} onClick={()=>props.onList({id:0,level:0})}>목록 해제</button>
 </div>;
}
