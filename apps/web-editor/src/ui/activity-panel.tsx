import { t, localizeMessage, locale } from '../i18n';
import { For, Show } from 'solid-js';
import type { ActivityEntry } from './activity';
import { quietButtonClass } from './editor-chrome';

const states = { pending: t('saving'), committed: t('saved'), failed: t('failed'), uncertain: t('needsReview') };
const actors = { ai: 'AI', human: t('person'), system: t('system') };
const time = () => new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit' });

function ActivityIcon() {
  return <svg aria-hidden="true" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
    <path d="M4 12h4l2.5-6 3 12 2.5-6H20" stroke-linecap="round" stroke-linejoin="round"/>
  </svg>;
}

export function ActivityPanel(props: {
  entries: ActivityEntry[]; onClose: () => void; presence?: string;
  canLocate?: (entry: ActivityEntry) => boolean; onLocate?: (entry: ActivityEntry) => void;
}) {
  return <aside id="document-activity"
    class="editor-activity flex h-full min-h-0 w-[min(320px,38vw)] min-w-64 shrink-0 flex-col border-l border-slate-200 bg-stone-50 max-sm:w-full max-sm:min-w-0"
    aria-label={t('documentActivity')}
    onKeyDown={(event) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      event.stopPropagation();
      props.onClose();
    }}>
    <div class="border-b border-slate-200 px-4 py-3">
      <div class="flex items-center gap-2">
        <span class="grid size-8 shrink-0 place-items-center rounded-md bg-teal-100 text-teal-800"><ActivityIcon/></span>
        <h2 id="document-activity-heading" tabindex={-1} class="min-w-0 flex-1 rounded-sm text-sm font-semibold text-slate-950 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-700">{t('documentActivity')}</h2>
        <button type="button" class={`${quietButtonClass} grid min-w-11 place-items-center`} aria-label={t('closeDocumentActivity')} title={t('close')}
          onClick={props.onClose}>
          <svg aria-hidden="true" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
            <path d="m6 6 12 12M18 6 6 18" stroke-linecap="round"/>
          </svg>
        </button>
      </div>
      <p class="mt-1 text-xs leading-5 text-slate-600">{t('changesObservedInThisView')}</p>
      <Show when={props.presence}><p class="mt-3 break-words rounded-md bg-teal-100/60 px-3 py-2 text-xs text-teal-900" role="status">{props.presence}</p></Show>
    </div>
    <div class="min-h-0 flex-1 overflow-auto overscroll-contain px-4 py-4">
      <Show when={props.entries.length} fallback={
        <div class="rounded-lg border border-slate-200 bg-white p-4">
          <p class="text-sm font-semibold text-slate-800">{t('noChangesYet')}</p>
          <p class="mt-1 text-xs leading-5 text-slate-600">{t('changesSavedByYouAndAiAppearHere')}</p>
        </div>
      }>
        <ol class="ml-1 border-l border-slate-200" aria-label={t('recentChanges')}>
          <For each={props.entries}>{(entry) => <li class="relative pb-3 pl-4 last:pb-0">
            <span aria-hidden="true" class="absolute -left-[5px] top-[18px] size-[9px] rounded-full border-2 border-stone-50"
              classList={{ 'bg-teal-700': entry.actor === 'ai', 'bg-slate-500': entry.actor !== 'ai' }}/>
            <div class="rounded-lg border border-slate-200 bg-white px-3 py-2.5">
              <div class="flex flex-wrap items-center justify-between gap-1.5 text-xs">
                <span class="font-semibold text-slate-700">{localizeMessage(actors[entry.actor])}</span>
                <span class="rounded-full px-2 py-0.5"
                  classList={{ 'bg-teal-50 text-teal-800': entry.state === 'committed', 'bg-amber-50 text-amber-800': entry.state !== 'committed' }}>{localizeMessage(states[entry.state])}</span>
              </div>
              <p class="mt-2 break-words text-sm font-medium leading-5 text-slate-900">{localizeMessage(entry.title)}</p>
              <Show when={localizeMessage(entry.location ?? '')}><p class="mt-1 break-words text-xs text-slate-600">{localizeMessage(entry.location ?? '')}</p></Show>
              <Show when={props.canLocate?.(entry)}>
                <button type="button" class="editor-action mt-1 min-h-11 rounded-md px-2 text-sm font-medium text-teal-800"
                  aria-label={t('goToChangedLocation')} onClick={() => props.onLocate?.(entry)}>{t('goToChangedLocation')} <span aria-hidden="true">↗</span></button>
              </Show>
              <p class="mt-2 text-xs text-slate-600">
                <Show when={entry.revision !== undefined}>{t('revision', { revision: entry.revision! })} · </Show>
                <time dateTime={new Date(entry.observedAt ?? 0).toISOString()}>{time().format(entry.observedAt ?? 0)}</time>
              </p>
            </div>
          </li>}</For>
        </ol>
      </Show>
    </div>
  </aside>;
}
