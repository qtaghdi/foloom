import { Show, createSignal, onCleanup } from 'solid-js';
import { t, localizeMessage } from '../i18n';
import { quietButtonClass } from '../ui/editor-chrome';
import { chooseDOCXCopyDestination, writeDOCXCopy } from '../transport/save-docx-copy';
import type { DownloadedFile } from '../transport/document-files';

export function SaveDOCXCopyAction(props:{name:string;disabled:boolean;onExport:(name:string)=>Promise<DownloadedFile|undefined>}) {
 const [busy,setBusy]=createSignal(false),[notice,setNotice]=createSignal(''),[error,setError]=createSignal('');
 let disposed=false;
 onCleanup(()=>{disposed=true;});
 const run=async(pickDestination=false)=>{
  if(busy()||props.disabled)return;
  setBusy(true);setNotice('');setError('');
  const base=props.name.replace(/\.docx$/i,'');
  // Long imported names still need a bounded safe suggested download name.
  let stem=base.replace(/[/\\:*?"<>|\p{Cc}]/gu,'_').replace(/^\.+/,'').trim()||'document';
  while(new TextEncoder().encode(stem).length>200)stem=Array.from(stem).slice(0,-1).join('');
  const name=`${stem} (${t('copyFileSuffix')}).docx`;
  try {
   const destination=pickDestination?await chooseDOCXCopyDestination(name):undefined;
   if(disposed)return;
   const file=await props.onExport(name);
   if(file&&!disposed){
    const result=await writeDOCXCopy(file,destination);
    if(!disposed)setNotice(t(result==='saved'?'docxCopyWritten':'downloadStarted'));
   }
  }catch(problem){
   if(!disposed&&!(problem instanceof DOMException&&problem.name==='AbortError'))setError(problem instanceof Error?problem.message:String(problem));
  }finally{if(!disposed)setBusy(false);}
 };
 return <div class="flex min-w-0 flex-wrap items-center gap-1">
  <button type="button" class={quietButtonClass} disabled={props.disabled||busy()} title={t('saveDOCXCopyHint')} onClick={()=>void run()}>{busy()?t('preparingDownload'):t('saveDOCXCopy')}</button>
  <button type="button" class={quietButtonClass} disabled={props.disabled||busy()} onClick={()=>void run(true)}>{t('chooseDOCXCopyLocation')}</button>
  <span role="status" class="sr-only">{notice()}</span>
  <Show when={error()}><p role="alert" class="max-w-64 text-xs text-red-700">{localizeMessage(error())}</p></Show>
 </div>;
}
