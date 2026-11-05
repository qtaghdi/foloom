import { LocaleSelect } from '../i18n/locale-select';
import { StatusPanel } from '../ui/status-panel';
import { problemMessage } from '../ui/messages';
import { t, localizeMessage, locale } from '../i18n';
import { For, Show, createSignal, onCleanup, onMount } from 'solid-js';
import { z } from 'zod';
import { buttonClass, primaryButtonClass, quietButtonClass } from '../ui/editor-chrome';

const documentSchema = z.object({ id: z.string().regex(/^[a-zA-Z0-9-]+$/), name: z.string(), path: z.string(), kind: z.enum(['xlsx', 'docx']) });
const listSchema = z.object({ documents: z.array(documentSchema).max(8), maxOpen: z.number().int().positive().max(8) });
const stateSchema = z.object({ type: z.enum(['workspace:state', 'workspace:checked']), id: z.string(), message: z.string(), dirty: z.boolean(), busy: z.boolean(), request: z.string().optional() });
type OpenDocument = z.infer<typeof documentSchema>;
type EditorState = z.infer<typeof stateSchema>;
const recentKey = 'document-workspace:recent:v1';
const recentSchema = z.array(z.string().max(4096)).max(10);

async function request(path: string, init?: RequestInit): Promise<unknown> {
  const response = await fetch(path, { ...init, credentials: 'same-origin', cache: 'no-store' });
  if (response.status === 204) return undefined;
  const body: unknown = await response.json();
  if (!response.ok) {
    const error = z.object({ error: z.string() }).safeParse(body);
    throw new Error(error.success ? error.data.error : t('theFileOperationFailed'));
  }
  return body;
}

export function WorkspaceApp() {
  const [documents, setDocuments] = createSignal<OpenDocument[]>([]);
  const [active, setActive] = createSignal('');
  const [visited, setVisited] = createSignal(new Set<string>());
  const [states, setStates] = createSignal<Record<string, EditorState>>({});
  const [paths, setPaths] = createSignal('');
  const [recent, setRecent] = createSignal<string[]>([]);
  const [opening, setOpening] = createSignal(false);
  const [error, setError] = createSignal('');
  const [confirmClose, setConfirmClose] = createSignal<OpenDocument>();
  const [closing, setClosing] = createSignal('');
  const [checking, setChecking] = createSignal('');
  const frames = new Map<string, HTMLIFrameElement>();
  let pending: { id: string; request: string; discard: boolean; timer: ReturnType<typeof setTimeout> } | undefined;
  let sequence = 0;
  let mutationVersion = 0;
  let disposed = false;
  let polling = false;
  const syncVisibility = () => {
    for (const [id, frame] of frames) { frame.contentWindow?.postMessage({ type: 'workspace:visibility', active: active() === id }, window.location.origin); frame.contentWindow?.postMessage({ type: 'workspace:locale', locale: locale() }, window.location.origin); }
  };
  const select = (id: string) => { setVisited(previous => new Set([...previous, id])); setActive(id); queueMicrotask(() => { syncVisibility(); document.getElementById(`tab-${id}`)?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' }); }); };
  const remember = (path: string) => {
    const next = [path, ...recent().filter(item => item !== path)].slice(0, 10);
    setRecent(next);
    try { localStorage.setItem(recentKey, JSON.stringify(next)); } catch { /* File opening remains usable without local storage. */ }
  };
  const refresh = async () => {
    if (polling || opening() || closing()) return;
    const version = mutationVersion;
    polling = true;
    try {
      const result = listSchema.parse(await request('/api/documents'));
      if (disposed || version !== mutationVersion) return;
      // Preserve object identity and mounted editors, including local drafts if
      // another window has closed a server session. Never silently discard them.
      setDocuments(previous => [...previous, ...result.documents.filter(item => !previous.some(existing => existing.id === item.id))]);
      if (!active() && result.documents.length) {
        const initial = new URLSearchParams(window.location.search).get('document');
        select(result.documents.find(item => item.id === initial)?.id ?? result.documents[0]!.id);
      }
    } catch (problem) { if (!disposed) setError(problem instanceof Error ? problem.message : t('openFilesCouldNotBeChecked')); }
    finally { polling = false; }
  };
  const openFiles = async (input = paths()) => {
    if (opening()) return;
    setOpening(true); mutationVersion++; setError('');
    const entries = input.split('\n').map(path => path.trim()).filter(Boolean);
    try {
      for (const path of entries) {
        const opened = documentSchema.parse(await request('/api/documents', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path }) }));
        if (disposed) return;
        setDocuments(previous => previous.some(item => item.id === opened.id) ? previous : [...previous, opened]);
        select(opened.id); remember(opened.path);
      }
      setPaths('');
    } catch (problem) { if (!disposed) setError(problem instanceof Error ? problem.message : t('theFileCouldNotBeOpened')); }
    finally { if (!disposed) setOpening(false); }
  };
  const clearPending = () => { if (pending) clearTimeout(pending.timer); pending = undefined; setChecking(''); };
  const close = async (item: OpenDocument) => {
    if (closing()) return;
    setClosing(item.id); mutationVersion++; setError('');
    try {
      const response = await fetch(`/api/documents/${encodeURIComponent(item.id)}`, { method: 'DELETE', credentials: 'same-origin' });
      if (!response.ok && response.status !== 404) throw new Error(t('theFileCouldNotBeClosedTryAgain'));
      if (disposed) return;
      const remaining = documents().filter(candidate => candidate.id !== item.id);
      setDocuments(remaining); frames.delete(item.id); setConfirmClose(undefined);
      setStates(previous => { const next = { ...previous }; delete next[item.id]; return next; });
      setVisited(previous => { const next = new Set(previous); next.delete(item.id); return next; });
      if (active() === item.id) select(remaining[0]?.id ?? '');
      queueMicrotask(() => document.getElementById(active() ? `tab-${active()}` : 'workspace-paths')?.focus());
    } catch (problem) { if (!disposed) setError(problem instanceof Error ? problem.message : t('theFileCouldNotBeClosed')); }
    finally { if (!disposed) setClosing(''); }
  };
  const checkClose = (item: OpenDocument, save = false, discard = false) => {
    if (pending) return;
    const frame = frames.get(item.id);
    if (!frame) { void close(item); return; }
    const requestID = String(++sequence);
    setChecking(item.id); setError('');
    pending = { id: item.id, request: requestID, discard, timer: setTimeout(() => { clearPending(); setError(t('theEditorStateCouldNotBeCheckedCheck')); }, 15000) };
    frame.contentWindow?.postMessage({ type: 'workspace:check', request: requestID, save }, window.location.origin);
  };
  const message = (event: MessageEvent) => {
    if (event.origin !== window.location.origin || !event.source) return;
    const parsed = stateSchema.safeParse(event.data);
    if (!parsed.success || frames.get(parsed.data.id)?.contentWindow !== event.source) return;
    const value = parsed.data;
    frames.get(value.id)?.contentWindow?.postMessage({ type: 'workspace:visibility', active: active() === value.id }, window.location.origin);
    frames.get(value.id)?.contentWindow?.postMessage({ type: 'workspace:locale', locale: locale() }, window.location.origin);
    setStates(previous => ({ ...previous, [value.id]: value }));
    if (value.type !== 'workspace:checked' || pending?.id !== value.id || pending.request !== value.request) return;
    const discard = pending.discard;
    clearPending();
    const item = documents().find(item => item.id === value.id);
    if (!item) return;
    if (value.busy) { setError(t('theFileIsSavingOrLoadingWaitUntil')); return; }
    if (value.dirty && !discard) { setConfirmClose(item); queueMicrotask(() => document.getElementById('workspace-close')?.focus()); return; }
    void close(item);
  };
  const beforeUnload = (event: BeforeUnloadEvent) => {
    if (Object.values(states()).some(state => state.dirty || state.busy)) { event.preventDefault(); event.returnValue = ''; }
  };
  const cancelClose = () => { const id = confirmClose()?.id; setConfirmClose(undefined); if (id) document.getElementById(`tab-${id}`)?.focus(); };
  const keydown = (event: KeyboardEvent) => { if (event.key === 'Escape' && confirmClose()) { event.preventDefault(); cancelClose(); } };
  const tabKey = (event: KeyboardEvent, index: number) => {
    const length = documents().length;
    const next = event.key === 'ArrowRight' ? (index + 1) % length : event.key === 'ArrowLeft' ? (index + length - 1) % length : event.key === 'Home' ? 0 : event.key === 'End' ? length - 1 : -1;
    if (next < 0) return;
    event.preventDefault(); const id = documents()[next]!.id; select(id); document.getElementById(`tab-${id}`)?.focus();
  };
  onMount(() => {
    try { const saved = recentSchema.safeParse(JSON.parse(localStorage.getItem(recentKey) ?? '[]')); if (saved.success) setRecent(saved.data); } catch { /* Ignore unavailable or malformed local history. */ }
    window.addEventListener('message', message); window.addEventListener('beforeunload', beforeUnload); window.addEventListener('keydown', keydown);
    void refresh();
  });
  const timer = setInterval(() => { void refresh(); }, 4000);
  onCleanup(() => { disposed = true; clearPending(); clearInterval(timer); window.removeEventListener('message', message); window.removeEventListener('beforeunload', beforeUnload); window.removeEventListener('keydown', keydown); });
  return <div class="flex h-dvh min-h-0 flex-col bg-slate-100 text-slate-900">
    <header class="shrink-0 border-b border-slate-200 bg-white px-4 py-3">
      <div class="flex flex-wrap items-center justify-between gap-3"><h1 class="font-semibold">{t('workspace')} <span class="ml-2 text-xs font-normal text-slate-500">{t('openCount', { count: documents().length })}</span></h1><LocaleSelect/><details class="relative"><summary class={`${quietButtonClass} flex items-center`}>{t('recent')}</summary><div class="absolute right-0 top-full z-50 mt-2 max-h-72 w-[min(28rem,calc(100vw-2rem))] overflow-auto rounded-lg border border-slate-200 bg-white p-3 shadow-lg"><Show when={recent().length} fallback={<p class="p-2 text-sm text-slate-500">{t('noRecent')}</p>}><For each={recent()}>{path => <button class="editor-action block w-full truncate rounded p-2 text-left text-xs" title={path} disabled={opening()} onClick={event => { event.currentTarget.closest('details')?.removeAttribute('open'); void openFiles(path); }}>{path}</button>}</For><button class={quietButtonClass} onClick={() => { setRecent([]); try { localStorage.removeItem(recentKey); } catch {} }}>{t('clearHistory')}</button></Show></div></details></div>
      <form class="mt-2 flex flex-wrap items-end gap-2" onSubmit={event => { event.preventDefault(); void openFiles(); }}>
        <label class="min-w-0 flex-1 text-xs text-slate-600" for="workspace-paths">{t('paths')} <span class="text-slate-400">{t('pathsHint')}</span><textarea id="workspace-paths" rows={1} value={paths()} onInput={event => setPaths(event.currentTarget.value)} placeholder="/absolute/path/document.xlsx" class="mt-1 block min-h-11 w-full resize-y rounded-md border border-slate-300 px-3 py-2 text-sm focus-visible:outline-2 focus-visible:outline-teal-700"/></label>
        <button type="submit" class={primaryButtonClass} disabled={opening() || !paths().trim()}>{opening() ? t('opening') : t('openFiles')}</button>
      </form>
    </header>
    <Show when={error()}><StatusPanel id="workspace-error" danger panel={{ title: t('operationFailed'), message: problemMessage(error()), details: error(), dismissible: true }} onDismiss={() => setError('')}/></Show>
    <Show when={confirmClose()}>{item => <section id="workspace-close" tabindex={-1} role="region" aria-label={t('confirmClosingFile')} class="shrink-0 border-b border-amber-200 bg-amber-50 p-4 text-sm"><p>{t('closeWarning', { name: item().name })}</p><div class="mt-2 flex flex-wrap gap-2"><Show when={item().kind === 'docx'}><button class={primaryButtonClass} disabled={Boolean(checking() || closing())} onClick={() => checkClose(item(), true)}>{t('saveAndClose')}</button></Show><button class={buttonClass} disabled={Boolean(checking() || closing())} onClick={() => checkClose(item(), false, true)}>{t('discardInputAndClose')}</button><button class={buttonClass} onClick={() => { select(item().id); cancelClose(); }}>{t('returnToDocument')}</button><button class={quietButtonClass} onClick={cancelClose}>{t('cancel')}</button></div></section>}</Show>
    <div role="tablist" aria-label={t('openDocuments')} class="flex shrink-0 gap-1 overflow-x-auto border-b border-slate-300 bg-slate-50 p-2"><For each={documents()}>{(item, index) => <div class="flex shrink-0 items-center rounded-md border" classList={{ 'border-teal-600 bg-white': active() === item.id, 'border-transparent': active() !== item.id }}>
      <button id={`tab-${item.id}`} role="tab" aria-selected={active() === item.id} aria-controls={`panel-${item.id}`} tabindex={active() === item.id ? 0 : -1} class="editor-action max-w-64 rounded-md px-3 py-2 text-left" title={item.path} onClick={() => select(item.id)} onKeyDown={event => tabKey(event, index())}><span class="block truncate text-sm font-medium">{item.name}</span><span class="block truncate text-xs text-slate-500">{item.kind.toUpperCase()} · {localizeMessage(states()[item.id]?.message ?? t('readyToOpen'))} · {item.path.split('/').at(-2)}</span></button>
      <button aria-label={t('closeNamed', { name: item.name })} class={`${quietButtonClass} px-2`} disabled={Boolean(checking() || closing())} onClick={() => checkClose(item)}>×</button>
    </div>}</For></div>
    <main class="relative min-h-0 flex-1"><Show when={!documents().length}><div class="mx-auto max-w-lg p-8 text-center"><h2 class="text-lg font-semibold">{t('workspaceEmpty')}</h2><p class="mt-3 text-sm leading-7 text-slate-600">{t('enterXlsxAndDocxPathsToSwitchBetween')}</p></div></Show>
      <For each={documents()}>{item => <section id={`panel-${item.id}`} role="tabpanel" aria-labelledby={`tab-${item.id}`} aria-hidden={active() !== item.id} class="absolute inset-0" style={{ visibility: active() === item.id ? 'visible' : 'hidden', 'pointer-events': active() === item.id ? 'auto' : 'none' }}><Show when={visited().has(item.id)}><iframe data-document-editor ref={frame => frames.set(item.id, frame)} onLoad={syncVisibility} title={t('editNamed', { name: item.name })} class="h-full w-full border-0 bg-white" src={`${item.kind === 'docx' ? '/docx' : '/'}?document=${encodeURIComponent(item.id)}&embedded=1`}/></Show></section>}</For>
    </main>
  </div>;
}
