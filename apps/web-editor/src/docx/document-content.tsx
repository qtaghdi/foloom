import { TabText, layoutTabCharacters } from './tab-layout';
import { t, localizeMessage, source } from '../i18n';
import { For, Show, createEffect, onMount, onCleanup, createMemo, createSignal } from 'solid-js';
import type { PasteMode, ClipboardParagraph } from './clipboard';
import { continuousInput } from './continuous-input';
import type { TextSelection } from './selection';
import { validateRunText } from './validation';
import type { ActivityEntry } from '../ui/activity';
import type { Snapshot, Run, Paragraph, TextOperation, StructureAction } from './contracts';

export function DocumentContent(props: { onListLevel?: (selection:TextSelection,delta:number)=>boolean; onHistory: (direction: 'undo' | 'redo') => void; pasteMode: PasteMode; onPasteRich: (selection: TextSelection, paragraphs: ClipboardParagraph[]) => boolean; richSelection: (selection: TextSelection) => ClipboardParagraph[] | undefined; snapshot: Snapshot; saving: boolean; onRange: (selection: TextSelection, text: string, inline?: boolean) => boolean; onDelete: (selection: TextSelection, direction: 'backward' | 'forward', word: boolean) => boolean; onComposition: (active: boolean) => void; onRefresh: (selection: TextSelection) => void; selectionText: (selection: TextSelection) => string | undefined; onEdit: (operation: TextOperation, selection?: TextSelection) => void; onStructure: (action: StructureAction, runId: string, start: number, end: number, insertion?: string) => boolean; changes: ReadonlyMap<string, ActivityEntry>; selected?: ActivityEntry; onChangeSelect: (entry: ActivityEntry) => void; onFocusRun: (run: Run) => void }) {
  let editor!: HTMLDivElement;
  const [notice, setNotice] = createSignal('');
  let frame=0;
  const scheduleTabs=()=>{cancelAnimationFrame(frame);frame=requestAnimationFrame(()=>{if(editor&&!editor.matches(':active'))layoutTabCharacters(editor);});};
  createEffect(()=>{props.snapshot;queueMicrotask(scheduleTabs);});
  onMount(()=>{if(typeof ResizeObserver==='undefined')return;const observer=new ResizeObserver(scheduleTabs);observer.observe(editor);onCleanup(()=>observer.disconnect());});
  onCleanup(()=>cancelAnimationFrame(frame));
  const input = continuousInput(() => editor, {
    history: direction => props.onHistory(direction), snapshot: () => props.snapshot, saving: () => props.saving, edit: (operation, selection) => props.onEdit(operation, selection),
    listLevel:(selection,delta)=>props.onListLevel?.(selection,delta)??false,
    replace: (...args) => props.onRange(...args), delete: (...args) => props.onDelete(...args), structure: (...args) => props.onStructure(...args),
    composition: active => props.onComposition(active), refresh: selection => props.onRefresh(selection), text: selection => props.selectionText(selection),
    focus: run => props.onFocusRun(run), pasteMode: () => props.pasteMode, pasteRich: (...args) => props.onPasteRich(...args), richText: selection => props.richSelection(selection), notice: message => setNotice(message === 'protected' ? source('paragraphStructureUnavailable') : message === 'invalid' ? source('thisInputContainsLineBreaksTabsOrUnsupported') : message === 'clipboard' ? source('clipboardNormalized') : ''),
  });
  const positions = createMemo(() => {
    const result = new Map<string, number>();
    for (const block of props.snapshot.blocks) {
      const paragraphs = block.type === 'paragraph' ? [block.paragraph] : block.rows.flatMap((row) => row.cells.flatMap((cell) => cell.paragraphs));
      for (const paragraph of paragraphs) for (const run of paragraph.runs) result.set(run.id, result.size + 1);
    }
    return result;
  });
  const pages = createMemo(() => {
    const pages: Array<Array<{ block: Snapshot['blocks'][number]; index: number }>> = [[]];
    let nextPage = false;
    props.snapshot.blocks.forEach((block, index) => {
      const paragraph = block.type === 'paragraph' ? block.paragraph : undefined;
      if ((nextPage || paragraph?.pageBreakBefore) && pages.at(-1)!.length) pages.push([]);
      pages.at(-1)!.push({ block, index });
      nextPage = Boolean(paragraph?.pageBreakAfter);
    });
    return pages;
  });
  const paragraph = (p: Paragraph, kind: TextOperation['type'], key: string) => <section
    data-docx-tab-stops={JSON.stringify(p.layout?.tabs??[])} contentEditable={p.structureEditable ? undefined : false} class="docx-paragraph relative min-h-[1.35em] min-w-0" id={`docx-paragraph-${key}`} tabindex={-1}
    style={{
      'margin-left': p.layout?.left===undefined?p.listLabel?`${36+(p.list?.level??0)*18}pt`:undefined:`${p.layout.left/20}pt`,
      'margin-right': p.layout?.right===undefined?undefined:`${p.layout.right/20}pt`,
      'text-indent': p.layout?.hanging!==undefined?`${-p.layout.hanging/20}pt`:p.layout?.firstLine===undefined?p.listLabel?'-18pt':undefined:`${p.layout.firstLine/20}pt`,
      'line-height': p.layout?.line===undefined?undefined:p.layout.lineRule==='exact'?`${p.layout.line/20}pt`:p.layout.lineRule==='atLeast'?`max(1.35em, ${p.layout.line/20}pt)`:String(p.layout.line/240),
      direction: p.layout?.bidi?'rtl':undefined,
      'writing-mode': p.layout?.textDirection==='tbRl'?'vertical-rl':p.layout?.textDirection==='btLr'?'vertical-lr':undefined,
      'margin-block-start': p.layout?.before === undefined ? undefined : `${p.layout.before / 20}pt`,
      'margin-block-end': p.layout?.after === undefined ? undefined : `${p.layout.after / 20}pt`,
    }}>
    <For each={(p.layout?.tabs??[]).filter(tab=>tab.align==='bar')}>{tab=><span contentEditable={false} aria-hidden="true" class="pointer-events-none absolute top-0 bottom-0 border-l border-slate-500" style={{left:`calc(${tab.position/20}pt - ${p.layout?.left===undefined?p.listLabel?36+(p.list?.level??0)*18:0:p.layout.left/20}pt)`}}/>}</For>
    <div class="min-w-0" classList={{ 'pr-12': props.changes.has(key) }} style={{ 'text-align': p.layout?.align }}>
    <Show when={p.listLabel}><span data-docx-list-marker contentEditable={false} aria-label={`목록 ${p.listLabel}`} class="inline-block min-w-[18pt] select-none" style={{'text-indent':'0'}}>{p.listLabel}</span></Show>
      <Show when={p.runs.length} fallback={<span aria-hidden="true">&nbsp;</span>}>
        <For each={p.runs}>{(run) => <Show when={run.editable} fallback={<span data-docx-text class="whitespace-pre-wrap" style={runStyle(run)} title={t('readOnlyPreservingTheOriginalStructure')}>{run.text || t('unrenderedStructure')}</span>}>
          <Show when={p.structureEditable} fallback={<RunField run={run} fullWidth={p.runs.length === 1} align={p.layout?.align}
            label={t('runLabel', { kind: kind === 'replace_cell_text' ? t('table') : t('body'), count: positions().get(run.id)! })} saving={props.saving}
            onFocus={() => props.onFocusRun(run)} onComposition={props.onComposition} onHistory={props.onHistory}
            onStructure={p.structureEditable ? (action, start, end, insertion) => props.onStructure(action, run.id, start, end, insertion) : undefined}
            onEdit={(text) => props.onEdit({ type: kind, targetId: run.id, expectedText: run.text, text })}/> }>
            <span data-docx-run={run.id} class="cursor-text whitespace-pre-wrap break-words" style={runStyle(run)}>{run.text ? <TabText text={run.text}/> : (p.runs.every(r => !r.text) && p.runs[0] === run ? <br data-docx-empty/> : '')}</span>
          </Show>
        </Show>}</For>
      </Show>
    </div>
    <Show when={props.changes.get(key)}>{(entry) => <button contentEditable={false} type="button" class="editor-action change-marker" aria-label={t('viewAI', { location: localizeMessage(entry().location ?? '') })} aria-expanded={props.selected?.documentChange?.key === key}
      onClick={() => props.onChangeSelect(entry())}>AI</button>}</Show>
    <Show when={props.selected?.documentChange?.key === key ? props.selected : undefined}>{(entry) =>
      <div contentEditable={false} class="mt-3 space-y-2 rounded-md border border-teal-200 bg-teal-50 p-3 text-sm leading-6 text-stone-800" role="region" aria-label={t('aiChangesBeforeAndAfter')}>
        <p class="font-semibold text-teal-900">{t('aiRevision', { revision: entry().revision! })}</p>
        <p class="break-words whitespace-pre-wrap"><span class="mr-2 font-medium">{t('before')}</span><del>{entry().documentChange?.before || t('emptyText')}</del></p>
        <p class="break-words whitespace-pre-wrap"><span class="mr-2 font-medium">{t('after')}</span>{entry().documentChange?.after || t('emptyText')}</p>
      </div>}
    </Show>
  </section>;
  return <>
    <Show when={notice()}><p class="text-sm text-red-800" role="status">{localizeMessage(notice())}</p></Show>
    <div ref={editor} data-continuous-editor role="textbox" aria-multiline="true" aria-readonly={props.saving} aria-label={t('docxBodyAndTableEditor')}
      contentEditable={!props.saving} tabindex={0} class="space-y-5 cursor-text outline-none focus-visible:ring-2 focus-visible:ring-teal-700"
      onKeyDown={input.keydown} onKeyUp={input.focus} onPointerUp={input.focus} onFocusIn={input.focus}
      onBeforeInput={input.beforeinput} onInput={event=>{input.input(event);scheduleTabs();}} onCompositionStart={input.compositionstart} onCompositionEnd={input.compositionend}
      onPaste={input.paste} onCopy={input.copy} onCut={input.cut} onDrop={input.drop}>
    <For each={pages()}>{(page) => <div class="docx-paper w-full border border-slate-200 bg-white shadow-sm">
    <Show when={page.length} fallback={<p>{t('noBodyTextCanBeDisplayedTheOriginal')}</p>}>
    <For each={page}>{({ block, index }) => block.type === 'paragraph' ? paragraph(block.paragraph, 'replace_run_text', `${index}`) :
      <div class="docx-table overflow-x-auto" tabindex={0} aria-label={t('documentTableScrollHorizontallyOnNarrowScreens')}>
        <table class="w-full border-collapse text-left"><tbody><For each={block.rows}>{(row, rowIndex) => <tr><For each={row.cells}>{(cell, cellIndex) =>
          <td class="min-w-24 border border-slate-300 px-2 py-1 align-top"><For each={cell.paragraphs}>{(p, pIndex) => paragraph(p, 'replace_cell_text', `${index}-${rowIndex()}-${cellIndex()}-${pIndex()}`)}</For></td>
        }</For></tr>}</For></tbody></table>
      </div>
    }</For>
    </Show>
  </div>}</For></div></>;
}

function runStyle(run: Run) {
  return {
    'font-size': run.sizeHalfPoints === undefined ? undefined : `${run.sizeHalfPoints / 2}pt`,
    'font-family': run.font ? `${JSON.stringify(run.font)}, system-ui, sans-serif` : undefined,
    color: run.color,
    'text-decoration': [run.underline?'underline':'',run.strike?'line-through':''].filter(Boolean).join(' ') || undefined,
    'background-color': ({yellow:'#ffff00',green:'#00ff00',cyan:'#00ffff',magenta:'#ff00ff',blue:'#0000ff',red:'#ff0000',darkBlue:'#000080',darkCyan:'#008080',darkGreen:'#008000',darkMagenta:'#800080',darkRed:'#800000',darkYellow:'#808000',darkGray:'#808080',lightGray:'#c0c0c0',black:'#000000',white:'#ffffff'} as Record<string,string>)[run.highlight??''],
    'vertical-align': run.vertical==='superscript'?'super':run.vertical==='subscript'?'sub':undefined,
    'font-weight': run.bold ? '700' : undefined,
    'font-style': run.italic ? 'italic' : undefined,
  };
}

function RunField(props: { onComposition: (active: boolean) => void; onHistory: (direction: 'undo' | 'redo') => void; run: Run; fullWidth: boolean; align?: 'start' | 'center' | 'end' | 'justify'; label: string; saving: boolean; onFocus: () => void; onStructure?: (action: StructureAction, start: number, end: number, insertion?: string) => boolean; onEdit: (text: string) => void }) {
  const [error, setError] = createSignal('');
  const [hint, setHint] = createSignal('');
  const [composing, setComposing] = createSignal(false);
  let softBreak = false;
  const descriptionId = `input-error-${props.run.id.replace(/[^a-zA-Z0-9_-]/g, '-')}`;
  return <>
    <textarea value={props.run.text} rows={Math.max(1, Math.ceil(props.run.text.length / 72))} readOnly={props.saving}
      title={props.onStructure ? undefined : t('paragraphStructureUnavailable')} placeholder={props.run.text ? undefined : t('typeHere')} name={props.run.id} autocomplete="off" aria-label={props.label} aria-invalid={Boolean(error())} aria-describedby={error() || hint() ? descriptionId : undefined}
      class="docx-run cursor-text min-w-0 resize-none bg-transparent leading-[inherit]"
      style={{ ...runStyle(props.run), 'text-align': props.align }}
      classList={{ 'docx-run-wide': props.fullWidth }}
      onFocus={props.onFocus}
      onCompositionStart={() => { setComposing(true); props.onComposition(true); }} onCompositionEnd={event => { setComposing(false); props.onEdit(event.currentTarget.value); props.onComposition(false); }}
      onKeyDown={(event) => {
        if (event.isComposing || composing() || props.saving) return;
        const field = event.currentTarget;
        softBreak = event.key === 'Enter' && event.shiftKey;
        const action: StructureAction | undefined = event.altKey && event.shiftKey && !event.ctrlKey && !event.metaKey
          ? (event.key === 'ArrowUp' ? 'moveUp' : event.key === 'ArrowDown' ? 'moveDown' : undefined)
          : !event.shiftKey && !event.altKey && !event.metaKey && !event.ctrlKey
            ? event.key === 'Enter' ? 'split' : event.key === 'Backspace' ? 'mergeBackward' : event.key === 'Delete' ? 'mergeForward' : undefined : undefined;
        if (action && props.onStructure?.(action, field.selectionStart, field.selectionEnd)) { event.preventDefault(); return; }
        if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); setHint(source('paragraphStructureUnavailable')); return; }
        if (event.shiftKey || event.altKey || event.metaKey || event.ctrlKey) return;
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
      onBeforeInput={(event) => {
        if (composing() || event.isComposing || props.saving || !event.cancelable) return;
        if (event.inputType === 'historyUndo' || event.inputType === 'historyRedo') { event.preventDefault(); props.onHistory(event.inputType === 'historyUndo' ? 'undo' : 'redo'); return; }
        const lineBreak = event.inputType === 'insertParagraph' || event.inputType === 'insertLineBreak' && !softBreak;
        softBreak = false;
        if (!lineBreak) return;
        const field = event.currentTarget;
        if (props.onStructure?.('split', field.selectionStart, field.selectionEnd)) event.preventDefault();
        else { event.preventDefault(); setHint(source('paragraphStructureUnavailable')); }
      }}
      onPaste={(event) => {
        const text = event.clipboardData?.getData('text/plain');
        if (!text || !/[\r\n]/.test(text) || props.saving) return;
        const field = event.currentTarget;
        if (props.onStructure?.('split', field.selectionStart, field.selectionEnd, text)) event.preventDefault();
      }}
      onInput={(event) => { const text = event.currentTarget.value; setHint(''); setError(validateRunText(text)); props.onEdit(text); }}/>
    <Show when={error() || hint()}><span id={descriptionId} class="w-full py-1 text-xs leading-5 text-red-700">{localizeMessage(error() || hint())}</span></Show>
  </>;
}
