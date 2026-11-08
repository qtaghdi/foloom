import { resolveHeaderText } from './document-header-footer';
import { t, localizeMessage, source } from '../i18n';
import { For, Show, createMemo, createSignal } from 'solid-js';
import type { PasteMode, ClipboardParagraph } from './clipboard';
import { continuousInput } from './continuous-input';
import type { TextSelection } from './selection';
import { validateRunText } from './validation';
import type { ActivityEntry } from '../ui/activity';
import { ObjectPreview } from './document-objects';
import { scopedDocumentURL } from '../transport/document-scope';
import type { Snapshot, Run, Paragraph, TextOperation, StructureAction } from './contracts';

export function DocumentContent(props: { onHistory: (direction: 'undo' | 'redo') => void; pasteMode: PasteMode; onPasteRich: (selection: TextSelection, paragraphs: ClipboardParagraph[]) => boolean; richSelection: (selection: TextSelection) => ClipboardParagraph[] | undefined; snapshot: Snapshot; saving: boolean; onRange: (selection: TextSelection, text: string, inline?: boolean) => boolean; onDelete: (selection: TextSelection, direction: 'backward' | 'forward', word: boolean) => boolean; onComposition: (active: boolean) => void; onRefresh: (selection: TextSelection) => void; selectionText: (selection: TextSelection) => string | undefined; onEdit: (operation: TextOperation, selection?: TextSelection) => void; onStructure: (action: StructureAction, runId: string, start: number, end: number, insertion?: string) => boolean; changes: ReadonlyMap<string, ActivityEntry>; selected?: ActivityEntry; onChangeSelect: (entry: ActivityEntry) => void; onFocusRun: (run: Run) => void }) {
  let editor!: HTMLDivElement;
  const [notice, setNotice] = createSignal('');
  const input = continuousInput(() => editor, {
    history: direction => props.onHistory(direction), snapshot: () => props.snapshot, saving: () => props.saving, edit: (operation, selection) => props.onEdit(operation, selection),
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
    let nextPage = false,previousSection:string|undefined;
    props.snapshot.blocks.forEach((block, index) => {
      const paragraph = block.type === 'paragraph' ? block.paragraph : undefined;
      const section=paragraph?.sectionId??(block.type==='table'?block.rows[0]?.cells[0]?.paragraphs[0]?.sectionId:undefined);
      if ((nextPage || paragraph?.pageBreakBefore || section!==previousSection) && pages.at(-1)!.length) pages.push([]);
      pages.at(-1)!.push({ block, index });
      nextPage = Boolean(paragraph?.pageBreakAfter);previousSection=section;
    });
    return pages;
  });
  const pageSection=(page:ReturnType<typeof pages>[number])=>{const block=page[0]?.block;const id=block?.type==='paragraph'?block.paragraph.sectionId:block?.rows[0]?.cells[0]?.paragraphs[0]?.sectionId;return props.snapshot.sections?.find(section=>section.id===id);};
  const sectionStyle=(page:ReturnType<typeof pages>[number])=>{
   const info=pageSection(page);if(!info)return {};
   const p=info.page,width=`min(100cqi, ${p.width/15}px)`;
   return {width:`${p.width/15}px`,'max-width':'100%','min-block-size':`calc(${width} * ${p.height/p.width})`,'--paper-insets':[p.top,p.right,p.bottom,p.left].map(value=>`calc(${width} * ${value/p.width})`).join(' ')};
  };
  const pageOrdinals=createMemo(()=>{
   let previous:string|undefined, ordinal=0;
   return pages().map(page=>{
    const section=pageSection(page)?.id;
    ordinal=section===previous?ordinal+1:1;
    previous=section;
    return ordinal;
   });
  });
  const headerText=(page:ReturnType<typeof pages>[number],index:number,kind:'header'|'footer')=>{
   const section=pageSection(page),ordinal=pageOrdinals()[index]??1;
   const variant=ordinal===1&&section?.firstPage?'first':props.snapshot.evenAndOddHeaders&&ordinal%2===0?'even':'default';
   return resolveHeaderText(props.snapshot.sections??[],section?.headerFooters?.find(slot=>slot.kind===kind&&slot.variant===variant));
  };
  const goToBookmark=(name:string)=>{
    const id=props.snapshot.bookmarks?.find(bookmark=>bookmark.name===name)?.paragraphId;
    if(!id)return;
    const target=[...editor.querySelectorAll<HTMLElement>('[data-docx-paragraph]')].find(node=>node.dataset.docxParagraph===id);
    target?.focus({preventScroll:true});
    target?.scrollIntoView({block:'center'});
  };
  const paragraph = (p: Paragraph, kind: TextOperation['type'], key: string) => <section
    contentEditable={p.structureEditable ? undefined : false} class="docx-paragraph relative min-h-[1.35em] min-w-0" id={`docx-paragraph-${key}`} data-docx-paragraph={p.id} tabindex={-1}
    style={{
      'margin-left': p.layout?.left===undefined?undefined:`${p.layout.left/20}pt`,
      'margin-right': p.layout?.right===undefined?undefined:`${p.layout.right/20}pt`,
      'text-indent': p.layout?.hanging!==undefined?`${-p.layout.hanging/20}pt`:p.layout?.firstLine===undefined?undefined:`${p.layout.firstLine/20}pt`,
      'line-height': p.layout?.line===undefined?undefined:p.layout.lineRule==='exact'?`${p.layout.line/20}pt`:p.layout.lineRule==='atLeast'?`max(1.35em, ${p.layout.line/20}pt)`:String(p.layout.line/240),
      direction: p.layout?.bidi?'rtl':undefined,
      'writing-mode': p.layout?.textDirection==='tbRl'?'vertical-rl':p.layout?.textDirection==='btLr'?'vertical-lr':undefined,
      'margin-block-start': p.layout?.before === undefined ? undefined : `${p.layout.before / 20}pt`,
      'margin-block-end': p.layout?.after === undefined ? undefined : `${p.layout.after / 20}pt`,
    }}>
    <div class="min-w-0" classList={{ 'pr-12': props.changes.has(key) }} style={{ 'text-align': p.layout?.align }}>
      <Show when={p.runs.length} fallback={<span aria-hidden="true">&nbsp;</span>}>
        <For each={p.runs}>{(run) => <Show when={run.editable} fallback={<Show when={run.picture?.preview} fallback={<Show when={run.link} fallback={<Show when={run.object} fallback={<span data-docx-text class="whitespace-pre-wrap" style={runStyle(run)} title={t('readOnlyPreservingTheOriginalStructure')}>{run.text || t('unrenderedStructure')}</span>}>{object=><ObjectPreview object={object()}/>}</Show>}>{link=>
          <a data-docx-text contentEditable={false} class="whitespace-pre-wrap text-blue-800 underline focus-visible:outline-2 focus-visible:outline-teal-700"
            href={link().url??`#${link().bookmark}`} target={link().url?'_blank':undefined}
            rel={link().url?'noopener noreferrer':undefined}
            onClick={event=>{const name=link().bookmark;if(name){event.preventDefault();goToBookmark(name);}}}>{run.text}</a>
        }</Show>}>
          <img loading="lazy" class="inline-block max-w-full align-middle" contentEditable={false}
            src={scopedDocumentURL(`/api/docx/picture?id=${encodeURIComponent(run.id)}`)} alt={run.picture!.alt}
            width={Math.max(1,Math.round(run.picture!.widthEmu/9525))} height={Math.max(1,Math.round(run.picture!.heightEmu/9525))}
            style={{height:'auto'}}/>
        </Show>}>
          <Show when={p.structureEditable} fallback={<RunField run={run} fullWidth={p.runs.length === 1} align={p.layout?.align}
            label={t('runLabel', { kind: kind === 'replace_cell_text' ? t('table') : t('body'), count: positions().get(run.id)! })} saving={props.saving}
            onFocus={() => props.onFocusRun(run)} onComposition={props.onComposition} onHistory={props.onHistory}
            onStructure={p.structureEditable ? (action, start, end, insertion) => props.onStructure(action, run.id, start, end, insertion) : undefined}
            onEdit={(text) => props.onEdit({ type: kind, targetId: run.id, expectedText: run.text, text })}/> }>
            <span data-docx-run={run.id} class="cursor-text whitespace-pre-wrap break-words" style={runStyle(run)}>{run.text || (p.runs.every(r => !r.text) && p.runs[0] === run ? <br data-docx-empty/> : '')}</span>
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
      onBeforeInput={input.beforeinput} onInput={input.input} onCompositionStart={input.compositionstart} onCompositionEnd={input.compositionend}
      onPaste={input.paste} onCopy={input.copy} onCut={input.cut} onDrop={input.drop}>
    <For each={pages()}>{(page,index) => <div class="docx-paper mx-auto w-full border border-slate-200 bg-white shadow-sm" style={sectionStyle(page)} data-docx-section={pageSection(page)?.id}>
    <Show when={headerText(page,index(),'header')}>{text=><div contentEditable={false} aria-label="머리글 미리보기" class="mb-4 whitespace-pre-wrap border-b border-dashed border-slate-200 pb-2 text-xs text-stone-600">{text()}</div>}</Show>
    <Show when={page.length} fallback={<p>{t('noBodyTextCanBeDisplayedTheOriginal')}</p>}>
    <Show when={pageSection(page)}>{section=><p contentEditable={false} class="mb-3 select-none text-xs text-stone-500">구역 {(props.snapshot.sections?.findIndex(s=>s.id===section().id)??0)+1} · {section().orientation==='landscape'?'가로 용지':'세로 용지'} · {section().columns}단</p>}</Show>
    <div style={{'column-count':pageSection(page)?.columns,'column-gap':pageSection(page)?`${pageSection(page)!.columnSpace/20}pt`:undefined}}>
    <For each={page}>{({ block, index }) => block.type === 'paragraph' ? paragraph(block.paragraph, 'replace_run_text', `${index}`) :
      <div class="docx-table overflow-x-auto" tabindex={0} aria-label={t('documentTableScrollHorizontallyOnNarrowScreens')}>
        <table class="w-full border-collapse text-left"><tbody><For each={block.rows}>{(row, rowIndex) => <tr><For each={row.cells}>{(cell, cellIndex) =>
          <td class="min-w-24 border border-slate-300 px-2 py-1 align-top"><For each={cell.paragraphs}>{(p, pIndex) => paragraph(p, 'replace_cell_text', `${index}-${rowIndex()}-${cellIndex()}-${pIndex()}`)}</For></td>
        }</For></tr>}</For></tbody></table>
      </div>
    }</For>
    </div>
    </Show>
    <Show when={headerText(page,index(),'footer')}>{text=><div contentEditable={false} aria-label="바닥글 미리보기" class="mt-4 whitespace-pre-wrap border-t border-dashed border-slate-200 pt-2 text-xs text-stone-600">{text()}</div>}</Show>
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
