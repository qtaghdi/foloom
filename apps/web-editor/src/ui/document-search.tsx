import { t, localizeMessage } from '../i18n';
import { For, Show, createEffect, createSignal, onCleanup, onMount } from 'solid-js';
import { buttonClass, quietButtonClass } from './editor-chrome';

export interface SearchItem { label: string; text: string; locate: () => void }
export interface SearchResults { items: SearchItem[]; truncated?: boolean }

export function DocumentSearch(props: { version: string; search: (query: string) => Promise<SearchResults>; hint: string }) {
  const [open, setOpen] = createSignal(false);
  const [query, setQuery] = createSignal('');
  const [results, setResults] = createSignal<SearchResults>();
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal('');
  let input!: HTMLInputElement;
  let trigger!: HTMLButtonElement;
  let generation = 0;
  const invalidate = () => { generation++; setResults(undefined); setError(''); setBusy(false); };
  createEffect(() => { props.version; invalidate(); });
  const close = () => { invalidate(); setOpen(false); trigger.focus(); };
  const show = () => { setOpen(true); queueMicrotask(() => { input?.focus(); input?.select(); }); };
  const keydown = (event: KeyboardEvent) => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f') { event.preventDefault(); show(); }
    if (event.key === 'Escape' && open()) { event.preventDefault(); close(); }
  };
  onMount(() => window.addEventListener('keydown', keydown));
  onCleanup(() => { generation++; window.removeEventListener('keydown', keydown); });
  const search = async () => {
    const current = ++generation;
    setResults(undefined); setError(''); setBusy(true);
    try {
      const found = await props.search(query());
      if (current === generation) setResults(found);
    } catch { if (current === generation) setError(t('searchFailedCheckTheDocumentStatusAndTry')); }
    finally { if (current === generation) setBusy(false); }
  };
  return <>
    <button ref={trigger} type="button" class={quietButtonClass} aria-expanded={open()} aria-controls={open() ? 'document-search' : undefined} onClick={() => open() ? close() : show()}>{t('search')}</button>
    <Show when={open()}><section id="document-search" aria-label={t('searchInDocument')} class="fixed right-3 top-16 z-50 w-[min(26rem,calc(100vw-1.5rem))] rounded-lg border border-slate-300 bg-white p-4 text-slate-900 shadow-lg">
      <form onSubmit={(event) => { event.preventDefault(); if (query().trim() && !busy()) void search(); }}>
        <div class="flex items-center gap-2">
          <label class="sr-only" for="document-search-query">{t('searchText')}</label>
          <input ref={input} id="document-search-query" type="search" maxlength={200} value={query()} autocomplete="off" placeholder={t('findInDocument')} class="min-h-11 min-w-0 flex-1 rounded border border-slate-300 px-3 text-sm focus-visible:outline-2 focus-visible:outline-teal-700" onInput={(event) => { setQuery(event.currentTarget.value); invalidate(); }}/>
          <button class={buttonClass} type="submit" disabled={busy() || !query().trim()}>{t('find')}</button>
          <button class={quietButtonClass} type="button" onClick={close}>{t('close')}</button>
        </div>
      </form>
      <p class="mt-2 text-xs leading-5 text-slate-500">{props.hint}</p>
      <p role="status" aria-live="polite" class="mt-2 text-sm">{busy() ? t('searching') : localizeMessage(error()) || (results() ? `${t('resultCount', { count: results()!.items.length })}${results()!.truncated ? ' ' + t('onlySomeResultsAreShownRefineYourSearch') : ''}` : '')}</p>
      <ul class="mt-2 max-h-[min(45dvh,24rem)] space-y-1 overflow-auto">
        <For each={results()?.items}>{(item) => <li><button type="button" class="editor-action w-full rounded border border-slate-200 px-3 py-2 text-left text-sm" onClick={() => { close(); item.locate(); }}><span class="block font-semibold text-teal-800">{localizeMessage(item.label)}</span><span class="block truncate">{item.text}</span></button></li>}</For>
      </ul>
    </section></Show>
  </>;
}
