import { createSignal, Show } from 'solid-js';
import type { StatusPanelState } from './app-view';
import { buttonClass } from './editor-chrome';

export const controlClass = buttonClass;

export function StatusPanel(props: { id: string; panel: StatusPanelState; danger?: boolean; onDismiss: () => void }) {
  const [expanded, setExpanded] = createSignal(Boolean(props.danger));
  return <section id={props.id} role={props.danger ? 'alert' : 'status'} aria-live={props.danger ? 'assertive' : 'polite'} tabindex={-1}
    class={`shrink-0 border-b px-4 py-2 text-sm break-words focus-visible:outline-2 focus-visible:outline-indigo-600 sm:px-6 ${props.danger ? 'border-red-200 bg-red-50 text-red-900' : 'border-indigo-100 bg-indigo-50/70 text-slate-700'}`}>
    <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1">
      <div class="flex min-w-0 items-center gap-3"><strong class="shrink-0 text-xs font-semibold">{props.panel.title}</strong><Show when={!props.danger}><span class="hidden border-l border-indigo-200 pl-3 text-xs text-slate-500 sm:inline">문서 보존과 지원 범위를 확인하세요.</span></Show></div>
      <div class="flex shrink-0 items-center gap-1">
        <button type="button" class="min-h-11 touch-manipulation rounded px-3 text-xs font-semibold text-indigo-700 underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-indigo-600" aria-expanded={expanded()} aria-controls={`${props.id}-details`} onClick={() => setExpanded(!expanded())}>{expanded() ? '접기' : '자세히'}</button>
        <Show when={props.panel.dismissible}><button type="button" class="min-h-11 touch-manipulation rounded px-3 text-xs hover:bg-white focus-visible:outline-2 focus-visible:outline-indigo-600" aria-label={`${props.panel.title} 닫기`} onClick={props.onDismiss}>닫기</button></Show>
      </div>
    </div>
    <div id={`${props.id}-details`} hidden={!expanded()} class="max-h-[32dvh] overflow-auto overscroll-contain pb-2">
      <p class="max-w-5xl py-2 text-xs leading-6 whitespace-pre-wrap">{props.panel.message}</p>
      <Show when={props.panel.reload}><button type="button" class={`${buttonClass} mt-2 bg-white`} onClick={() => props.panel.reload?.()}>저장된 문서 다시 불러오기</button></Show>
    </div>
  </section>;
}
