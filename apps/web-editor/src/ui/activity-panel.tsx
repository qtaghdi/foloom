import { For, Show } from 'solid-js';
import type { ActivityEntry } from './activity';
import { buttonClass } from './editor-chrome';

const states = { pending: '저장 중', committed: '저장됨', failed: '실패', uncertain: '확인 필요' };
const actors = { ai: 'AI', human: '사람', system: '시스템' };
export function ActivityPanel(props: { entries: ActivityEntry[]; onClose: () => void; presence?: string }) {
  return <aside id="document-activity" class="flex h-full min-h-0 w-72 max-w-full shrink-0 flex-col border-l border-slate-200 bg-white max-sm:w-full" aria-label="문서 활동">
    <div class="border-b border-slate-200 p-4">
      <div class="flex items-center justify-between gap-2"><h2 id="document-activity-heading" tabindex={-1} class="text-base font-semibold text-slate-950 focus-visible:outline-2 focus-visible:outline-indigo-600">문서 활동</h2><button type="button" class={buttonClass} aria-label="문서 활동 닫기" onClick={props.onClose}>닫기</button></div>
      <p class="mt-1 text-xs leading-5 text-slate-500">현재 연결에서 확인한 작업</p>
      <Show when={props.presence}><p class="mt-3 rounded bg-indigo-50 px-3 py-2 text-xs break-words text-indigo-700" role="status">{props.presence}</p></Show>
    </div>
    <div class="min-h-0 flex-1 overflow-auto overscroll-contain">
      <Show when={props.entries.length} fallback={<div class="grid min-h-56 content-center px-5 py-12 text-center"><p class="text-sm font-medium text-slate-700">아직 기록된 변경이 없습니다.</p><p class="mt-2 text-xs leading-6 text-slate-500">AI와 사람의 저장 결과가 여기에 표시됩니다.</p></div>}>
        <ol class="divide-y divide-slate-100" aria-label="최근 변경"><For each={props.entries}>{(entry) => <li class="space-y-2 px-4 py-4 text-sm">
          <div class="flex items-center justify-between gap-2 text-xs"><span class="font-semibold text-slate-600">{actors[entry.actor]}</span><span classList={{ 'text-emerald-700': entry.state === 'committed', 'text-amber-700': entry.state !== 'committed' }}>{states[entry.state]}</span></div>
          <p class="font-medium text-slate-800">{entry.title}</p>
          <Show when={entry.location}><p class="break-words text-xs text-slate-500">{entry.location}</p></Show>
          <p class="text-xs text-slate-400"><Show when={entry.revision !== undefined}>리비전 {entry.revision} · </Show><time dateTime={new Date(entry.observedAt ?? 0).toISOString()}>{new Intl.DateTimeFormat('ko-KR', { hour: '2-digit', minute: '2-digit' }).format(entry.observedAt ?? 0)}</time></p>
        </li>}</For></ol>
      </Show>
    </div>
    <p class="border-t border-slate-100 px-4 py-3 text-[11px] leading-5 text-slate-400">시각은 이 화면에서 확인한 시간입니다. 이전 실행의 이력은 포함하지 않습니다.</p>
  </aside>;
}
