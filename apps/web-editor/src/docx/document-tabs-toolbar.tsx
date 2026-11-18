import { For, createEffect, createSignal } from 'solid-js';
import type { ParagraphFormat, TabStop } from './contracts';
import { quietButtonClass } from '../ui/editor-chrome';

export function DocumentTabsToolbar(props:{
 formats:ParagraphFormat[];
 disabled:boolean;
 defaultTabStop?:number;
 defaultDisabled:boolean;
 onDefault:(twips:number)=>void;
 onFormat:(format:ParagraphFormat)=>void;
 onInsert:()=>void;
}) {
 const currentInterval=()=>props.defaultTabStop??720;
 const [interval,setInterval]=createSignal(currentInterval()/20);
 createEffect(()=>setInterval(currentInterval()/20));
 const intervalTwips=()=>Math.round(interval()*20);
 const canApplyInterval=()=>!props.defaultDisabled&&intervalTwips()>=1&&intervalTwips()<=32767&&intervalTwips()!==currentInterval();
 const [position,setPosition]=createSignal(72),[align,setAlign]=createSignal<TabStop['align']>('left'),[leader,setLeader]=createSignal<TabStop['leader']>('none');
 const stops=()=>props.formats[0]?.tabs??[];
 const update=(clear=false)=>{
  const twips=Math.round(position()*20);
  if(!Number.isFinite(twips)||twips<0||twips>28800)return;
  props.onFormat({tabs:[{position:twips,align:clear?'clear':align(),leader:clear?'none':leader()}]});
 };
 return <details class="min-w-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
  <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">탭 위치·정렬</summary>
  <div role="group" aria-label="문서 탭" class="flex flex-wrap items-center gap-2 pb-2">
   <label>기본 탭 간격 (pt) <input type="number" min="0.05" max="1638.35" step="0.05"
    class="w-24 min-h-11 rounded border border-slate-300 px-2" value={interval()} disabled={props.defaultDisabled}
    onInput={e=>setInterval(e.currentTarget.valueAsNumber)}/></label>
   <button type="button" class={quietButtonClass} disabled={!canApplyInterval()} onClick={()=>props.onDefault(intervalTwips())}>기본 간격 적용</button>
  </div>
  <div role="group" aria-label="문단 탭" class="flex min-w-0 flex-wrap items-center gap-2 pb-2">
   <label>위치 (pt) <input type="number" aria-label="탭 위치" min="0" max="1440" step="0.5" class="w-20 min-h-11 rounded border border-slate-300 px-2" value={position()} disabled={props.disabled} onChange={e=>setPosition(e.currentTarget.valueAsNumber)}/></label>
   <select aria-label="탭 정렬" class="editor-select min-h-11" value={align()} disabled={props.disabled} onChange={e=>setAlign(e.currentTarget.value as TabStop['align'])}>
    <option value="left">왼쪽</option><option value="center">가운데</option><option value="right">오른쪽</option><option value="decimal">소수점</option><option value="bar">세로선</option>
   </select>
   <select aria-label="탭 채움선" class="editor-select min-h-11" value={leader()} disabled={props.disabled||align()==='bar'} onChange={e=>setLeader(e.currentTarget.value as TabStop['leader'])}>
    <option value="none">채움선 없음</option><option value="dot">점</option><option value="hyphen">대시</option><option value="underscore">밑줄</option><option value="heavy">굵은 선</option><option value="middleDot">가운뎃점</option>
   </select>
   <button type="button" class={quietButtonClass} disabled={props.disabled} onClick={()=>update()}>탭 위치 적용</button>
   <button type="button" class={quietButtonClass} disabled={props.disabled} onClick={()=>update(true)}>이 위치 해제</button>
   <button type="button" class={quietButtonClass} disabled={props.disabled} onClick={props.onInsert}>탭 문자 삽입</button>
   <p class="basis-full text-slate-600">첫 선택 문단: {stops().length?'':`기본 ${currentInterval()/20}pt 간격`}<For each={stops()}>{stop=><button class={quietButtonClass} type="button" onClick={()=>{setPosition(stop.position/20);setAlign(stop.align);setLeader(stop.leader??'none');}}>{stop.position/20}pt · {({left:'왼쪽',center:'가운데',right:'오른쪽',decimal:'소수점',bar:'세로선',clear:'해제'})[stop.align]}</button>}</For></p>
  </div>
 </details>;
}
