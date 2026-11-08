import { FileActions } from '../ui/file-actions';
import { warningMessage } from '../ui/messages';
import { DiagnosticDetails } from '../i18n/diagnostic-details';
import { t, localizeMessage } from '../i18n';
import { connectWorkspaceBridge, publishEditorState } from '../ui/workspace-bridge';
import { DocumentSearch } from '../ui/document-search';
import { readTextSelection, restoreTextSelection } from './selection';
import { searchDocument } from './search';
import { Show, createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { DocumentController } from './controller';
import { createDocumentView } from './view';
import { StatusPanel } from '../ui/status-panel';
import type { ActivityEntry } from '../ui/activity';
import { ActivityPanel } from '../ui/activity-panel';
import { DocumentContent } from './document-content';
import { DocumentToolbar } from './document-toolbar';
import type { PasteMode } from './clipboard';
import type { Run } from './contracts';
import { EditorHeader, buttonClass, primaryButtonClass } from '../ui/editor-chrome';



const defaultPaper = { width: 11906, height: 16838, top: 1440, right: 1440, bottom: 1440, left: 1440 };

export function DocumentApp() {
  const { state, view, activity } = createDocumentView();
  const [draftVersion, setDraftVersion] = createSignal(0);
  const [pasteMode, setPasteMode] = createSignal<PasteMode>('text');
  const [zoom, setZoom] = createSignal(100);
  const [focusedRun, setFocusedRun] = createSignal<Run>();
  const paper = () => state().snapshot?.page ?? defaultPaper;
  const paperStyle = () => {
    const { width, height, top, right, bottom, left } = paper();
    const inset = (twips: number) => `${100 * twips / width}%`;
    return {
      zoom: String(zoom() / 100),
      '--paper-width': `${width / 15}px`,
      '--paper-ratio': String(height / width),
      '--paper-insets': [top, right, bottom, left].map(inset).join(' '),
    };
  };
  const [structuralDraft, setStructuralDraft] = createSignal(false);
  const [selectedChange, setSelectedChange] = createSignal<ActivityEntry>();
  const canLocate = (entry: ActivityEntry) => Boolean(!structuralDraft() && entry.documentChange && entry.documentChange.session === state().snapshot?.id && entry.revision === state().snapshot?.revision);
  const changes = createMemo(() => {
    const latest = new Map<string, ActivityEntry>();
    for (const entry of activity.entries()) {
      if (canLocate(entry) && !latest.has(entry.documentChange!.key)) latest.set(entry.documentChange!.key, entry);
    }
    return latest;
  });
  const locate = (entry: ActivityEntry) => {
    if (!canLocate(entry)) return;
    if (window.matchMedia('(max-width: 640px)').matches) activity.toggle();
    setSelectedChange(entry);
    queueMicrotask(() => document.getElementById(`docx-paragraph-${entry.documentChange!.key}`)?.focus());
  };
  const controller = new DocumentController({ ...view, render: (snapshot, focus) => {
    setStructuralDraft(Boolean(focus));
    const article = document.getElementById('document-content');
    const active = document.activeElement;
    const previousEditor = article?.querySelector<HTMLElement>('[data-continuous-editor]');
    const richSelection = previousEditor && readTextSelection(previousEditor);
    const previousRuns = [...article?.querySelectorAll<HTMLElement>('[data-docx-run]') ?? []];
    const fields = [...article?.querySelectorAll('textarea') ?? []];
    const previousField = active instanceof HTMLTextAreaElement ? active
      : state().saving ? fields.find(field => field.name === focusedRun()?.id) : undefined;
    const index = previousField && state().snapshot?.id === snapshot.id ? fields.indexOf(previousField) : -1;
    const selection = index >= 0 && previousField ? [previousField.selectionStart, previousField.selectionEnd] as const : undefined;
    const restoringSave = state().saving;
    if (state().snapshot?.id !== snapshot.id) setFocusedRun(undefined);
    view.render(snapshot);
    if (focus) {
      queueMicrotask(() => {
        const field = [...article?.querySelectorAll('textarea') ?? []].find(field => field.name === focus.runId);
        if (field) { field.focus({ preventScroll: true }); field.setSelectionRange(focus.start, focus.end); }
        else {
          const editor = article?.querySelector<HTMLElement>('[data-continuous-editor]');
          if (editor) restoreTextSelection(editor, focus.selection ?? { anchor: { runId: focus.runId, offset: focus.start }, focus: { runId: focus.runId, offset: focus.end } });
        }
      });
    } else if (richSelection && restoringSave) {
      queueMicrotask(() => {
        const editor = article?.querySelector<HTMLElement>('[data-continuous-editor]');
        const currentRuns = [...article?.querySelectorAll<HTMLElement>('[data-docx-run]') ?? []];
        const remap = (point: typeof richSelection.anchor) => {
          const index = previousRuns.findIndex(run => run.dataset.docxRun === point.runId);
          const run = index >= 0 ? currentRuns[Math.min(index, currentRuns.length - 1)] : undefined;
          return run ? { runId: run.dataset.docxRun!, offset: Math.min(point.offset, run.textContent?.length ?? 0) } : undefined;
        };
        const anchor = remap(richSelection.anchor), focus = remap(richSelection.focus);
        if (editor && anchor && focus) restoreTextSelection(editor, { anchor, focus });
      });
    } else if (selection && (restoringSave || document.activeElement === document.body)) {
      const field = article?.querySelectorAll('textarea')[index];
      field?.focus({ preventScroll: true });
      field?.setSelectionRange(...selection);
    }
  } });
  const [confirmReload, setConfirmReload] = createSignal(false);
  const cancelReload = () => { setConfirmReload(false); document.getElementById('reload-document')?.focus(); };
  const requestReload = () => {
    if (!controller.hasDrafts()) { reload(); return; }
    setConfirmReload(true);
    queueMicrotask(() => document.getElementById('discard-draft')?.focus());
  };
  const reload = () => {
    setConfirmReload(false);
    void controller.load().then(() => {
      document.getElementById(state().error ? 'document-error' : 'document-content')?.focus();
    });
  };
  const beforeUnload = (event: BeforeUnloadEvent) => {
    if (controller.hasDrafts()) { event.preventDefault(); event.returnValue = ''; }
  };
  const restoreHistory = (direction: 'undo' | 'redo') => {
    const editor = document.querySelector<HTMLElement>('[data-continuous-editor]');
    void controller.restore(direction, editor ? readTextSelection(editor) : undefined).then(changed => { if (changed) setDraftVersion(v => v + 1); });
  };
  const keydown = (event: KeyboardEvent) => {
    if (event.key === 'Escape' && confirmReload()) { event.preventDefault(); cancelReload(); return; }
    const key = event.key.toLowerCase();
    if ((event.ctrlKey || event.metaKey) && (key === 'z' || key === 'y')) {
      const target = event.target as HTMLElement | null;
      if (event.isComposing || target?.closest('input, select') || target?.closest('textarea') && !target.closest('#document-content')) return;
      event.preventDefault(); restoreHistory(key === 'y' || event.shiftKey ? 'redo' : 'undo'); return;
    }
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
      event.preventDefault();
      if (state().validation) { document.querySelector<HTMLTextAreaElement>('textarea[aria-invalid="true"]')?.focus(); return; }
      void controller.save();
    }
  };
  const editorState = () => ({ message: state().message, dirty: controller.hasDrafts(), busy: state().saving || state().loading });
  createEffect(() => { state(); draftVersion(); queueMicrotask(() => publishEditorState(editorState())); });
  let disconnectWorkspace = () => {};
  let disposed = false;
  const dispose = () => {
    if (disposed) return;
    disposed = true;
    window.removeEventListener('beforeunload', beforeUnload);
    window.removeEventListener('keydown', keydown);
    window.removeEventListener('pagehide', dispose);
    disconnectWorkspace();
    controller.dispose();
  };
  onMount(() => {
    disconnectWorkspace = connectWorkspaceBridge(editorState, async () => { await controller.save(); });
    window.addEventListener('beforeunload', beforeUnload);
    window.addEventListener('keydown', keydown);
    window.addEventListener('pagehide', dispose, { once: true });
    void controller.load();
  });
  onCleanup(dispose);
  return <>
    <a class="skip-link" href="#document-content">{t('skipToContent')}</a>
    <div class="flex h-full min-h-0 flex-col overflow-hidden">
    <EditorHeader name={state().snapshot?.name ?? t('openingDocument')} kind={t('document')} nameId="document-name" statusId="save-status"
      message={state().message} status={state().status} onActivity={activity.toggle} activityOpen={activity.open()}
      actions={<><FileActions saveID="save-document" disabled={!state().snapshot || state().saving || state().loading || Boolean(state().error || state().validation)} downloadAvailable onSave={() => controller.save()} onDownload={() => controller.download()}/><DocumentSearch version={`${state().snapshot?.id}:${state().snapshot?.revision}:${state().message}:${state().saving}:${draftVersion()}`} search={async (query) => searchDocument(document.getElementById('document-content'), query)} hint={t('searchBodyTextAndTablesIncludingUnsavedInput')}/></>}
    />
    <DocumentToolbar history={state().history} onHistory={restoreHistory} pasteMode={pasteMode()} onPasteMode={setPasteMode} saving={state().saving} loading={state().loading}
      needsRecovery={Boolean(state().error)} run={focusedRun()} zoom={zoom()} onZoom={setZoom}
      onReload={requestReload}/>
    <Show when={state().error}>{(message) => <StatusPanel id="document-error" panel={{ title: t('reviewRequired'), message: message(), details: state().errorDetails, dismissible: false }} danger onDismiss={() => {}}/>}</Show>
    <Show when={state().validation}>{(message) => <p class="shrink-0 border-b border-red-200 bg-red-50 px-4 py-2 text-xs leading-6 text-red-800 sm:px-6" role="alert">{localizeMessage(message())}</p>}</Show>
    <Show when={confirmReload()}><section class="shrink-0 border-b border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-950 sm:px-6" role="region" aria-label={t('confirmDiscardingUnsavedInput')}>
      <p>{t('discardUnsavedInputAndReloadTheSavedDocument')}</p>
      <div class="mt-3 flex flex-wrap gap-2"><button id="discard-draft" type="button" class={primaryButtonClass} onClick={reload}>{t('discardReload')}</button><button type="button" class={buttonClass} onClick={cancelReload}>{t('cancel')}</button></div>
    </section></Show>
    <div class="editor-workspace flex min-h-0 flex-1 overflow-hidden" data-activity-open={activity.open()}>
    <main class="editor-primary min-h-0 min-w-0 flex-1 space-y-4 overflow-auto bg-[#e9ebed] px-3 py-4 sm:px-8 sm:py-8">
      <div class="docx-preview mx-auto" style={paperStyle()}>
      <article id="document-content" aria-label={t('docxBodyAndTableEditor')} aria-busy={!state().snapshot} class="space-y-5 text-[11pt] leading-[1.35] text-stone-900" tabindex={-1}>
        <Show when={state().snapshot}>{(snapshot) => <DocumentContent onHistory={restoreHistory} pasteMode={pasteMode()} onPasteRich={(selection, paragraphs) => { const changed=controller.pasteRich(selection,paragraphs);if(changed)setDraftVersion(v=>v+1);return changed; }} richSelection={selection=>controller.richSelection(selection)} snapshot={snapshot()} saving={state().saving || state().loading} changes={changes()}
          selected={selectedChange() && canLocate(selectedChange()!) ? selectedChange() : undefined}
          onRange={(selection, text, inline) => { const changed = controller.replaceSelection(selection, text, inline); if (changed) setDraftVersion(v => v + 1); return changed; }}
          onDelete={(selection, direction, word) => { const changed = controller.deleteSelection(selection, direction, word); if (changed) setDraftVersion(v => v + 1); return changed; }}
          onComposition={active => controller.composition(active)} onRefresh={selection => controller.refreshSelection(selection)} selectionText={selection => controller.selectionText(selection)}
          onStructure={(action, id, start, end, insertion) => { const changed = controller.changeStructure(action, id, start, end, insertion); if (changed) setDraftVersion(v => v + 1); return changed; }}
          onFocusRun={setFocusedRun} onChangeSelect={(entry) => setSelectedChange(selectedChange()?.documentChange?.key === entry.documentChange?.key ? undefined : entry)} onEdit={(operation, selection) => { controller.edit(operation, selection); setDraftVersion((version) => version + 1); }}/>}</Show>
      </article>
      <details class="mt-2 rounded-md px-2 py-2 text-xs leading-6 text-stone-600">
        <summary class="editor-action min-h-11 w-fit rounded px-1 py-2 font-medium">{t('editingSupportAndPreservation')}</summary>
        <p>{t('previewPageSizeSomeStylesAndExplicitPage')}</p><p>{t('paragraphEditingHelp')}</p>
        <Show when={state().snapshot?.warnings.length}><p class="whitespace-pre-wrap">{state().snapshot?.warnings.map(warningMessage).join('\n')}</p></Show>
      <DiagnosticDetails value={state().snapshot?.warnings.join('\n')}/>
      </details>
      </div>
    </main>
    <Show when={activity.open()}><ActivityPanel entries={activity.entries()} onClose={activity.toggle} canLocate={canLocate} onLocate={locate}/></Show>
    </div>
    </div>
  </>;
}
