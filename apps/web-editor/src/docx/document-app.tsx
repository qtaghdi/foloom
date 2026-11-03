import { For, Show, createSignal, onCleanup, onMount } from 'solid-js';
import { DocumentController } from './controller';
import { createDocumentView } from './view';
import { DocumentContent } from './document-content';
import { EditorHeader, buttonClass, primaryButtonClass } from '../ui/editor-chrome';



export function DocumentApp() {
  const { state, view } = createDocumentView();
  const controller = new DocumentController(view);
  const [confirmReload, setConfirmReload] = createSignal(false);
  const reload = () => {
    setConfirmReload(false);
    void controller.load();
  };
  const beforeUnload = (event: BeforeUnloadEvent) => {
    if (controller.hasDrafts()) { event.preventDefault(); event.returnValue = ''; }
  };
  const keydown = (event: KeyboardEvent) => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') { event.preventDefault(); void controller.save(); }
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
    <EditorHeader name={state().snapshot?.name ?? '문서를 여는 중'} kind="문서" nameId="document-name" statusId="save-status"
      message={state().message} status={state().status}
      actions={<>
        <button id="reload-document" class={buttonClass} type="button" disabled={state().saving} onClick={() => controller.hasDrafts() ? setConfirmReload(true) : reload()}>다시 불러오기</button>
        <button id="save-document" class={primaryButtonClass} type="button" disabled={!state().canSave} onClick={() => { void controller.save(); }}>변경 저장</button>
      </>}/>
    <main class="mx-auto max-w-5xl space-y-5 px-3 py-6 sm:px-6">
      <details class="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-950"><summary class="cursor-pointer font-semibold">편집 범위와 원본 보존 안내</summary><ul id="document-warnings" class="mt-3 list-disc space-y-2 pl-5"><For each={state().snapshot?.warnings ?? []}>{(warning) => <li>{warning}</li>}</For></ul></details>
      <p class="text-sm text-slate-600">본문과 표의 텍스트를 수정한 뒤 ‘변경 저장’을 누르세요. 서식이 다른 텍스트는 별도 입력칸으로 표시됩니다. 줄바꿈과 탭은 현재 지원하지 않습니다.</p>
      <Show when={confirmReload()}><section class="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm" role="alert">
        <p>미저장 입력을 버리고 저장된 문서를 다시 불러올까요? 필요한 입력을 먼저 복사하세요.</p>
        <div class="mt-3 flex flex-wrap gap-2"><button class="min-h-11 rounded bg-indigo-600 px-3 text-white" onClick={reload}>입력 버리고 불러오기</button><button class="min-h-11 rounded border px-3" onClick={() => setConfirmReload(false)}>취소</button></div>
      </section></Show>
      <Show when={state().error}><div id="document-error" class="rounded-lg border border-red-300 bg-red-50 p-4 text-sm whitespace-pre-wrap text-red-900" role="alert">{state().error}</div></Show>
      <article id="document-content" aria-label="DOCX 본문과 표 편집" aria-busy={!state().snapshot} class="min-h-96 space-y-5 rounded-lg border border-slate-200 bg-white p-4 shadow-sm sm:p-10" tabindex={-1}>
        <Show when={state().snapshot}>{(snapshot) => <DocumentContent snapshot={snapshot()} saving={state().saving} onEdit={(operation) => controller.edit(operation)}/>}</Show>
      </article>
    </main>
  </>;
}
