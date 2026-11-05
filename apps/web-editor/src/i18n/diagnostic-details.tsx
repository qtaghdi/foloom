import { Show } from 'solid-js';
import { t } from './index';
export function DiagnosticDetails(props: { value?: string }) {
  return <Show when={props.value}><details class="mt-2 max-h-40 overflow-auto rounded border border-slate-200 bg-white/70 p-2 text-xs text-slate-600"><summary class="editor-action cursor-pointer py-1 font-medium">{t('diagnostics')}</summary><pre class="mt-2 whitespace-pre-wrap break-words font-mono">{props.value}</pre></details></Show>;
}
