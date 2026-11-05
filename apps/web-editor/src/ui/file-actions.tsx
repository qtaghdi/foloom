import { Show, createSignal, onCleanup } from 'solid-js';
import { t, localizeMessage } from '../i18n';
import { DiagnosticDetails } from '../i18n/diagnostic-details';
import { problemMessage } from './messages';
import { buttonClass, quietButtonClass } from './editor-chrome';
import { startBrowserDownload, type DownloadedFile } from '../transport/document-files';

export function FileActions(props: { saveID: string; disabled: boolean; downloadAvailable: boolean; onSave: () => Promise<boolean>; onDownload: () => Promise<DownloadedFile | undefined> }) {
  const [busy, setBusy] = createSignal<'save' | 'download'>();
  const [notice, setNotice] = createSignal('');
  const [error, setError] = createSignal('');
  let disposed = false;
  onCleanup(() => { disposed = true; });
  const run = async (action: 'save' | 'download') => {
    if (busy() || props.disabled) return;
    setBusy(action); setNotice(''); setError('');
    try {
      if (action === 'save') { if (await props.onSave() && !disposed) setNotice(t('savedToFile')); }
      else {
        const file = await props.onDownload();
        if (file && !disposed) { startBrowserDownload(file); setNotice(t('downloadStarted')); }
      }
    } catch (problem) { if (!disposed) setError(problem instanceof Error ? problem.message : String(problem)); }
    finally { if (!disposed) setBusy(undefined); }
  };
  return <div class="flex min-w-0 flex-wrap items-center gap-1" role="group" aria-label={t('fileActions')}>
    <button id={props.saveID} type="button" class={`${buttonClass} inline-flex items-center gap-2`} disabled={props.disabled || Boolean(busy())} title={t('saveCurrentFile')} aria-keyshortcuts="Control+S Meta+S" onClick={() => { void run('save'); }}>
      <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><path d="M5 3h12l4 4v14H3V3h2Zm2 0v6h10V3M7 21v-8h10v8" stroke-linejoin="round"/></svg>
      {busy() === 'save' ? t('checkingSave') : t('saveFile')}
    </button>
    <button type="button" class={`${quietButtonClass} inline-flex items-center gap-2`} disabled={props.disabled || Boolean(busy()) || !props.downloadAvailable} title={props.downloadAvailable ? t('downloadSavedFile') : t('browserDownloadOnly')} onClick={() => { void run('download'); }}>
      <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><path d="M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5" stroke-linecap="round" stroke-linejoin="round"/></svg>
      {busy() === 'download' ? t('preparingDownload') : t('downloadFile')}
    </button>
    <Show when={!props.downloadAvailable}><span class="max-w-60 text-xs text-slate-500">{t('browserDownloadOnly')}</span></Show>
    <span class="sr-only" role="status" aria-live="polite">{localizeMessage(notice())}</span>
    <Show when={error()}><div role="alert" class="w-full rounded border border-red-200 bg-red-50 p-2 text-xs text-red-800"><p>{t('fileActionFailed')} {localizeMessage(problemMessage(error()))}</p><DiagnosticDetails value={error()}/></div></Show>
  </div>;
}
