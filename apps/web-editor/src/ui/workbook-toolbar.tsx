import { For } from 'solid-js';
import type { AppView } from './app-view';

const icons = {
  undo: 'M9 5 4 10l5 5M4 10h10a5 5 0 0 1 0 10h-2',
  redo: 'm15 5 5 5-5 5m5-5H10a5 5 0 0 0 0 10h2',
  bold: 'M8 12h5a3.5 3.5 0 0 1 0 7H8V5h4.5a3.5 3.5 0 0 1 0 7',
  italic: 'M12 5h7M5 19h7m4-14L8 19',
  chevron: 'm6 9 6 6 6-6',
};

function Icon(props: { name: keyof typeof icons; size?: number }) {
  return <svg aria-hidden="true" width={props.size ?? 19} height={props.size ?? 19} viewBox="0 0 24 24"
    fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
    <path d={icons[props.name]}/>
  </svg>;
}

const toolClass = 'sheet-tool inline-flex min-h-11 min-w-11 shrink-0 items-center justify-center rounded-md text-stone-700 hover:bg-stone-200/70 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-teal-700 disabled:text-stone-400 disabled:hover:bg-transparent';

export function WorkbookToolbar(props: { view: AppView }) {
  const state = props.view.state;
  const busy = () => ['pending', 'conflict', 'recovery'].includes(state().status);
  const blocked = () => !state().selection || busy();
  const sizes = () => [...new Set([8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 32, 36, 48, 72, state().selection?.fontSize ?? 11])].sort((a, b) => a - b);

  return <div class="editor-toolstrip flex min-w-0 shrink-0 flex-wrap items-center gap-x-1 gap-y-1 border-b border-stone-200 bg-stone-50 px-3 py-1 sm:gap-x-2 sm:px-5" aria-label="스프레드시트 편집 도구">
    <div class="flex items-center border-r border-stone-200 pr-2" role="group" aria-label="문서 변경 이력">
      <button id="history-undo" type="button" class={toolClass} disabled={!state().canUndo || busy()}
        onClick={() => props.view.history('undo')} title="실행 취소" aria-label="실행 취소"><Icon name="undo"/></button>
      <button id="history-redo" type="button" class={toolClass} disabled={!state().canRedo || busy()}
        onClick={() => props.view.history('redo')} title="다시 실행" aria-label="다시 실행"><Icon name="redo"/></button>
    </div>
    <div class="flex items-center gap-0.5" role="group" aria-label="글자 서식">
      <div class="relative mr-1 inline-flex items-center">
        <select aria-label="글자 크기" title="글자 크기" name="fontSize"
          class="min-h-11 w-14 appearance-none sm:w-16 rounded-md bg-transparent pl-3 pr-6 text-sm font-medium tabular-nums text-stone-700 hover:bg-stone-200/70 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-teal-700 disabled:text-stone-400 disabled:hover:bg-transparent"
          disabled={blocked()} value={state().selection?.fontSize ?? 11}
          onChange={(event) => props.view.format({ fontSize: Number(event.currentTarget.value) })}>
          <For each={sizes()}>{(size) => <option value={size}>{size}</option>}</For>
        </select>
        <span class="pointer-events-none absolute right-2 text-stone-500"><Icon name="chevron" size={13}/></span>
      </div>
      <button type="button" class={toolClass} aria-pressed={state().selection?.bold ?? false} disabled={blocked()}
        onClick={() => props.view.format({ bold: !state().selection?.bold })} title="굵게" aria-label="굵게"><Icon name="bold"/></button>
      <button type="button" class={toolClass} aria-pressed={state().selection?.italic ?? false} disabled={blocked()}
        onClick={() => props.view.format({ italic: !state().selection?.italic })} title="기울임" aria-label="기울임"><Icon name="italic"/></button>
    </div>
    <button type="button" class={`${toolClass} ml-auto gap-1.5 px-3 text-sm font-medium`} aria-expanded={state().nativeTools ?? false}
      disabled={busy()} onClick={() => props.view.toggleNativeTools()}>
      고급 도구
      <span classList={{ 'rotate-180': state().nativeTools }}><Icon name="chevron" size={14}/></span>
    </button>
  </div>;
}
