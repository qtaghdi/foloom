import { createSignal, For, Show } from 'solid-js';
import { quietButtonClass, buttonClass } from '../ui/editor-chrome';
import { DocumentFormatToolbar } from './document-format-toolbar';
import { DocumentParagraphToolbar } from './document-paragraph-toolbar';
import { utf8Length } from './validation';
import type { StyleDefinition, StyleInfo } from './contracts';

export function DocumentStyleToolbar(props:{styles:StyleInfo[];paragraphIds:string[];characterIds:string[];disabled:boolean;definitionDisabled:boolean;pending:boolean;
 onApply:(kind:'paragraph'|'character',id:string)=>void;onDefine:(definition:StyleDefinition)=>void}) {
 const common=(ids:string[])=>ids.some(id=>id!==ids[0])?'mixed':ids[0]??'';
 const [open,setOpen]=createSignal(false),[selectedId,setSelectedId]=createSignal('');
 const [newName,setNewName]=createSignal('새 스타일'),[newKind,setNewKind]=createSignal<'paragraph'|'character'>('paragraph');
 const selected=()=>props.styles.find(style=>style.id===selectedId());
 let dialog:HTMLDialogElement|undefined,opener!:HTMLButtonElement;
 const close=()=>{dialog?.close();setOpen(false);opener.focus({preventScroll:true});};
 const define=(patch:Partial<StyleDefinition>)=>{const style=selected();if(style?.editable)props.onDefine({id:style.id,kind:style.kind,...patch});};
 const show=()=>{if(!selected())setSelectedId(props.styles[0]?.id??'');setOpen(true);queueMicrotask(()=>dialog?.showModal());};
 const validName=(name:string)=>Boolean(name.trim())&&utf8Length(name)<=120&&!/[\u0000-\u001f\u007f]/u.test(name);
 return <>
  <div role="group" aria-label="문서 스타일" class="flex w-full min-w-0 items-center gap-2 overflow-x-auto border-t border-slate-200 py-1 text-xs">
   <For each={['paragraph','character'] as const}>{kind=><select aria-label={kind==='paragraph'?'문단 스타일 적용':'글자 스타일 적용'} class="editor-select min-h-11 max-w-48 shrink-0 rounded px-2" disabled={props.disabled}
    value={common(kind==='paragraph'?props.paragraphIds:props.characterIds)} onChange={e=>props.onApply(kind,e.currentTarget.value)}>
    <option value="mixed" disabled>혼합 스타일</option><option value="">{kind==='paragraph'?'기본 문단 스타일':'글자 스타일 없음'}</option>
    <For each={props.styles.filter(style=>style.kind===kind)}>{style=><option value={style.id} selected={common(kind==='paragraph'?props.paragraphIds:props.characterIds)===style.id}>{style.name} ({style.id})</option>}</For>
   </select>}</For>
   <button ref={opener} type="button" class={quietButtonClass} onClick={show}>스타일 정의 편집</button>
   <Show when={props.pending}><span role="status" class="shrink-0 text-amber-800">스타일 미리 보기는 저장 후 갱신됩니다.</span></Show>
  </div>
  <Show when={open()}>
   <dialog ref={dialog} aria-label="스타일 정의 편집" class="fixed inset-0 m-auto max-h-[85vh] w-[min(56rem,90vw)] overflow-y-auto rounded-xl border border-slate-200 bg-white p-4 text-sm text-slate-900 shadow-xl backdrop:bg-slate-900/40"
    onCancel={e=>{e.preventDefault();close();}}>
    <div class="mb-3 flex items-center justify-between gap-3"><h2 class="font-semibold">스타일 정의 편집</h2><button class={quietButtonClass} type="button" onClick={close}>닫기</button></div>
    <p class="mb-3 text-xs leading-5 text-slate-600">정의 변경은 초안입니다. 문서를 저장하면 같은 스타일을 쓰는 본문과 표가 갱신됩니다. 직접 서식은 스타일보다 우선합니다. 도구는 이 정의에 지정한 값을 표시하며 비어 있는 값은 기준 스타일을 따릅니다.</p>
    <div class="flex flex-wrap items-center gap-2 border-b border-slate-200 pb-3">
     <input aria-label="새 스타일 이름" value={newName()} maxLength={120} class="editor-select min-h-11 min-w-0 rounded border border-slate-200 px-2" onInput={e=>setNewName(e.currentTarget.value)}/>
     <select aria-label="새 스타일 종류" class="editor-select min-h-11 rounded px-2" value={newKind()} onChange={e=>setNewKind(e.currentTarget.value==='character'?'character':'paragraph')}><option value="paragraph">문단 스타일</option><option value="character">글자 스타일</option></select>
     <button type="button" class={buttonClass} disabled={props.definitionDisabled||!validName(newName())||props.styles.length>=500} onClick={()=>{const id=`dw${crypto.randomUUID().replaceAll('-','')}`;props.onDefine({id,name:newName().trim(),kind:newKind()});queueMicrotask(()=>setSelectedId(id));}}>스타일 추가</button>
    </div>
    <label class="my-3 flex items-center gap-2">편집할 스타일<select aria-label="편집할 스타일" value={selectedId()} class="editor-select min-h-11 min-w-0 flex-1 rounded px-2" onChange={e=>setSelectedId(e.currentTarget.value)}><For each={props.styles}>{style=><option value={style.id} selected={selectedId()===style.id}>{style.name} ({style.kind==='paragraph'?'문단':'글자'})</option>}</For></select></label>
    <Show when={selected()}>{style=><div class="space-y-3">
     <label class="flex flex-wrap items-center gap-2">스타일 이름<input aria-label="스타일 이름" class="editor-select min-h-11 min-w-0 flex-1 rounded border border-slate-200 px-2" value={style().name} maxLength={120} disabled={props.definitionDisabled||!style().editable}
      onChange={e=>{e.currentTarget.setCustomValidity('');if(validName(e.currentTarget.value))define({name:e.currentTarget.value.trim()});else{e.currentTarget.setCustomValidity('스타일 이름을 줄이거나 지원하지 않는 문자를 제거하세요.');e.currentTarget.reportValidity();}}}/></label>
     <label class="flex flex-wrap items-center gap-2">기준 스타일<select aria-label="기준 스타일" value={style().basedOn??''} disabled={props.definitionDisabled||!style().editable} class="editor-select min-h-11 min-w-0 flex-1 rounded px-2" onChange={e=>define({basedOn:e.currentTarget.value})}>
      <option value="">상속 없음</option><For each={props.styles.filter(s=>s.kind===style().kind&&s.id!==style().id)}>{base=><option value={base.id} selected={style().basedOn===base.id}>{base.name} ({base.id})</option>}</For>
     </select></label>
     <Show when={!style().editable}><p role="status">이 정의에는 보호된 원본 구조가 있어 수정하지 않습니다. 새 스타일을 추가할 수 있습니다.</p></Show>
     <p class="text-xs leading-5 text-slate-600">스타일의 굵게·기울임·취소선 버튼은 상속한 상태를 반전하는 Word 규칙을 설정합니다. 해제하면 해당 반전을 적용하지 않습니다.</p>
     <DocumentFormatToolbar styleDefinition styles={[style().run??{}]} disabled={props.definitionDisabled||!style().editable} onFormat={run=>define({run})}/>
     <Show when={style().kind==='paragraph'}><DocumentParagraphToolbar styleDefinition formats={[style().paragraph??{}]} disabled={props.definitionDisabled||!style().editable} onFormat={paragraph=>define({paragraph})}/></Show>
    </div>}</Show>
    <p class="mt-3 text-xs text-slate-600">변경을 마친 뒤 닫고 문서를 저장하세요. 잘못된 상속과 순환 참조는 저장하지 않습니다. 목록은 최대 500개입니다.</p>
   </dialog>
  </Show>
 </>;
}
