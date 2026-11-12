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
  pageCount:number; onLocatePage:(page:number)=>void;
}) {
  const [open,setOpen]=createSignal(false);
  const [last,setLast]=createSignal<{snapshot:Snapshot;paragraphId:string}>();
  let toggle!: HTMLButtonElement;
  const close = () => {
    setOpen(false);
    toggle.focus();
  };
  const locate = (paragraphId: string) => {
    const snapshot = props.snapshot;
    if (!snapshot) return;
    setLast({snapshot,paragraphId});
    props.onLocate(paragraphId);
  };
  const outline=createMemo(()=>{
    if(!open())return {headings:[],currentHeading:"",currentParagraph:"",truncated:false};
    const styles=new Map(props.styles.filter(style=>style.kind==='paragraph').map(style=>[style.id,style]));
    const headings:Array<{id:string;level:number;text:string}>=[];
    let currentHeading='', currentParagraph='';
    for(const block of props.snapshot?.blocks??[]){
      if(block.type!=='paragraph')continue;
      const paragraph=block.paragraph,level=levelOf(paragraph,styles);
      if(level!==undefined&&level<9){
        const text=paragraph.runs.map(run=>run.text).join('').trim();
        if(text)headings.push({id:paragraph.id,level,text:text.slice(0,160)});
      }
      if(props.activeRun&&paragraph.runs.some(run=>run.id===props.activeRun)){
        currentParagraph=paragraph.id;
        currentHeading=headings.at(-1)?.id??'';
      }
    }
    return {headings:headings.slice(0,200),currentHeading,currentParagraph,truncated:headings.length>200};
  });
  const lastParagraph = () => {
    const location = last();
    return location?.snapshot === props.snapshot ? location?.paragraphId : undefined;
  };
  return <>
    <button ref={toggle} class={quietButtonClass} type="button" aria-expanded={open()} aria-controls={open()?'docx-outline':undefined}
      onClick={()=>setOpen(value=>!value)}>문서 개요</button>
    <Show when={open()}><nav id="docx-outline" aria-label="문서 개요"
      class="fixed right-3 top-16 z-50 w-[min(23rem,calc(100vw-1.5rem))] rounded-lg border border-slate-300 bg-white p-3 text-slate-900 shadow-lg"
      onKeyDown={event=>{if(event.key==='Escape'){event.preventDefault();event.stopPropagation();close();}}}>
      <div class="flex items-center justify-between"><strong>문서 개요</strong><button type="button" class={quietButtonClass} onClick={close}>닫기</button></div>
      <div class="max-h-[50dvh] overflow-y-auto">
      <Show when={props.snapshot?.id} keyed>
        <form class="flex flex-wrap items-end gap-2 px-2 py-2" onSubmit={event=>{
          event.preventDefault();
          const input = event.currentTarget.elements.namedItem('page') as HTMLInputElement;
          const number = input.valueAsNumber;
          if (Number.isInteger(number) && number >= 1 && number <= props.pageCount) props.onLocatePage(number);
        }}>
          <label class="flex flex-col gap-1 text-sm">미리보기 페이지 (1–{props.pageCount})
            <input name="page" type="number" min={1} max={props.pageCount} step={1} required
              class="min-h-11 w-24 rounded border border-slate-300 px-2 focus-visible:outline-2 focus-visible:outline-teal-700"/>
          </label>
          <button type="submit" class={quietButtonClass}>페이지 이동</button>
        </form>
      </Show>
      <h2 class="px-2 pt-2 text-sm font-medium">제목</h2>
      <Show when={outline().headings.length} fallback={<p class="px-2 py-3 text-sm text-slate-600">개요 수준이 지정된 제목이 없습니다.</p>}>
        <ul aria-label="제목">
          <For each={outline().headings}>{heading=><li>
            <button type="button" class="editor-action min-h-11 w-full rounded px-2 text-left text-sm"
              style={{'padding-inline-start':`${0.5+heading.level*0.75}rem`}}
              aria-current={(outline().currentHeading||lastParagraph())===heading.id?'location':undefined}
              onClick={()=>locate(heading.id)}>{heading.text}</button>
          </li>}</For>
        </ul>
        <Show when={outline().truncated}><p class="px-2 py-1 text-xs text-slate-600">처음 200개 제목을 표시합니다.</p></Show>
      </Show>
      <h2 class="px-2 pt-2 text-sm font-medium">북마크</h2>
      <Show when={props.snapshot?.bookmarks?.length} fallback={<p class="px-2 py-3 text-sm text-slate-600">저장된 본문 북마크가 없습니다.</p>}>
        <ul aria-label="북마크">
          <For each={props.snapshot?.bookmarks}>{bookmark=><li>
            <button type="button" class="editor-action min-h-11 w-full break-all rounded px-2 text-left text-sm"
              aria-current={(outline().currentParagraph||lastParagraph())===bookmark.paragraphId?'location':undefined}
              onClick={()=>locate(bookmark.paragraphId)}>{bookmark.name}</button>
          </li>}</For>
        </ul>
      </Show>
      </div>
    </nav></Show>
  </>;
}
