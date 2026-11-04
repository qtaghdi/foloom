import { For } from 'solid-js';
import type { AppView } from './app-view';
import { buttonClass } from './editor-chrome';

export function WorkbookToolbar(props: { view: AppView }) {
  const state = props.view.state;
  const blocked = () => !state().selection || ['pending', 'conflict', 'recovery'].includes(state().status);
  const sizes = () => [...new Set([8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 32, 36, 48, 72, state().selection?.fontSize ?? 11])].sort((a, b) => a - b);
  return <div class="editor-toolstrip flex min-h-14 min-w-0 shrink-0 flex-wrap items-center gap-2 border-b border-slate-200 bg-white px-4 py-2 sm:px-6" aria-label="스프레드시트 편집 도구">
    <div class="flex gap-1" role="group" aria-label="문서 변경 이력">
      <button id="history-undo" type="button" class={buttonClass} disabled={!state().canUndo || state().status === 'pending'} onClick={() => props.view.history('undo')} title="실행 취소" aria-label="실행 취소"><span aria-hidden="true" class="text-lg">↶</span></button>
      <button id="history-redo" type="button" class={buttonClass} disabled={!state().canRedo || state().status === 'pending'} onClick={() => props.view.history('redo')} title="다시 실행" aria-label="다시 실행"><span aria-hidden="true" class="text-lg">↷</span></button>
    </div>
    <button type="button" class={buttonClass} classList={{ 'text-indigo-700': state().selection?.bold }} aria-pressed={state().selection?.bold ?? false} disabled={blocked()} onClick={() => props.view.format({ bold: !state().selection?.bold })} title="굵게" aria-label="굵게"><span aria-hidden="true" class="font-bold">B</span></button>
    <button type="button" class={`${buttonClass} italic`} aria-pressed={state().selection?.italic ?? false} disabled={blocked()} onClick={() => props.view.format({ italic: !state().selection?.italic })} title="기울임" aria-label="기울임"><span aria-hidden="true" class="italic">I</span></button>
    <label class="flex min-h-11 items-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-xs text-slate-600">글자 크기
      <select aria-label="글자 크기" name="fontSize" class="min-h-10 bg-white text-sm text-slate-900 focus-visible:outline-2 focus-visible:outline-indigo-600" disabled={blocked()} value={state().selection?.fontSize ?? 11} onChange={(event) => props.view.format({ fontSize: Number(event.currentTarget.value) })}>
        <For each={sizes()}>{(size) => <option value={size}>{size}</option>}</For>
      </select>
    </label>
    <button type="button" class={`${buttonClass} sm:ml-auto`} aria-expanded={state().nativeTools ?? false} disabled={['pending', 'conflict', 'recovery'].includes(state().status)} onClick={() => props.view.toggleNativeTools()}>고급 도구</button>
  </div>;
}
