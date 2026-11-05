import { utf8Length } from './validation';
import { For } from 'solid-js';
import { quietButtonClass } from '../ui/editor-chrome';
import type { InlineStyle } from './contracts';

export function DocumentFormatToolbar(props: { styleDefinition?:boolean; styles: InlineStyle[]; disabled: boolean; onFormat: (style: InlineStyle) => void }) {
  const common = <K extends keyof InlineStyle>(key: K): InlineStyle[K] | 'mixed' | undefined => {
    const first=props.styles[0]?.[key];
    return props.styles.some(style=>style[key]!==first)?'mixed':first;
  };
  const toggles = [{key:'bold',label:'굵게',text:'B'},{key:'italic',label:'기울임',text:'I'},{key:'underline',label:'밑줄',text:'U'},{key:'strike',label:'취소선',text:'S'}] as const;
  return <div role="group" aria-label="글자 서식" class="flex w-full min-w-0 items-center gap-1 overflow-x-auto border-t border-slate-200 py-1">
    <For each={toggles}>{item=><button type="button" class={`${quietButtonClass} shrink-0`} aria-label={props.styleDefinition && item.key!=='underline'?`상속 ${item.label} 반전`:item.label} title={props.styleDefinition && item.key!=='underline'?`상속 ${item.label} 반전`:item.label}
      aria-pressed={common(item.key)==='mixed'?'mixed':Boolean(common(item.key))} disabled={props.disabled}
      classList={{'bg-teal-100 text-teal-900':common(item.key)===true,'bg-amber-100 text-amber-900':common(item.key)==='mixed'}}
      onClick={()=>props.onFormat({[item.key]:common(item.key)!==true})}>{item.text}{common(item.key)==='mixed'?'·':''}</button>}</For>
    <input aria-label="글꼴" title="설치되지 않은 글꼴은 대체 글꼴로 표시됩니다." class="editor-select min-h-11 w-32 shrink-0 rounded px-2 text-sm"
      placeholder={common('font')==='mixed'?'혼합 글꼴':props.styleDefinition?'상속 글꼴':'기본 글꼴'} value={common('font')==='mixed'?'':common('font')??''} disabled={props.disabled} maxLength={120}
      onChange={event=>{const font=event.currentTarget.value.trim();event.currentTarget.setCustomValidity('');if(font && utf8Length(font)<=120 && !/["\\\u0000-\u001f\u007f]/u.test(font))props.onFormat({font});else if(font){event.currentTarget.setCustomValidity('글꼴 이름을 줄이거나 지원하지 않는 문자를 제거하세요.');event.currentTarget.reportValidity();}}}/>
    <input aria-label="글자 크기" title="글자 크기 (pt)" type="number" min="4" max="144" step="0.5" class="editor-select min-h-11 w-20 shrink-0 rounded px-2 text-sm"
      placeholder={common('sizeHalfPoints')==='mixed'?'혼합':props.styleDefinition?'상속':'11'} value={common('sizeHalfPoints')==='mixed'?'':common('sizeHalfPoints')===undefined&&props.styleDefinition?'':Number(common('sizeHalfPoints')??22)/2} disabled={props.disabled}
      onChange={event=>{const size=Number(event.currentTarget.value)*2;if(Number.isInteger(size)&&size>=8&&size<=288)props.onFormat({sizeHalfPoints:size});}}/>
    <input aria-label="글자 색" title="글자 색 (#RRGGBB)" class="editor-select min-h-11 w-24 shrink-0 rounded px-2 text-sm" maxLength={7}
      placeholder={common('color')==='mixed'?'혼합 색':props.styleDefinition?'상속 색':'#000000'} value={common('color')==='mixed'?'':common('color')??''} disabled={props.disabled}
      onChange={event=>{if(/^#[0-9a-f]{6}$/i.test(event.currentTarget.value))props.onFormat({color:event.currentTarget.value});}}/>
    <select aria-label="강조 색" class="editor-select min-h-11 shrink-0 rounded px-2 text-sm" disabled={props.disabled} value={common('highlight')??(props.styleDefinition?'inherit':'none')}
      onChange={event=>props.onFormat({highlight:event.currentTarget.value})}>
      <option value="inherit" disabled>상속 강조</option><option value="mixed" disabled>혼합 강조</option><option value="none">강조 없음</option><option value="yellow">노랑 강조</option><option value="green">초록 강조</option><option value="cyan">청록 강조</option><option value="magenta">자홍 강조</option>
    </select>
    <select aria-label="위 아래 첨자" class="editor-select min-h-11 shrink-0 rounded px-2 text-sm" disabled={props.disabled} value={common('vertical')??(props.styleDefinition?'inherit':'baseline')}
      onChange={event=>props.onFormat({vertical:event.currentTarget.value})}>
      <option value="inherit" disabled>상속 위치</option><option value="mixed" disabled>혼합 첨자</option><option value="baseline">기본 위치</option><option value="superscript">위 첨자</option><option value="subscript">아래 첨자</option>
    </select>
    <button type="button" class={`${quietButtonClass} shrink-0`} disabled={props.disabled} onClick={()=>props.onFormat({reset:true})}>직접 서식 지우기</button>
  </div>;
}
