import { Show, type JSX } from 'solid-js';

export const buttonClass = 'min-h-11 shrink-0 touch-manipulation rounded-md border border-slate-300 bg-white px-3 text-sm font-medium text-slate-700 hover:border-slate-400 hover:bg-slate-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 disabled:cursor-not-allowed disabled:opacity-40';
export const primaryButtonClass = 'min-h-11 shrink-0 touch-manipulation rounded-md border border-indigo-600 bg-indigo-600 px-4 text-sm font-semibold text-white hover:bg-indigo-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 disabled:cursor-not-allowed disabled:opacity-40';

export function EditorHeader(props: {
  name: string; kind: string; message: string; status: string; revision?: number;
  nameId: string; statusId: string; actions?: JSX.Element; onActivity?: () => void; activityOpen?: boolean;
}) {
  return <header class="app-toolbar flex min-h-16 shrink-0 flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-slate-200 bg-white px-4 py-2 text-slate-900 sm:px-6">
    <div class="flex min-w-0 flex-1 items-center gap-3 sm:gap-5">
      <div class="flex shrink-0 items-center gap-2 text-indigo-700" aria-label="document-workspace">
        <svg viewBox="0 0 28 32" width="24" height="28" fill="none" aria-hidden="true"><path d="M3 1h14l8 8v22H3zM17 1v9h8" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round"/><text x="8" y="24" fill="currentColor" font-size="15" font-weight="600">D</text></svg>
        <span class="hidden text-base font-semibold tracking-tight text-slate-950 lg:inline" translate="no">document-workspace</span>
      </div>
      <div class="flex min-w-0 items-center gap-3 border-l border-slate-200 pl-3 sm:pl-5">
        <h1 id={props.nameId} class="min-w-0 truncate text-sm font-semibold sm:text-base" title={props.name}>{props.name}</h1>
        <span class="hidden shrink-0 rounded bg-slate-100 px-2 py-1 text-xs text-slate-600 sm:inline">{props.kind}</span>
      </div>
    </div>
    <div class="flex min-w-0 flex-wrap items-center gap-2 sm:gap-3">
      <p id={props.statusId} class="revision-status min-w-0 text-xs text-slate-600" role="status" aria-live="polite" aria-atomic="true" data-state={props.status}>
        {props.message}{props.revision && props.revision > 0 ? ` · 리비전 ${props.revision}` : ''}
      </p>
      {props.actions}
      <Show when={props.onActivity}><button type="button" data-activity-toggle class={buttonClass} aria-expanded={props.activityOpen} aria-controls="document-activity" onClick={() => props.onActivity?.()}>활동</button></Show>
    </div>
  </header>;
}
