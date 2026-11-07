import { For } from 'solid-js';
import type { ParagraphFormat } from './contracts';
export function DocumentParagraphToolbar(props:{styleDefinition?:boolean;formats:ParagraphFormat[];disabled:boolean;onFormat:(format:ParagraphFormat)=>void}) {
 const common=<K extends keyof ParagraphFormat>(key:K):ParagraphFormat[K]|'mixed'|undefined=>{
  const first=props.formats[0]?.[key];return props.formats.some(f=>f[key]!==first)?'mixed':first;
 };
 const lengths=[{key:'left',name:'왼쪽 들여쓰기',min:-720,max:720},{key:'right',name:'오른쪽 들여쓰기',min:-720,max:720},{key:'before',name:'문단 앞 간격',min:0,max:144},{key:'after',name:'문단 뒤 간격',min:0,max:144}] as const;
 return <div role="group" aria-label="문단 서식" class="flex w-full min-w-0 items-center gap-2 overflow-x-auto border-t border-slate-200 py-1 text-xs">
  <select aria-label="문단 정렬" class="editor-select min-h-11 shrink-0 rounded px-2" disabled={props.disabled} value={common('align')??(props.styleDefinition?'inherit':'start')} onChange={e=>props.onFormat({align:e.currentTarget.value as ParagraphFormat['align']})}>
   <option value="inherit" disabled>상속 정렬</option><option value="mixed" disabled>혼합 정렬</option><option value="start">시작 정렬</option><option value="center">가운데 정렬</option><option value="end">끝 정렬</option><option value="justify">양쪽 정렬</option>
  </select>
  <For each={lengths}>{item=><label class="flex shrink-0 items-center gap-1">{item.name}<input aria-label={item.name} type="number" min={item.min} max={item.max} step="0.5" disabled={props.disabled}
   class="editor-select min-h-11 w-20 rounded px-2" placeholder={common(item.key)==='mixed'?'혼합':props.styleDefinition?'상속':'0'} value={common(item.key)==='mixed'?'':common(item.key)===undefined&&props.styleDefinition?'':Number(common(item.key)??0)/20}
   onChange={e=>{const value=Number(e.currentTarget.value);if(Number.isFinite(value)&&value>=item.min&&value<=item.max)props.onFormat({[item.key]:Math.round(value*20)});}}/><span>pt</span></label>}</For>
  <label class="flex shrink-0 items-center gap-1">첫 줄<input aria-label="첫 줄 들여쓰기" title="음수는 내어쓰기 (pt)" type="number" min="-720" max="720" step="0.5" disabled={props.disabled} class="editor-select min-h-11 w-20 rounded px-2"
   value={common('firstLine')==='mixed'||common('hanging')==='mixed'||props.styleDefinition&&common('firstLine')===undefined&&common('hanging')===undefined?'':Number(common('firstLine')??-Number(common('hanging')??0))/20} placeholder={props.styleDefinition?'상속':'혼합'}
   onChange={e=>{const value=Number(e.currentTarget.value);if(Number.isFinite(value)&&Math.abs(value)<=720)props.onFormat(value<0?{hanging:Math.round(-value*20)}:{firstLine:Math.round(value*20)});}}/><span>pt</span></label>
  <select aria-label="줄 간격 방식" class="editor-select min-h-11 shrink-0 rounded px-2" disabled={props.disabled} value={common('lineRule')??(props.styleDefinition?'inherit':'auto')} onChange={e=>{const rule=e.currentTarget.value as 'auto'|'exact'|'atLeast';props.onFormat({lineRule:rule,line:rule==='auto'?324:240});}}>
   <option value="inherit" disabled>상속 줄 간격</option><option value="mixed" disabled>혼합 줄 간격</option><option value="auto">줄 간격 배수</option><option value="exact">고정 간격 (pt)</option><option value="atLeast">최소 간격 (pt)</option>
  </select>
  <input aria-label="줄 간격" type="number" min={common('lineRule')==='auto'||!common('lineRule')?0.5:1} max={common('lineRule')==='auto'||!common('lineRule')?10:144} step="0.1" disabled={props.disabled} class="editor-select min-h-11 w-20 shrink-0 rounded px-2"
   value={common('lineRule')==='mixed'||common('line')==='mixed'||props.styleDefinition&&common('line')===undefined?'':Number(common('line')??324)/(common('lineRule')==='exact'||common('lineRule')==='atLeast'?20:240)} placeholder={props.styleDefinition?'상속':'혼합'}
   onChange={e=>{const value=Number(e.currentTarget.value),rule=common('lineRule')==='exact'?'exact':common('lineRule')==='atLeast'?'atLeast':'auto';if(rule==='auto'?value>=0.5&&value<=10:value>=1&&value<=144)props.onFormat({line:Math.round(value*(rule==='auto'?240:20)),lineRule:rule});}}/>
  <select aria-label="문단 읽기 방향" class="editor-select min-h-11 shrink-0 rounded px-2" disabled={props.disabled} value={common('bidi')==='mixed'?'mixed':common('bidi')===undefined&&props.styleDefinition?'inherit':common('bidi')?'rtl':'ltr'} onChange={e=>props.onFormat({bidi:e.currentTarget.value==='rtl'})}>
   <option value="inherit" disabled>상속 읽기 방향</option><option value="mixed" disabled>혼합 읽기 방향</option><option value="ltr">왼쪽에서 오른쪽</option><option value="rtl">오른쪽에서 왼쪽</option>
  </select>
  <select aria-label="문단 텍스트 방향" class="editor-select min-h-11 shrink-0 rounded px-2" disabled={props.disabled} value={common('textDirection')??(props.styleDefinition?'inherit':'lrTb')} onChange={e=>props.onFormat({textDirection:e.currentTarget.value})}>
   <option value="inherit" disabled>상속 텍스트 방향</option><option value="mixed" disabled>혼합 텍스트 방향</option><option value="lrTb">가로 쓰기</option><option value="tbRl">세로 쓰기 (오른쪽)</option><option value="btLr">세로 쓰기 (왼쪽)</option>
  </select>
 </div>;
}
