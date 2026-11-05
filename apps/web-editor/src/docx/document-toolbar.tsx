import { For } from 'solid-js';
import { buttonClass, quietButtonClass } from '../ui/editor-chrome';
import type { PasteMode } from './clipboard';
import type { Run } from './contracts';

const scales = [75, 90, 100, 110, 125, 150];

export function DocumentToolbar(props: {
  pasteMode: PasteMode; onPasteMode: (mode: PasteMode) => void;
  saving: boolean; loading: boolean; needsRecovery: boolean;
  run?: Run; zoom: number;
  onZoom: (value: number) => void; onReload: () => void;
}) {
  const format = () => {
    const run = props.run;
    return run ? `${run.font ?? '기본 글꼴'} · ${(run.sizeHalfPoints ?? 22) / 2} pt` : '본문 서식';
  };
  return <div role="group" aria-label="문서 도구" class="flex min-w-0 shrink-0 flex-wrap items-center gap-x-2 gap-y-1 border-b border-slate-200 bg-stone-50 px-3 py-1 sm:px-6">
    <button id="reload-document" class={props.needsRecovery ? buttonClass : quietButtonClass} type="button"
      disabled={props.saving || props.loading} title="다시 불러오기" aria-label="다시 불러오기" onClick={props.onReload}>
      <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7">
        <path d="M20 7v5h-5M20 12a8 8 0 1 0-2 5.3" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    </button>
    <label class="flex min-h-11 items-center gap-2 text-xs text-slate-700">
      <span class="hidden sm:inline">붙여넣기</span>
      <select aria-label="붙여넣기 서식" class="editor-select min-h-11 max-w-36 rounded-md bg-stone-50 px-2" value={props.pasteMode}
        onChange={event => props.onPasteMode(event.currentTarget.value === 'basic' ? 'basic' : 'text')}>
        <option value="text">텍스트만</option><option value="basic">기본 서식 유지</option>
      </select>
    </label>
    <div class="flex min-w-0 flex-1 items-center gap-2 border-l border-slate-200 pl-2 sm:gap-3">
      <output class="min-w-0 truncate text-xs text-slate-700 sm:text-sm" aria-label="현재 글자 서식" title={format()}>{format()}</output>
      <label class="ml-auto flex min-h-11 shrink-0 items-center gap-2 rounded-md px-2 text-xs text-slate-700">
        <span class="hidden sm:inline">보기 배율</span>
        <select aria-label="보기 배율" name="document-zoom" value={props.zoom}
          class="editor-select min-h-11 cursor-pointer rounded-md bg-stone-50 px-2 text-sm tabular-nums"
          onChange={(event) => props.onZoom(Number(event.currentTarget.value))}>
          <For each={scales}>{scale => <option value={scale}>{scale}%</option>}</For>
        </select>
      </label>
    </div>
  </div>;
}
