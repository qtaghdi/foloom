import { Show, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { DocumentController } from './controller';
import { createDocumentView } from './view';
import { StatusPanel } from '../ui/status-panel';
import type { ActivityEntry } from '../ui/activity';
import { ActivityPanel } from '../ui/activity-panel';
import { DocumentContent } from './document-content';
import { EditorHeader, buttonClass, primaryButtonClass, quietButtonClass } from '../ui/editor-chrome';



export function DocumentApp() {
  const { state, view, activity } = createDocumentView();
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
      actions={<>
        <Show when={state().canSave || state().saving || state().validation || state().error}>
          <button id="save-document" class={primaryButtonClass} type="button" disabled={!state().canSave} onClick={() => { void controller.save(); }}>변경 저장</button>
        </Show>
        <button id="reload-document" class={state().error ? buttonClass : quietButtonClass} type="button" disabled={state().saving || state().loading} onClick={requestReload} title="다시 불러오기" aria-label="다시 불러오기">
          <Show when={state().error} fallback={<svg aria-hidden="true" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path d="M20 7v5h-5M20 12a8 8 0 1 0-2 5.3" stroke-linecap="round" stroke-linejoin="round"/></svg>}>다시 불러오기</Show>
        </button>
      </>}/>
    <Show when={state().error}>{(message) => <StatusPanel id="document-error" panel={{ title: '작업 확인 필요', message: message(), dismissible: false }} danger onDismiss={() => {}}/>}</Show>
    <Show when={state().validation}>{(message) => <p class="shrink-0 border-b border-red-200 bg-red-50 px-4 py-2 text-xs leading-6 text-red-800 sm:px-6" role="alert">{message()}</p>}</Show>
    <Show when={confirmReload()}><section class="shrink-0 border-b border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-950 sm:px-6" role="region" aria-label="미저장 입력 폐기 확인">
      <p>미저장 입력을 버리고 저장된 문서를 다시 불러올까요? 필요한 입력을 먼저 복사하세요.</p>
      <div class="mt-3 flex flex-wrap gap-2"><button id="discard-draft" type="button" class={primaryButtonClass} onClick={reload}>입력 버리고 불러오기</button><button type="button" class={buttonClass} onClick={cancelReload}>취소</button></div>
    </section></Show>
    <div class="editor-workspace flex min-h-0 flex-1 overflow-hidden" data-activity-open={activity.open()}>
    <main class="editor-primary min-h-0 min-w-0 flex-1 space-y-4 overflow-auto bg-stone-100 px-3 py-4 sm:px-8 sm:py-8">
      <article id="document-content" aria-label="DOCX 본문과 표 편집" aria-busy={!state().snapshot} class="docx-paper mx-auto min-h-[42rem] w-full max-w-[780px] space-y-4 rounded-sm border border-stone-200 bg-white px-6 py-9 text-[17px] leading-8 text-stone-900 shadow-sm sm:px-16 sm:py-14" tabindex={-1}>
        <Show when={state().snapshot}>{(snapshot) => <DocumentContent snapshot={snapshot()} saving={state().saving || state().loading} changes={changes()}
          selected={selectedChange() && canLocate(selectedChange()!) ? selectedChange() : undefined}
          onChangeSelect={(entry) => setSelectedChange(selectedChange()?.documentChange?.key === entry.documentChange?.key ? undefined : entry)} onEdit={(operation) => controller.edit(operation)}/>}</Show>
      </article>
      <details class="mx-auto max-w-[780px] rounded-md px-2 py-2 text-xs leading-6 text-stone-600">
        <summary class="w-fit cursor-pointer font-medium">편집 범위와 원본 보존</summary>
        <p>본문과 표의 텍스트를 편집할 수 있습니다. 줄바꿈과 탭 입력은 지원하지 않으며 화면의 페이지 배치는 미리보기입니다.</p>
        <Show when={state().snapshot?.warnings.length}><p class="whitespace-pre-wrap">{state().snapshot?.warnings.join('\n')}</p></Show>
      </details>
    </main>
    <Show when={activity.open()}><ActivityPanel entries={activity.entries()} onClose={activity.toggle} canLocate={canLocate} onLocate={locate}/></Show>
    </div>
    </div>
  </>;
}
