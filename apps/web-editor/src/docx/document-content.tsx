import { For, Show, createMemo, createSignal } from 'solid-js';
import { validateRunText } from './validation';
import type { ActivityEntry } from '../ui/activity';
import type { Snapshot, Run, Paragraph, Operation } from './contracts';

export function DocumentContent(props: { snapshot: Snapshot; saving: boolean; onEdit: (operation: Operation) => void; changes: ReadonlyMap<string, ActivityEntry>; selected?: ActivityEntry; onChangeSelect: (entry: ActivityEntry) => void }) {
  const positions = createMemo(() => {
    const result = new Map<string, number>();
    for (const block of props.snapshot.blocks) {
      const paragraphs = block.type === 'paragraph' ? [block.paragraph] : block.rows.flatMap((row) => row.cells.flatMap((cell) => cell.paragraphs));
      for (const paragraph of paragraphs) for (const run of paragraph.runs) result.set(run.id, result.size + 1);
    }
    return result;
  });
  const paragraph = (p: Paragraph, kind: Operation['type'], key: string) => <section
    class="docx-paragraph relative min-w-0" id={`docx-paragraph-${key}`} tabindex={-1}
    style={{
      'margin-block-start': p.layout?.before === undefined ? undefined : `${p.layout.before / 20}pt`,
      'margin-block-end': p.layout?.after === undefined ? undefined : `${p.layout.after / 20}pt`,
    }}>
    <div class="min-w-0" classList={{ 'pr-12': props.changes.has(key) }} style={{ 'text-align': p.layout?.align }}>
      <Show when={p.runs.length} fallback={<span aria-hidden="true">&nbsp;</span>}>
        <For each={p.runs}>{(run) => <Show when={run.editable} fallback={<span class="whitespace-pre-wrap" style={runStyle(run)} title="읽기 전용: 원본 구조를 보존합니다.">{run.text || '표시되지 않는 구조'}</span>}>
          <RunField run={run} fullWidth={p.runs.length === 1} align={p.layout?.align}
            label={`${kind === 'replace_cell_text' ? '표' : '본문'} 텍스트 ${positions().get(run.id)}`} saving={props.saving}
            onEdit={(text) => props.onEdit({ type: kind, targetId: run.id, expectedText: run.text, text })}/>
        </Show>}</For>
      </Show>
    </div>
    <Show when={props.changes.get(key)}>{(entry) => <button type="button" class="editor-action change-marker" aria-label={`AI 변경 보기 · ${entry().location}`} aria-expanded={props.selected?.documentChange?.key === key}
      onClick={() => props.onChangeSelect(entry())}>AI</button>}</Show>
    <Show when={props.selected?.documentChange?.key === key ? props.selected : undefined}>{(entry) =>
      <div class="mt-3 space-y-2 rounded-md border border-teal-200 bg-teal-50 p-3 text-sm leading-6 text-stone-800" role="region" aria-label="AI 변경 전후">
        <p class="font-semibold text-teal-900">AI 변경 · 리비전 {entry().revision}</p>
        <p class="break-words whitespace-pre-wrap"><span class="mr-2 font-medium">이전</span><del>{entry().documentChange?.before || '빈 텍스트'}</del></p>
        <p class="break-words whitespace-pre-wrap"><span class="mr-2 font-medium">이후</span>{entry().documentChange?.after || '빈 텍스트'}</p>
      </div>}
    </Show>
  </section>;
  return <Show when={props.snapshot.blocks.length} fallback={<p>표시할 수 있는 본문이 없습니다. 원본 문서는 보존됩니다.</p>}>
    <For each={props.snapshot.blocks}>{(block, blockIndex) => block.type === 'paragraph' ? paragraph(block.paragraph, 'replace_run_text', `${blockIndex()}`) :
      <div class="docx-table overflow-x-auto" tabindex={0} aria-label="문서 표. 좁은 화면에서는 좌우로 스크롤하세요.">
        <table class="w-full border-collapse text-left"><tbody><For each={block.rows}>{(row, rowIndex) => <tr><For each={row.cells}>{(cell, cellIndex) =>
          <td class="min-w-24 border border-slate-300 px-2 py-1 align-top"><For each={cell.paragraphs}>{(p, pIndex) => paragraph(p, 'replace_cell_text', `${blockIndex()}-${rowIndex()}-${cellIndex()}-${pIndex()}`)}</For></td>
        }</For></tr>}</For></tbody></table>
      </div>
    }</For>
  </Show>;
}

function runStyle(run: Run) {
  return {
    'font-size': run.sizeHalfPoints === undefined ? undefined : `${run.sizeHalfPoints / 2}pt`,
    'font-family': run.font ? `${JSON.stringify(run.font)}, system-ui, sans-serif` : undefined,
    color: run.color,
    'font-weight': run.bold ? '700' : undefined,
    'font-style': run.italic ? 'italic' : undefined,
  };
}

function RunField(props: { run: Run; fullWidth: boolean; align?: 'start' | 'center' | 'end' | 'justify'; label: string; saving: boolean; onEdit: (text: string) => void }) {
  const [error, setError] = createSignal('');
  const descriptionId = `input-error-${props.run.id.replace(/[^a-zA-Z0-9_-]/g, '-')}`;
  return <>
    <textarea value={props.run.text} rows={Math.max(1, Math.ceil(props.run.text.length / 72))} readOnly={props.saving}
      name={props.run.id} autocomplete="off" aria-label={props.label} aria-invalid={Boolean(error())} aria-describedby={error() ? descriptionId : undefined}
      class="docx-run cursor-text min-w-0 resize-none bg-transparent leading-[inherit] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-teal-600"
      style={{ ...runStyle(props.run), 'text-align': props.align }}
      classList={{ 'docx-run-wide': props.fullWidth, 'outline-red-500': Boolean(error()) }}
      onKeyDown={(event) => {
        if (event.isComposing || event.shiftKey || event.altKey || event.metaKey || event.ctrlKey) return;
        const field = event.currentTarget;
        const down = event.key === 'ArrowDown' && field.selectionStart === field.value.length;
        const up = event.key === 'ArrowUp' && field.selectionStart === 0;
        if ((!down && !up) || field.selectionStart !== field.selectionEnd) return;
        const fields = [...field.closest('article')?.querySelectorAll('textarea') ?? []];
        const next = fields[fields.indexOf(field) + (down ? 1 : -1)];
        if (!next) return;
        event.preventDefault();
        next.focus();
        const position = down ? 0 : next.value.length;
        next.setSelectionRange(position, position);
      }}
      onInput={(event) => { const text = event.currentTarget.value; setError(validateRunText(text)); props.onEdit(text); }}/>
    <Show when={error()}><span id={descriptionId} class="w-full py-1 text-xs leading-5 text-red-700">{error()}</span></Show>
  </>;
}
