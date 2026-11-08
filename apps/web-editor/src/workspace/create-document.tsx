import { createSignal, Show } from 'solid-js';
import { t, localizeMessage } from '../i18n';
import { buttonClass, primaryButtonClass, quietButtonClass } from '../ui/editor-chrome';
import { docxName, uniqueDOCXName } from './document-names';

export function CreateDocument(props:{names:string[];copy?:boolean;onOpen:()=>void;busy:boolean;disabled:boolean;error:string;onCreate:(name:string)=>Promise<boolean>}) {
 const [open,setOpen]=createSignal(false),[name,setName]=createSignal('');
 let dialog:HTMLDialogElement|undefined,opener!:HTMLButtonElement;
 const title=()=>t(props.copy?'copySavedDOCX':'newDOCX');
 const normalized=()=>docxName(name());
 const duplicate=()=>props.names.some(existing=>existing.normalize('NFC').toLowerCase()===normalized()?.toLowerCase());
 const message=()=>!normalized()?t('documentNameInvalid'):duplicate()?t('documentNameTaken'):'';
 const close=()=>{if(props.busy)return;dialog?.close();setOpen(false);opener.focus({preventScroll:true});};
 const show=()=>{props.onOpen();setName(uniqueDOCXName(t(props.copy?'untitledDOCXCopy':'untitledDOCX'),props.names));setOpen(true);queueMicrotask(()=>dialog?.showModal());};
 const create=async()=>{const value=normalized();if(!value||message()||props.busy)return;if(await props.onCreate(value)){dialog?.close();setOpen(false);opener.focus({preventScroll:true});}};
 return <>
  <button ref={opener} id={props.copy?'workspace-copy-docx':'workspace-new-docx'} type="button" class={buttonClass} disabled={props.disabled} onClick={show}>{title()}</button>
  <Show when={open()}><dialog ref={dialog} aria-label={title()} class="fixed inset-0 m-auto max-h-[85vh] w-[min(28rem,90vw)] overflow-y-auto rounded-xl border border-slate-200 bg-white p-5 text-slate-900 shadow-xl backdrop:bg-slate-900/40" onCancel={e=>{e.preventDefault();close();}}>
   <h2 class="font-semibold">{title()}</h2>
   <p class="mt-2 text-sm leading-6 text-slate-600">{t(props.copy?'copySavedDOCXHint':'newDOCXHint')}</p>
   <form class="mt-4 space-y-3" onSubmit={e=>{e.preventDefault();void create();}}>
    <label class="block text-sm">{t('documentName')}<input aria-label={t('documentName')} aria-invalid={Boolean(message())} aria-describedby="new-docx-name-help" value={name()} maxLength={240} disabled={props.busy}
     class="mt-1 block min-h-11 w-full rounded-md border border-slate-300 px-3 py-2 outline-teal-700 focus-visible:outline-2" onInput={e=>setName(e.currentTarget.value)}/></label>
    <p id="new-docx-name-help" class="break-all text-xs leading-5" classList={{'text-red-700':Boolean(message()),'text-slate-600':!message()}}>{message()||`${t('createdFileName')}: ${normalized()}`}</p>
    <Show when={props.error}><p role="alert" class="text-sm text-red-700">{localizeMessage(props.error)}</p></Show>
    <div class="flex flex-wrap justify-end gap-2"><button class={quietButtonClass} type="button" disabled={props.busy} onClick={close}>{t('cancel')}</button><button class={primaryButtonClass} type="submit" disabled={props.busy||Boolean(message())}>{props.busy?t('opening'):t(props.copy?'createDOCXCopy':'createDocument')}</button></div>
   </form>
  </dialog></Show>
 </>;
}
