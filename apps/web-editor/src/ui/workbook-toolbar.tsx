import { For } from 'solid-js';
import type { AppView } from './app-view';
import { buttonClass } from './editor-chrome';

export function WorkbookToolbar(props: { view: AppView }) {
  const state = props.view.state;
  const blocked = () => !state().selection || ['pending', 'conflict', 'recovery'].includes(state().status);
  const sizes = () => [...new Set([8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 32, 36, 48, 72, state().selection?.fontSize ?? 11])].sort((a, b) => a - b);
  return <div class="editor-toolstrip flex min-h-14 min-w-0 shrink-0 flex-wrap items-center gap-2 border-b border-slate-200 bg-white px-4 py-2 sm:px-6" aria-label="스프레드시트 편집 도구">
    <div class="flex gap-1" role="group" aria-label="문서 변경 이력">
      <button id="history-undo" type="button" class={buttonClass} disabled={!state().canUndo || state().status === 'pending'} onClick={() => props.view.history('undo')}>실행 취소</button>
      <button id="history-redo" type="button" class={buttonClass} disabled={!state().canRedo || state().status === 'pending'} onClick={() => props.view.history('redo')}>다시 실행</button>
    </div>
    <output class="min-w-28 max-w-60 truncate rounded border border-slate-200 px-3 py-2 text-sm text-slate-600" aria-label="선택 범위" title={state().selection ? `${state().selection?.sheet}!${state().selection?.range}` : undefined}>{state().selection?.range ?? '셀을 선택하세요'}</output>
    <button type="button" class={buttonClass} classList={{ 'text-indigo-700': state().selection?.bold }} aria-pressed={state().selection?.bold ?? false} disabled={blocked()} onClick={() => props.view.format({ bold: !state().selection?.bold })}>굵게</button>
    <button type="button" class={`${buttonClass} italic`} aria-pressed={state().selection?.italic ?? false} disabled={blocked()} onClick={() => props.view.format({ italic: !state().selection?.italic })}>기울임</button>
    <label class="flex min-h-11 items-center gap-2 rounded-md border border-slate-300 bg-white px-3 text-xs text-slate-600">글자 크기
      <select aria-label="글자 크기" name="fontSize" class="min-h-10 bg-white text-sm text-slate-900 focus-visible:outline-2 focus-visible:outline-indigo-600" disabled={blocked()} value={state().selection?.fontSize ?? 11} onChange={(event) => props.view.format({ fontSize: Number(event.currentTarget.value) })}>
        <For each={sizes()}>{(size) => <option value={size}>{size}</option>}</For>
      </select>
    </label>
    <button type="button" class={`${buttonClass} sm:ml-auto`} aria-expanded={state().nativeTools ?? false} disabled={['pending', 'conflict', 'recovery'].includes(state().status)} onClick={() => props.view.toggleNativeTools()}>고급 도구</button>
  </div>;
}
