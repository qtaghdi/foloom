import { Show } from 'solid-js';
import { AppView } from './app-view';
import { StatusPanel, controlClass } from './status-panel';

export function WorkbookShell(props: { view: AppView }) {
  const state = props.view.state;
  return <>
    <a class="skip-link" href="#spreadsheet">본문으로 이동</a>
    <div id="app-shell" class="grid h-full grid-rows-[auto_minmax(0,1fr)] overflow-hidden">
      <header class="app-toolbar z-10 flex min-h-14 flex-wrap items-center justify-between gap-2 border-b border-emerald-900 bg-emerald-800 text-emerald-50">
        <div class="flex min-w-0 items-center gap-3">
          <span class="grid size-8 shrink-0 place-items-center rounded border border-emerald-500 font-semibold" aria-hidden="true">W</span>
          <div class="grid min-w-0"><strong id="workbook-name" class="truncate text-sm">{state().name}</strong>
            <span id="revision-status" class="revision-status text-[11px] leading-5 text-emerald-100" role="status" aria-live="polite" aria-atomic="true" data-state={state().status}>{state().message}{state().revision > 0 ? ` · 리비전 ${state().revision}` : ''}</span>
          </div>
        </div>
        <div class="toolbar-actions flex min-w-0 items-center gap-2">
          <div class="flex gap-1" role="toolbar" aria-label="문서 변경 이력">
            <button id="history-undo" type="button" class={`${controlClass} border-emerald-600 bg-emerald-700 text-white hover:bg-emerald-600`} disabled={!state().canUndo} onClick={() => props.view.history('undo')}>실행 취소</button>
            <button id="history-redo" type="button" class={`${controlClass} border-emerald-600 bg-emerald-700 text-white hover:bg-emerald-600`} disabled={!state().canRedo} onClick={() => props.view.history('redo')}>다시 실행</button>
          </div>
          <Show when={state().presence}><div id="ai-presence" class="ai-presence flex max-w-[42vw] items-center gap-2 rounded-full border border-violet-300 bg-violet-50 px-3 py-2 text-xs text-violet-950" role="status" aria-live="polite"><span class="size-2 shrink-0 rounded-full bg-violet-600" aria-hidden="true"/><span id="ai-presence-text" class="truncate">{state().presence}</span></div></Show>
        </div>
      </header>
      {/* Solid owns this container's lifecycle; Univer exclusively owns its children. */}
      <main id="spreadsheet" ref={(element) => { props.view.spreadsheet = element; }} class="min-h-0 min-w-0" aria-label="Workbook editor" tabindex={-1} aria-busy={state().status === 'pending'}/>
      <Show when={state().warning} keyed>{(panel) => <StatusPanel id="feature-warnings" panel={panel} onDismiss={() => props.view.dismiss('warning')}/>}</Show>
      <Show when={state().error} keyed>{(panel) => <StatusPanel id="fatal-error" panel={panel} danger onDismiss={() => props.view.dismiss('error')}/>}</Show>
    </div>
  </>;
}
