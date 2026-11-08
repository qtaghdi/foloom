import { FoloomBrand } from '../brand/brand';
import { LocaleSelect } from '../i18n/locale-select';
import { t, localizeMessage } from '../i18n';
import { Show, type JSX } from 'solid-js';

export const buttonClass = 'editor-action min-h-11 shrink-0 rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-700 disabled:opacity-40';
export const quietButtonClass = 'editor-action min-h-11 shrink-0 rounded-md px-3 text-sm font-medium text-stone-700 disabled:opacity-40';
export const primaryButtonClass = 'editor-action min-h-11 shrink-0 rounded-md border border-teal-700 bg-teal-700 px-4 text-sm font-semibold text-white disabled:opacity-40';

export function EditorHeader(props: {
  name: string; kind: string; message: string; status: string;
  nameId: string; statusId: string; actions?: JSX.Element; onActivity?: () => void; activityOpen?: boolean;
}) {
  return <header class="app-toolbar flex min-h-14 shrink-0 flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-slate-200 bg-white px-4 py-2 text-slate-900 sm:px-6">
    <div class="flex min-w-0 flex-1 items-center gap-3">
      <FoloomBrand symbolOnly/>
      <div class="flex min-w-0 items-center gap-3">
        <h1 id={props.nameId} class="min-w-0 truncate text-sm font-semibold sm:text-base" title={props.name}>{props.name}</h1>
        <span class="sr-only">{props.kind}</span>
      </div>
    </div>
    <div class="flex min-w-0 flex-wrap items-center gap-2 sm:gap-3">
      <p id={props.statusId} class="revision-status min-w-0 text-xs text-slate-600" role="status" aria-live="polite" aria-atomic="true" data-state={props.status}>
        {localizeMessage(props.message)}
      </p>
      <Show when={new URLSearchParams(window.location.search).get('embedded') !== '1'}><LocaleSelect/></Show>
      {props.actions}
      <Show when={props.onActivity}><button type="button" data-activity-toggle class={`${quietButtonClass} inline-flex items-center gap-2`} aria-expanded={props.activityOpen} aria-controls={props.activityOpen ? 'document-activity' : undefined} onClick={() => props.onActivity?.()}><svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path d="M4 12h4l2.5-6 3 12 2.5-6H20" stroke-linecap="round" stroke-linejoin="round"/></svg>{t('activity')}</button></Show>
    </div>
  </header>;
}
