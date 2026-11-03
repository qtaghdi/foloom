import { For, Show, createMemo } from 'solid-js';
import type { Snapshot, Run, Operation } from './contracts';

export function DocumentContent(props: { snapshot: Snapshot; saving: boolean; onEdit: (operation: Operation) => void }) {
  const positions = createMemo(() => {
    const result = new Map<string, number>();
    for (const block of props.snapshot.blocks) {
      const paragraphs = block.type === 'paragraph' ? [block.paragraph] : block.rows.flatMap((row) => row.cells.flatMap((cell) => cell.paragraphs));
      for (const paragraph of paragraphs) for (const run of paragraph.runs) result.set(run.id, result.size + 1);
    }
    return result;
  });
  const paragraph = (runs: Run[], kind: Operation['type']) => <div class="docx-paragraph flex flex-wrap items-baseline gap-x-1 gap-y-0">
    <Show when={runs.length} fallback={<span class="text-sm text-slate-400">빈 문단</span>}>
      <For each={runs}>{(run) => <Show when={run.editable} fallback={<span class="whitespace-pre-wrap text-slate-700" title="읽기 전용: 원본 구조를 보존합니다.">{run.text || '표시되지 않는 구조'}</span>}>
        <textarea value={run.text} rows={Math.max(1, Math.ceil(run.text.length / 72))} disabled={props.saving}
          class="docx-run min-h-11 min-w-0 resize-y rounded-sm border border-transparent bg-transparent px-1 py-1 leading-8 hover:border-slate-300 focus-visible:border-indigo-400 focus-visible:outline-2 focus-visible:outline-indigo-200 disabled:opacity-60"
          classList={{ 'font-bold': run.bold, italic: run.italic }}
          aria-label={`${kind === 'replace_cell_text' ? '표' : '본문'} 텍스트 ${positions().get(run.id)}`}
          onInput={(event) => props.onEdit({ type: kind, targetId: run.id, expectedText: run.text, text: event.currentTarget.value })}/>
      </Show>}</For>
    </Show>
  </div>;
  return <Show when={props.snapshot.blocks.length} fallback={<p>표시할 수 있는 본문이 없습니다. 원본 문서는 보존됩니다.</p>}>
    <For each={props.snapshot.blocks}>{(block) => block.type === 'paragraph' ? paragraph(block.paragraph.runs, 'replace_run_text') :
      <div class="overflow-x-auto border border-slate-300" tabindex={0} aria-label="문서 표. 좁은 화면에서는 좌우로 스크롤하세요.">
        <table class="w-full border-collapse text-left"><tbody><For each={block.rows}>{(row) => <tr><For each={row.cells}>{(cell) =>
          <td class="min-w-32 border border-slate-300 px-3 py-2 align-top"><For each={cell.paragraphs}>{(p) => paragraph(p.runs, 'replace_cell_text')}</For></td>
        }</For></tr>}</For></tbody></table>
      </div>
    }</For>
  </Show>;
}
