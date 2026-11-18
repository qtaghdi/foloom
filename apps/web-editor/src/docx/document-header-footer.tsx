import { For,Show,createEffect,createMemo,createSignal } from 'solid-js';
import type { SectionInfo,SectionFormat,HeaderFooterInfo } from './contracts';
import { buttonClass,quietButtonClass } from '../ui/editor-chrome';

export function resolveHeaderText(sections:SectionInfo[],slot:HeaderFooterInfo|undefined):string {
 if(!slot)return '';
 if(!slot.linked)return slot.text;
 return sections.find(section=>section.id===slot.sourceSectionId)?.headerFooters?.find(info=>info.kind===slot.kind&&info.variant===slot.variant)?.text??'';
}
export function DocumentHeaderFooter(props:{evenAndOddHeaders?:boolean;onParityChange?:(enabled:boolean)=>boolean;sections:SectionInfo[];disabled:boolean;onApply:(id:string,format:SectionFormat)=>boolean}) {
 const [id,setId]=createSignal(''),[kind,setKind]=createSignal<'header'|'footer'>('header'),[variant,setVariant]=createSignal<'default'|'first'|'even'>('default'),[text,setText]=createSignal(''),[firstPage,setFirstPage]=createSignal(false),[error,setError]=createSignal('');
 const [pageNumber,setPageNumber]=createSignal(false),[totalPages,setTotalPages]=createSignal(false);
 const [compact,setCompact]=createSignal(false);
 const selected=createMemo(()=>props.sections.find(section=>section.id===id())??props.sections[0]);
 const slot=createMemo(()=>selected()?.headerFooters?.find(info=>info.kind===kind()&&info.variant===variant()));
 createEffect(()=>{
  const info=slot();
  setText(resolveHeaderText(props.sections,info));
  setFirstPage(Boolean(selected()?.firstPage));
  setPageNumber(Boolean(info?.pageNumber));
  setTotalPages(Boolean(info?.totalPages));
  setCompact(Boolean(info?.compactPageNumbers));
  setError('');
 });
 const disabled=()=>props.disabled||!selected()?.editable||!slot()?.editable;
 const apply=(linkPrevious=false)=>{
  const section=selected();if(!section)return;
  const changingField=pageNumber()!==Boolean(slot()?.pageNumber)||Boolean(slot()?.linked&&pageNumber());
  const changingTotal=totalPages()!==Boolean(slot()?.totalPages)||Boolean(slot()?.linked&&totalPages());
  const compactPageNumbers=pageNumber()&&totalPages()&&compact();
  const changingLayout=compactPageNumbers!==Boolean(slot()?.compactPageNumbers)||Boolean(slot()?.linked&&compactPageNumbers);
  const independent={
   text:text(),
   ...(changingField?{pageNumber:pageNumber()}:{}),
   ...(changingTotal?{totalPages:totalPages()}:{}),
   ...(changingLayout?{compactPageNumbers}:{}),
  };
  const update={kind:kind(),variant:variant(),...(linkPrevious?{linkPrevious:true}:independent)};
  const format:SectionFormat={headerFooters:[update]};if(firstPage()!==Boolean(section.firstPage))format.firstPage=firstPage();
  setError(props.onApply(section.id,format)?'':'내용 크기와 문서 연결 상태를 확인하세요.');
 };
 return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
  <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">머리글·바닥글</summary>
  <Show when={selected()?.headerFooters?.length}>
  <div role="group" aria-label="머리글 바닥글 작성" class="flex max-h-[40dvh] min-w-0 flex-wrap items-center gap-2 overflow-y-auto overscroll-contain pb-2">
   <select aria-label="머리글 바닥글 구역" class="editor-select min-h-11 max-w-full" value={selected()?.id??''} onChange={e=>setId(e.currentTarget.value)}><For each={props.sections}>{(section,index)=><option value={section.id}>구역 {index()+1}</option>}</For></select>
   <select aria-label="머리글 바닥글 종류" class="editor-select min-h-11" value={kind()} onChange={e=>setKind(e.currentTarget.value as 'header'|'footer')}><option value="header">머리글</option><option value="footer">바닥글</option></select>
   <select aria-label="머리글 바닥글 페이지 구분" class="editor-select min-h-11" value={variant()} onChange={e=>setVariant(e.currentTarget.value as 'default'|'first'|'even')}><option value="default">기본·홀수 페이지</option><option value="first">첫 페이지</option><option value="even">짝수 페이지</option></select>
   <label class="flex min-h-11 cursor-pointer items-center gap-2"><input type="checkbox" aria-label="첫 페이지 머리글 바닥글 구분" disabled={props.disabled||!selected()?.editable} checked={firstPage()} onChange={e=>setFirstPage(e.currentTarget.checked)}/>첫 페이지 구분</label>
   <button type="button" class={quietButtonClass} disabled={props.disabled||!selected()?.editable||firstPage()===Boolean(selected()?.firstPage)} onClick={()=>{const section=selected();if(section)setError(props.onApply(section.id,{firstPage:firstPage()})?'':'문서 연결 상태를 확인하세요.');}}>첫 페이지 구분 적용</button>
   <Show when={props.evenAndOddHeaders!==undefined&&props.onParityChange}>
    <label class="flex min-h-11 cursor-pointer items-center gap-2">
     <input type="checkbox" disabled={props.disabled} checked={props.evenAndOddHeaders}
      onChange={e=>{if(!props.onParityChange?.(e.currentTarget.checked))e.currentTarget.checked=Boolean(props.evenAndOddHeaders);}}/>
     문서 전체 홀수·짝수 페이지 구분
    </label>
   </Show>
   <p class="basis-full text-slate-600">{slot()?.linked?'이전 구역과 연결됨. 내용을 적용하면 이 구역만 독립된 내용으로 바뀝니다.':'이 구역의 독립된 내용입니다.'}</p>
   <Show when={!slot()?.editable}><p role="status" class="basis-full text-amber-800">원본 서식·필드 또는 내용 한도를 보존하기 위해 읽기 전용입니다.</p></Show>
   <textarea aria-label="머리글 바닥글 내용" rows="3" class="w-full min-w-0 cursor-text rounded border border-slate-300 bg-white p-2" value={text()} disabled={disabled()} onInput={e=>setText(e.currentTarget.value)}/>
   <label class="flex min-h-11 cursor-pointer items-center gap-2">
    <input type="checkbox" checked={pageNumber()} disabled={disabled()} onChange={event=>setPageNumber(event.currentTarget.checked)}/>
    쪽 번호 필드
   </label>
   <label class="flex min-h-11 cursor-pointer items-center gap-2">
    <input type="checkbox" checked={totalPages()} disabled={disabled()} onChange={event=>setTotalPages(event.currentTarget.checked)}/>
    전체 쪽 수 필드
   </label>
   <Show when={pageNumber()&&totalPages()}>
    <label class="flex min-h-11 cursor-pointer items-center gap-2">
     <input type="checkbox" checked={compact()} disabled={disabled()} onChange={event=>setCompact(event.currentTarget.checked)}/>
     한 줄에 쪽 번호 / 전체 쪽 수
    </label>
   </Show>
   <button type="button" class={buttonClass} disabled={disabled()} onClick={()=>apply()}>내용 적용</button>
   <button type="button" class={quietButtonClass} disabled={disabled()||props.sections[0]?.id===selected()?.id||Boolean(slot()?.linked)} onClick={()=>apply(true)}>이전 구역 연결</button>
   <p class="basis-full text-slate-600">적용 뒤 저장으로 확정합니다. 빈 내용과 이전 구역 연결은 다릅니다. 홀수·짝수 구분은 문서 전체에 적용됩니다. 실제 페이지 조판은 검증 중입니다.</p>
   <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
  </div>
  </Show>
 </details>;
}
