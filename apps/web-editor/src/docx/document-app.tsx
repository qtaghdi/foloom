import { For, Show, createSignal, onCleanup, onMount } from 'solid-js';
import { DocumentController } from './controller';
import { createDocumentView } from './view';
import { StatusPanel } from '../ui/status-panel';
import { ActivityPanel } from '../ui/activity-panel';
import { DocumentContent } from './document-content';
import { EditorHeader, buttonClass, primaryButtonClass } from '../ui/editor-chrome';



export function DocumentApp() {
  const { state, view, activity } = createDocumentView();
  const controller = new DocumentController(view);
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
      />
    <div class="editor-toolstrip flex min-h-14 shrink-0 flex-wrap items-center gap-2 border-b border-slate-200 bg-white px-4 py-2 sm:px-6" aria-label="문서 편집 도구">
      <button id="reload-document" class={buttonClass} type="button" disabled={state().saving} onClick={requestReload}>다시 불러오기</button>
      <button id="save-document" class={primaryButtonClass} type="button" disabled={!state().canSave} onClick={() => { void controller.save(); }}>변경 저장</button>
      <span class="hidden border-l border-slate-200 pl-4 text-xs text-slate-500 sm:inline">본문과 표의 텍스트 편집</span>
    </div>
    <Show when={state().snapshot?.warnings.length}><StatusPanel id="document-warnings" panel={{ title: '편집 범위 안내', message: state().snapshot?.warnings.join('\n\n') ?? '', dismissible: false }} onDismiss={() => {}}/></Show>
    <Show when={state().error}>{(message) => <StatusPanel id="document-error" panel={{ title: '작업 확인 필요', message: message(), dismissible: false }} danger onDismiss={() => {}}/>}</Show>
    <Show when={state().validation}>{(message) => <p class="shrink-0 border-b border-red-200 bg-red-50 px-4 py-2 text-xs leading-6 text-red-800 sm:px-6" role="alert">{message()}</p>}</Show>
    <Show when={confirmReload()}><section class="shrink-0 border-b border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-950 sm:px-6" role="region" aria-label="미저장 입력 폐기 확인">
      <p>미저장 입력을 버리고 저장된 문서를 다시 불러올까요? 필요한 입력을 먼저 복사하세요.</p>
      <div class="mt-3 flex flex-wrap gap-2"><button id="discard-draft" type="button" class={primaryButtonClass} onClick={reload}>입력 버리고 불러오기</button><button type="button" class={buttonClass} onClick={cancelReload}>취소</button></div>
    </section></Show>
    <div class="editor-workspace flex min-h-0 flex-1 overflow-hidden" data-activity-open={activity.open()}>
    <main class="editor-primary min-h-0 min-w-0 flex-1 space-y-4 overflow-auto bg-slate-100 px-3 py-5 sm:px-8 sm:py-8">
      <article id="document-content" aria-label="DOCX 본문과 표 편집" aria-busy={!state().snapshot} class="docx-paper mx-auto min-h-[42rem] w-full max-w-[800px] space-y-6 border border-slate-300 bg-white px-5 py-8 text-base leading-8 text-slate-900 sm:px-16 sm:py-16" tabindex={-1}>
        <Show when={state().snapshot}>{(snapshot) => <DocumentContent snapshot={snapshot()} saving={state().saving} onEdit={(operation) => controller.edit(operation)}/>}</Show>
      </article>
      <p class="mx-auto max-w-[800px] text-xs leading-5 text-slate-500">지원하지 않는 구조는 원본을 유지합니다. 줄바꿈과 탭 입력은 현재 지원하지 않습니다.</p>
    </main>
    <Show when={activity.open()}><ActivityPanel entries={activity.entries()} onClose={activity.toggle}/></Show>
    </div>
    <footer class="border-t border-slate-200 bg-white px-4 py-2 text-xs text-slate-500">문서 배치는 미리보기입니다.</footer>
    </div>
  </>;
}
