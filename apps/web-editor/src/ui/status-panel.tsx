import { problemMessage } from './messages';
import { DiagnosticDetails } from '../i18n/diagnostic-details';
import { t, localizeMessage } from '../i18n';
import { createSignal, Show } from 'solid-js';
import type { StatusPanelState } from './app-view';
import { buttonClass } from './editor-chrome';

export const controlClass = buttonClass;

export function StatusPanel(props: { id: string; panel: StatusPanelState; danger?: boolean; initiallyExpanded?: boolean; onDismiss: () => void }) {
  const [expanded, setExpanded] = createSignal(Boolean(props.danger || props.initiallyExpanded));
  return <section id={props.id} role={props.danger ? 'alert' : 'status'} aria-live={props.danger ? 'assertive' : 'polite'} tabindex={-1}
    class={`shrink-0 border-b px-4 py-2 text-sm break-words focus-visible:outline-2 focus-visible:outline-teal-700 sm:px-6 ${props.danger ? 'border-red-200 bg-red-50 text-red-900' : 'border-slate-200 bg-slate-50 text-slate-700'}`}>
    <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1">
      <div class="flex min-w-0 items-center gap-3"><strong class="shrink-0 text-xs font-semibold">{localizeMessage(props.panel.title)}</strong><Show when={!props.danger}><span class="hidden border-l border-slate-200 pl-3 text-xs text-slate-500 sm:inline">{t('reviewDocumentPreservationAndSupportedFeatures')}</span></Show></div>
      <div class="flex shrink-0 items-center gap-1">
        <button type="button" class="editor-action min-h-11 rounded px-3 text-xs font-semibold text-teal-800" aria-expanded={expanded()} aria-controls={`${props.id}-details`} onClick={() => setExpanded(!expanded())}>{expanded() ? t('collapse') : t('details')}</button>
        <Show when={props.panel.dismissible}><button type="button" class="editor-action min-h-11 rounded px-3 text-xs" aria-label={t('closeNamed', { name: localizeMessage(props.panel.title) })} onClick={props.onDismiss}>{t('close')}</button></Show>
      </div>
    </div>
    <div id={`${props.id}-details`} hidden={!expanded()} class="max-h-[32dvh] overflow-auto overscroll-contain pb-2">
      <p class="max-w-5xl py-2 text-xs leading-6 whitespace-pre-wrap">{localizeMessage(props.panel.message === '' ? '' : props.panel.details && props.panel.message === props.panel.details ? problemMessage(props.panel.message) : props.panel.message)}</p>
      <DiagnosticDetails value={props.panel.details}/>
      <Show when={props.panel.reload}><button type="button" class={`${buttonClass} mt-2 bg-white`} onClick={() => props.panel.reload?.()}>{t('reloadSavedDocument')}</button></Show>
    </div>
  </section>;
}
