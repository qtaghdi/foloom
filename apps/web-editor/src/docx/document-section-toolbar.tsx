import { For,Show,createEffect,createMemo,createSignal } from 'solid-js';
import { maxHeaderFooterDistance,type SectionFormat,type SectionInfo } from './contracts';
import { buttonClass } from '../ui/editor-chrome';
import { defaultHeaderFooterDistance,previewSection,validateSection } from './sections';

const numberFormats={decimal:'1, 2, 3',upperRoman:'I, II, III',lowerRoman:'i, ii, iii',upperLetter:'A, B, C',lowerLetter:'a, b, c'} as const;
type NumberFormat=keyof typeof numberFormats;
function knownNumberFormat(format:string|undefined):format is NumberFormat {
 return !!format&&Object.hasOwn(numberFormats,format);
}
export function DocumentSectionToolbar(props:{sections:SectionInfo[];disabled:boolean;onApply:(id:string,format:SectionFormat)=>boolean}) {
 const [id,setId]=createSignal(''),[values,setValues]=createSignal<SectionFormat>({}),[error,setError]=createSignal('');
 const selected=createMemo(()=>props.sections.find(section=>section.id===id())??props.sections[0]);
 createEffect(()=>{
  const section=selected();
  if(!section)return;
  setValues({
   ...section.page,
   headerDistance:section.headerDistance??defaultHeaderFooterDistance,
   footerDistance:section.footerDistance??defaultHeaderFooterDistance,
   pageNumberStart:section.pageNumberStart??-1,
   pageNumberFormat:knownNumberFormat(section.pageNumberFormat)?section.pageNumberFormat:section.pageNumberFormat?undefined:'decimal',
   orientation:section.orientation,columns:section.columns,columnSpace:section.columnSpace,
   breakType:['continuous','nextPage','nextColumn','evenPage','oddPage'].includes(section.breakType)?section.breakType as SectionFormat['breakType']:'nextPage',
  });
  setError('');
 });
 const update=(patch:SectionFormat)=>setValues(previous=>({...previous,...patch}));
 const disabled=()=>props.disabled||!selected()?.editable;
 const field=(key:'width'|'height'|'top'|'right'|'bottom'|'left'|'columnSpace'|'headerDistance'|'footerDistance',label:string)=>{
  const distance=key==='headerDistance'||key==='footerDistance';
  const max=distance?maxHeaderFooterDistance/20:key==='columnSpace'?720:2500;
  return <label class="flex min-h-11 items-center gap-1">{label}
   <input type="number" aria-label={label} class="w-20 min-h-11 rounded border border-slate-300 px-2"
    min={key==='width'||key==='height'?100:0} max={max} step="0.05" disabled={disabled()}
    value={values()[key]===undefined?'':values()[key]!/20}
    onChange={e=>update({[key]:Math.round(e.currentTarget.valueAsNumber*20)})}/>pt
  </label>;
 };
 const apply=()=>{
  const section=selected();
  if(!section)return;
  const form=values();
  const { headerDistance,footerDistance,pageNumberStart }=form;
  if([headerDistance,footerDistance].some(value=>value===undefined||!Number.isInteger(value)||value<0||value>maxHeaderFooterDistance)){
   setError(`머리글과 바닥글 거리를 0~${maxHeaderFooterDistance/20}pt 범위로 입력하세요.`);
   return;
  }
  const message=validateSection(previewSection(section,form));
  if(message){setError(message);return;}
  if(pageNumberStart===undefined||!Number.isInteger(pageNumberStart)||pageNumberStart< -1||pageNumberStart>2147483647){
   setError('시작 번호를 0에서 2147483647 사이 정수로 입력하세요.');
   return;
  }
  const original={
   ...section.page,
   headerDistance:section.headerDistance??defaultHeaderFooterDistance,
   footerDistance:section.footerDistance??defaultHeaderFooterDistance,
   pageNumberStart:section.pageNumberStart??-1,pageNumberFormat:section.pageNumberFormat||'decimal',
   orientation:section.orientation,columns:section.columns,columnSpace:section.columnSpace,breakType:section.breakType,
  };
  const patch=Object.fromEntries(Object.entries(form).filter(([key,value])=>value!==undefined&&value!==original[key as keyof typeof original])) as SectionFormat;
  if(!Object.keys(patch).length){setError('변경한 설정이 없습니다.');return;}
  setError(props.onApply(section.id,patch)?'':'설정 범위와 문서 연결 상태를 확인하세요.');
 };
 return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
  <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">구역·용지 설정</summary>
  <div role="group" aria-label="구역 용지 설정" class="flex max-h-[35dvh] min-w-0 flex-wrap items-center gap-2 overflow-y-auto overscroll-contain pb-2">
   <select aria-label="편집할 구역" class="editor-select min-h-11 max-w-full" value={selected()?.id??''} onChange={e=>setId(e.currentTarget.value)}><For each={props.sections}>{(section,index)=><option value={section.id}>구역 {index()+1}{section.editable?'':' (보호됨)'}</option>}</For></select>
   <select aria-label="용지 크기" class="editor-select min-h-11" disabled={disabled()} value="custom" onChange={e=>{const [width,height]=e.currentTarget.value==='letter'?[12240,15840]:[11906,16838];update({width:values().orientation==='landscape'?height:width,height:values().orientation==='landscape'?width:height});}}><option value="custom" disabled>사용자 지정 크기</option><option value="a4">A4</option><option value="letter">Letter</option></select>
   <select aria-label="용지 방향" class="editor-select min-h-11" disabled={disabled()} value={values().orientation} onChange={e=>{const orientation=e.currentTarget.value as 'portrait'|'landscape',width=values().width!,height=values().height!;update({orientation,width:orientation==='landscape'?Math.max(width,height):Math.min(width,height),height:orientation==='landscape'?Math.min(width,height):Math.max(width,height)});}}><option value="portrait">세로 용지</option><option value="landscape">가로 용지</option></select>
   {field('width','용지 폭')}{field('height','용지 높이')}{field('top','위 여백')}{field('right','오른쪽 여백')}{field('bottom','아래 여백')}{field('left','왼쪽 여백')}{field('headerDistance','위쪽 모서리에서 머리글까지')}{field('footerDistance','아래쪽 모서리에서 바닥글까지')}
   <label class="flex min-h-11 items-center gap-1">단 수<input type="number" aria-label="단 수" min="1" max="12" class="w-16 min-h-11 rounded border border-slate-300 px-2" value={values().columns} disabled={disabled()} onChange={e=>update({columns:e.currentTarget.valueAsNumber})}/></label>{field('columnSpace','단 간격')}
   <select aria-label="구역 나누기 유형" class="editor-select min-h-11" disabled={disabled()} value={values().breakType} onChange={e=>update({breakType:e.currentTarget.value as SectionFormat['breakType']})}><option value="nextPage">다음 페이지</option><option value="continuous">연속 구역</option><option value="evenPage">짝수 페이지</option><option value="oddPage">홀수 페이지</option><option value="nextColumn">다음 단</option></select>
   <label class="flex min-h-11 items-center gap-1">쪽 번호<select aria-label="쪽 번호 시작 방식" class="editor-select min-h-11" disabled={disabled()} value={values().pageNumberStart===-1?'continue':'restart'} onChange={e=>update({pageNumberStart:e.currentTarget.value==='continue'?-1:1})}><option value="continue">이전 구역에서 계속</option><option value="restart">지정 번호부터 시작</option></select></label>
   <Show when={values().pageNumberStart!==-1}><label class="flex min-h-11 items-center gap-1">시작 번호<input aria-label="쪽 번호 시작값" type="number" min="0" max="2147483647" step="1" class="w-24 min-h-11 rounded border border-slate-300 px-2" disabled={disabled()} value={Number.isFinite(values().pageNumberStart)?values().pageNumberStart:''} onChange={e=>update({pageNumberStart:e.currentTarget.valueAsNumber===-1?NaN:e.currentTarget.valueAsNumber})}/></label></Show>
   <select aria-label="쪽 번호 형식" class="editor-select min-h-11" disabled={disabled()} value={values().pageNumberFormat??'original'} onChange={e=>update({pageNumberFormat:e.currentTarget.value as NumberFormat})}>
    <option value="original" disabled>원본 번호 형식</option>
    <For each={Object.entries(numberFormats)}>{([format,label])=><option value={format}>{label}</option>}</For>
   </select>
   <button type="button" class={buttonClass} disabled={disabled()} onClick={apply}>선택 구역 적용</button>
   <p class="basis-full text-slate-600">구역 미리보기는 경계를 분리해 표시합니다. 쪽 번호 설정과 머리글·바닥글 위치는 문서에 저장됩니다. 실제 페이지 조판은 문서 편집기에서 확인하세요.</p>
   <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
  </div>
 </details>;
}
