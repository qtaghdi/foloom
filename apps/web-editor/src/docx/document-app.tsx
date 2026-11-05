import { DocumentSearch } from '../ui/document-search';
import { searchDocument } from './search';
import { Show, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { DocumentController } from './controller';
import { createDocumentView } from './view';
import { StatusPanel } from '../ui/status-panel';
import type { ActivityEntry } from '../ui/activity';
import { ActivityPanel } from '../ui/activity-panel';
import { DocumentContent } from './document-content';
import { DocumentToolbar } from './document-toolbar';
import type { Run } from './contracts';
import { EditorHeader, buttonClass, primaryButtonClass } from '../ui/editor-chrome';



const defaultPaper = { width: 11906, height: 16838, top: 1440, right: 1440, bottom: 1440, left: 1440 };

export function DocumentApp() {
  const { state, view, activity } = createDocumentView();
  const [draftVersion, setDraftVersion] = createSignal(0);
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
  const [selectedChange, setSelectedChange] = createSignal<ActivityEntry>();
  const canLocate = (entry: ActivityEntry) => Boolean(entry.documentChange && entry.documentChange.session === state().snapshot?.id);
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
  const controller = new DocumentController({ ...view, render: (snapshot) => {
    const article = document.getElementById('document-content');
    const active = document.activeElement;
    const fields = [...article?.querySelectorAll('textarea') ?? []];
    const index = active instanceof HTMLTextAreaElement && state().snapshot?.id === snapshot.id ? fields.indexOf(active) : -1;
    const selection = index >= 0 && active instanceof HTMLTextAreaElement
      ? [active.selectionStart, active.selectionEnd] as const : undefined;
    if (state().snapshot?.id !== snapshot.id) setFocusedRun(undefined);
    view.render(snapshot);
    if (selection && document.activeElement === document.body) {
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
  const keydown = (event: KeyboardEvent) => {
    if (event.key === 'Escape' && confirmReload()) { event.preventDefault(); cancelReload(); return; }
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
      event.preventDefault();
      if (state().validation) { document.querySelector<HTMLTextAreaElement>('textarea[aria-invalid="true"]')?.focus(); return; }
      void controller.save();
    }
  };
  let disposed = false;
  const dispose = () => {
    if (disposed) return;
    disposed = true;
    window.removeEventListener('beforeunload', beforeUnload);
    window.removeEventListener('keydown', keydown);
    window.removeEventListener('pagehide', dispose);
    controller.dispose();
  };
  onMount(() => {
    window.addEventListener('beforeunload', beforeUnload);
    window.addEventListener('keydown', keydown);
    window.addEventListener('pagehide', dispose, { once: true });
    void controller.load();
  });
  onCleanup(dispose);
  return <>
    <a class="skip-link" href="#document-content">본문으로 이동</a>
    <div class="flex h-full min-h-0 flex-col overflow-hidden">
    <EditorHeader name={state().snapshot?.name ?? '문서를 여는 중'} kind="문서" nameId="document-name" statusId="save-status"
      message={state().message} status={state().status} onActivity={activity.toggle} activityOpen={activity.open()}
      actions={<DocumentSearch version={`${state().snapshot?.id}:${state().snapshot?.revision}:${state().message}:${state().saving}:${draftVersion()}`} search={async (query) => searchDocument(document.getElementById('document-content'), query)} hint="본문과 표에서 미저장 입력을 포함해 찾습니다. 대소문자를 구분하지 않습니다."/>}
    />
    <DocumentToolbar canSave={state().canSave} saving={state().saving} loading={state().loading}
      needsRecovery={Boolean(state().error)} run={focusedRun()} zoom={zoom()} onZoom={setZoom}
      onSave={() => { void controller.save(); }} onReload={requestReload}/>
    <Show when={state().error}>{(message) => <StatusPanel id="document-error" panel={{ title: '작업 확인 필요', message: message(), dismissible: false }} danger onDismiss={() => {}}/>}</Show>
    <Show when={state().validation}>{(message) => <p class="shrink-0 border-b border-red-200 bg-red-50 px-4 py-2 text-xs leading-6 text-red-800 sm:px-6" role="alert">{message()}</p>}</Show>
    <Show when={confirmReload()}><section class="shrink-0 border-b border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-950 sm:px-6" role="region" aria-label="미저장 입력 폐기 확인">
      <p>미저장 입력을 버리고 저장된 문서를 다시 불러올까요? 필요한 입력을 먼저 복사하세요.</p>
      <div class="mt-3 flex flex-wrap gap-2"><button id="discard-draft" type="button" class={primaryButtonClass} onClick={reload}>입력 버리고 불러오기</button><button type="button" class={buttonClass} onClick={cancelReload}>취소</button></div>
    </section></Show>
    <div class="editor-workspace flex min-h-0 flex-1 overflow-hidden" data-activity-open={activity.open()}>
    <main class="editor-primary min-h-0 min-w-0 flex-1 space-y-4 overflow-auto bg-[#e9ebed] px-3 py-4 sm:px-8 sm:py-8">
      <div class="docx-preview mx-auto" style={paperStyle()}>
      <article id="document-content" aria-label="DOCX 본문과 표 편집" aria-busy={!state().snapshot} class="space-y-5 text-[11pt] leading-[1.35] text-stone-900" tabindex={-1}>
        <Show when={state().snapshot}>{(snapshot) => <DocumentContent snapshot={snapshot()} saving={state().saving || state().loading} changes={changes()}
          selected={selectedChange() && canLocate(selectedChange()!) ? selectedChange() : undefined}
          onFocusRun={setFocusedRun} onChangeSelect={(entry) => setSelectedChange(selectedChange()?.documentChange?.key === entry.documentChange?.key ? undefined : entry)} onEdit={(operation) => { controller.edit(operation); setDraftVersion((version) => version + 1); }}/>}</Show>
      </article>
      <details class="mt-2 rounded-md px-2 py-2 text-xs leading-6 text-stone-600">
        <summary class="editor-action min-h-11 w-fit rounded px-1 py-2 font-medium">편집 범위와 원본 보존</summary>
        <p>용지, 일부 스타일과 명시적으로 나눈 페이지를 미리 봅니다. 내용의 길이에 따른 자동 페이지 매김은 지원하지 않습니다. 본문과 표의 기존 텍스트를 편집할 수 있습니다.</p>
        <Show when={state().snapshot?.warnings.length}><p class="whitespace-pre-wrap">{state().snapshot?.warnings.join('\n')}</p></Show>
      </details>
      </div>
    </main>
    <Show when={activity.open()}><ActivityPanel entries={activity.entries()} onClose={activity.toggle} canLocate={canLocate} onLocate={locate}/></Show>
    </div>
    </div>
  </>;
}
