import { createSignal, Show } from 'solid-js';
import type { StatusPanelState } from './app-view';

export const controlClass = 'min-h-11 min-w-11 touch-manipulation rounded border border-current px-3 py-2 text-xs font-semibold focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-current disabled:cursor-not-allowed disabled:opacity-40';

export function StatusPanel(props: { id: string; panel: StatusPanelState; danger?: boolean; onDismiss: () => void }) {
  const [expanded, setExpanded] = createSignal(true);
  return <section id={props.id} role={props.danger ? 'alert' : 'status'} aria-live={props.danger ? 'assertive' : 'polite'} tabindex={-1}
    class={`status-panel fixed z-20 max-h-[50dvh] max-w-4xl overflow-auto rounded-lg border px-4 py-3 text-sm break-words shadow-xl focus-visible:outline-2 ${props.danger ? 'border-red-200 bg-red-50 text-red-900' : 'border-amber-300 bg-amber-50 text-amber-950'}`}>
    <div class="flex items-center justify-between gap-3">
      <strong class="min-w-0 text-sm font-bold">{props.panel.title}</strong>
      <div class="flex shrink-0 items-center gap-1">
        <button type="button" class={`${controlClass} bg-white/70 hover:bg-white`} aria-expanded={expanded()} aria-controls={`${props.id}-details`} onClick={() => setExpanded(!expanded())}>{expanded() ? '접기' : '펼치기'}</button>
        <Show when={props.panel.dismissible}><button type="button" class={`${controlClass} bg-white/70 hover:bg-white`} aria-label={`${props.panel.title} 닫기`} onClick={props.onDismiss}>닫기</button></Show>
      </div>
    </div>
    <div id={`${props.id}-details`} hidden={!expanded()} class="status-panel-details">
      <p class="mt-2 leading-6 whitespace-pre-wrap">{props.panel.message}</p>
      <Show when={props.panel.reload}><button type="button" class={`${controlClass} mt-3 bg-white`} onClick={() => props.panel.reload?.()}>저장된 문서 다시 불러오기</button></Show>
    </div>
  </section>;
}
