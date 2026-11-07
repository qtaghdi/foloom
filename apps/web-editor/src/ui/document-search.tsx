import { t, localizeMessage } from '../i18n';
import { For, Show, createEffect, createSignal, onCleanup, onMount } from 'solid-js';
import { buttonClass, quietButtonClass } from './editor-chrome';

export interface SearchOptions { matchCase?:boolean; wholeWord?:boolean; scope?:'all'|'body'|'table' }
export interface SearchItem { label: string; text: string; locate: () => void; replace?: (text:string)=>boolean }
export interface SearchResults { items: SearchItem[]; truncated?: boolean; replaceAll?: (text:string)=>boolean; protectedCount?:number }

export function DocumentSearch(props: { version: string; search: (query: string,options?:SearchOptions) => Promise<SearchResults>; hint: string; docxEditing?:boolean }) {
  const [open, setOpen] = createSignal(false);
  const [query, setQuery] = createSignal('');
  const [results, setResults] = createSignal<SearchResults>();
  const [caseSensitive,setCaseSensitive] = createSignal(false);
  const [wholeWord,setWholeWord] = createSignal(false);
  const [scope,setScope] = createSignal<NonNullable<SearchOptions['scope']>>('all');
  const [active,setActive] = createSignal(0);
  const [replacement,setReplacement] = createSignal('');
  const [notice,setNotice] = createSignal('');
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal('');
  let input!: HTMLInputElement;
  let trigger!: HTMLButtonElement;
  let generation = 0;
  const invalidate = () => { generation++; setResults(undefined); setError(''); setNotice(''); setActive(0); setBusy(false); };
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
    setResults(undefined); setError(''); setNotice(''); setActive(0); setBusy(true);
    try {
      const found = await props.search(query(),props.docxEditing ? {matchCase:caseSensitive(),wholeWord:wholeWord(),scope:scope()}:undefined);
      if (current === generation) setResults(found);
    } catch { if (current === generation) setError(t('searchFailedCheckTheDocumentStatusAndTry')); }
    finally { if (current === generation) setBusy(false); }
  };
  const locate = (index:number) => {
    const items = results()?.items;
    if (!items?.length) return;
    const next = (index+items.length)%items.length;
    setActive(next);
    items[next]!.locate();
  };
  const replace = (all:boolean) => {
    const apply = all ? results()?.replaceAll : results()?.items[active()]?.replace;
    if (!apply) return;
    if (!apply(replacement())) {
      invalidate();
      setError('문서나 검색 결과가 바뀌었습니다. 다시 찾은 뒤 바꾸세요.');
      return;
    }
    invalidate();
    setNotice('변경을 적용했습니다. 저장하거나 실행 취소할 수 있습니다.');
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
      <Show when={props.docxEditing}>
        <div class="mt-2 flex flex-wrap items-center gap-2 text-xs">
          <label class="flex min-h-11 items-center gap-1"><input type="checkbox" checked={caseSensitive()} onChange={event=>{setCaseSensitive(event.currentTarget.checked);invalidate();}}/>대소문자 구분</label>
          <label class="flex min-h-11 items-center gap-1"><input type="checkbox" checked={wholeWord()} onChange={event=>{setWholeWord(event.currentTarget.checked);invalidate();}}/>단어 단위</label>
          <select aria-label="찾기 범위" class="editor-select min-h-11" value={scope()} onChange={event=>{setScope(event.currentTarget.value as NonNullable<SearchOptions['scope']>);invalidate();}}>
            <option value="all">본문과 표</option><option value="body">본문</option><option value="table">표</option>
          </select>
        </div>
        <div class="mt-2 flex flex-wrap gap-2">
          <input type="text" maxlength="200" aria-label="바꿀 텍스트" value={replacement()}
            class="min-h-11 min-w-28 flex-1 rounded border border-slate-300 px-2 text-sm"
            onInput={event=>setReplacement(event.currentTarget.value)}/>
          <button type="button" class={quietButtonClass} disabled={!results()?.items[active()]?.replace} onClick={()=>replace(false)}>한 건 바꾸기</button>
          <button type="button" class={quietButtonClass} disabled={!results()?.replaceAll} onClick={()=>replace(true)}>전체 바꾸기</button>
        </div>
        <Show when={results()?.items.length}>
          <div class="mt-1 flex flex-wrap items-center gap-2 text-xs">
            <button type="button" class={quietButtonClass} onClick={()=>locate(active()-1)}>이전 결과</button>
            <button type="button" class={quietButtonClass} onClick={()=>locate(active()+1)}>다음 결과</button>
            <span>{active()+1} / {results()?.items.length}</span>
            <Show when={results()?.protectedCount}><span>바꿀 수 없는 결과 {results()?.protectedCount}개</span></Show>
          </div>
        </Show>
      </Show>
      <p class="mt-2 text-xs leading-5 text-slate-500">{props.hint}</p>
      <p role="status" aria-live="polite" class="mt-2 text-sm">{busy() ? t('searching') : localizeMessage(error()) || notice() || (results() ? `${t('resultCount', { count: results()!.items.length })}${results()!.truncated ? ' ' + t('onlySomeResultsAreShownRefineYourSearch') : ''}` : '')}</p>
      <ul class="mt-2 max-h-[min(45dvh,24rem)] space-y-1 overflow-auto">
        <For each={results()?.items}>{(item,index) => <li><button type="button" class="editor-action w-full rounded border border-slate-200 px-3 py-2 text-left text-sm" aria-current={props.docxEditing&&index()===active()?'true':undefined} onClick={() => { if(props.docxEditing)locate(index());else{close();item.locate();} }}><span class="block font-semibold text-teal-800">{localizeMessage(item.label)}</span><span class="block truncate">{item.text}</span></button></li>}</For>
      </ul>
    </section></Show>
  </>;
}
