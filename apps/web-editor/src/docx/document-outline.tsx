import { createMemo, createSignal, For, Show } from 'solid-js';
import { quietButtonClass } from '../ui/editor-chrome';
import type { Paragraph, Snapshot, StyleInfo } from './contracts';

function levelOf(paragraph:Paragraph,styles:Map<string,StyleInfo>):number|undefined {
  if(paragraph.outlineOverride!==undefined)return paragraph.outlineOverride;
  let id=paragraph.styleId;
  if(!id||!styles.has(id))return paragraph.outlineLevel;
  const seen=new Set<string>();
  while(id&&!seen.has(id)&&seen.size<32){
    seen.add(id);
    const style=styles.get(id);
    if(!style)break;
    if(style.outlineLevel!==undefined)return style.outlineLevel;
    id=style.basedOn;
  }
}

export function DocumentOutline(props:{
  snapshot?:Snapshot; styles:StyleInfo[]; activeRun?:string;
  onLocate:(paragraphId:string)=>void;
}) {
  const [open,setOpen]=createSignal(false),[last,setLast]=createSignal('');
  const outline=createMemo(()=>{
    if(!open())return {headings:[],current:"",truncated:false};
    const styles=new Map(props.styles.filter(style=>style.kind==='paragraph').map(style=>[style.id,style]));
    const headings:Array<{id:string;level:number;text:string}>=[];
    let current='';
    for(const block of props.snapshot?.blocks??[]){
      if(block.type!=='paragraph')continue;
      const paragraph=block.paragraph,level=levelOf(paragraph,styles);
      if(level!==undefined&&level<9){
        const text=paragraph.runs.map(run=>run.text).join('').trim();
        if(text)headings.push({id:paragraph.id,level,text:text.slice(0,160)});
      }
      if(props.activeRun&&paragraph.runs.some(run=>run.id===props.activeRun))current=headings.at(-1)?.id??'';
    }
    return {headings:headings.slice(0,200),current,truncated:headings.length>200};
  });
  return <>
    <button class={quietButtonClass} type="button" aria-expanded={open()} aria-controls={open()?'docx-outline':undefined}
      onClick={()=>setOpen(value=>!value)}>문서 개요</button>
    <Show when={open()}><nav id="docx-outline" aria-label="제목 개요"
      class="fixed right-3 top-16 z-50 w-[min(23rem,calc(100vw-1.5rem))] rounded-lg border border-slate-300 bg-white p-3 text-slate-900 shadow-lg"
      onKeyDown={event=>{if(event.key==='Escape'){event.stopPropagation();setOpen(false);}}}>
      <div class="flex items-center justify-between"><strong>제목 개요</strong><button type="button" class={quietButtonClass} onClick={()=>setOpen(false)}>닫기</button></div>
      <Show when={outline().headings.length} fallback={<p class="px-2 py-3 text-sm text-slate-600">개요 수준이 지정된 제목이 없습니다.</p>}>
        <ul class="max-h-[50dvh] overflow-y-auto">
          <For each={outline().headings}>{heading=><li>
            <button type="button" class="editor-action min-h-11 w-full rounded px-2 text-left text-sm"
              style={{'padding-inline-start':`${0.5+heading.level*0.75}rem`}}
              aria-current={(outline().current||last())===heading.id?'location':undefined}
              onClick={()=>{setLast(heading.id);props.onLocate(heading.id);}}>{heading.text}</button>
          </li>}</For>
        </ul>
        <Show when={outline().truncated}><p class="px-2 py-1 text-xs text-slate-600">처음 200개 제목을 표시합니다.</p></Show>
      </Show>
    </nav></Show>
  </>;
}
