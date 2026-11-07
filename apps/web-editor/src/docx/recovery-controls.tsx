import { Show, createSignal } from 'solid-js';
import { locale, t } from '../i18n';
import { buttonClass, quietButtonClass } from '../ui/editor-chrome';
import type { RecoveryStatus } from './recovery';

export function RecoveryControls(props: {
  status: RecoveryStatus;
  disabled: boolean;
  onEnable: () => void;
  onDelete: () => void;
  onRetry: () => void;
}) {
  const [confirmDelete, setConfirmDelete] = createSignal(false);
  return <details class="shrink-0 border-b border-slate-200 bg-white px-4 py-1 text-xs">
    <summary class="editor-action w-fit cursor-pointer rounded py-1 focus-visible:outline-2">
      {t('docxRecovery')}{props.status.error ? ` · ${t('docxRecoveryFailed')}` : props.status.entry ? ` · ${t('docxRecoveryEnabled')}` : ''}
    </summary>
    <p class="max-w-2xl py-1 leading-5">{t('docxRecoveryConsent')}</p>
    <Show when={props.status.entry} keyed fallback={<button type="button" class={buttonClass} disabled={props.disabled || props.status.busy} onClick={props.onEnable}>{t('docxRecoveryEnable')}</button>}>
      {entry => <>
        <p aria-live="polite" class="py-1">{t('docxRecoveryLast', { time: new Intl.DateTimeFormat(locale(), { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(entry.updatedAt)) })}</p>
        <Show when={confirmDelete()} fallback={<button type="button" class={quietButtonClass} disabled={props.status.busy} onClick={() => setConfirmDelete(true)}>{t('docxRecoveryDelete')}</button>}>
          <span>{t('docxRecoveryConfirmDelete')}</span>
          <button type="button" class={buttonClass} disabled={props.status.busy} onClick={() => { setConfirmDelete(false); props.onDelete(); }}>{t('docxRecoveryDelete')}</button>
          <button type="button" class={quietButtonClass} onClick={() => setConfirmDelete(false)}>{t('cancel')}</button>
        </Show>
      </>}
    </Show>
    <Show when={props.status.error}>
      <p role="status" class="text-red-700">{t('docxRecoveryFailedHint')}</p>
      <Show when={props.status.entry}><button type="button" class={buttonClass} disabled={props.disabled} onClick={props.onRetry}>{t('docxRecoveryRetry')}</button></Show>
    </Show>
  </details>;
}
