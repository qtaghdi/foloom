import { runFont, splitRunFonts } from './run-fonts';
import { TabText, layoutTabCharacters } from './tab-layout';
import { resolveHeaderText } from './document-header-footer';
import { defaultHeaderFooterDistance, previewPageLayout, sectionPaper, type PreviewPageLayout } from './sections';
import { t, localizeMessage, source } from '../i18n';
import { For, Show, createEffect, onMount, onCleanup, createMemo, createSignal } from 'solid-js';
import type { PasteMode, ClipboardParagraph } from './clipboard';
import { continuousInput } from './continuous-input';
import type { TextSelection } from './selection';
import { validateRunText } from './validation';
import type { ActivityEntry } from '../ui/activity';
import { printPaperFor } from './print-layout';
import { ObjectPreview } from './document-objects';
import { scopedDocumentURL } from '../transport/document-scope';
import type { Snapshot, Run, Paragraph, TextOperation, StructureAction, SectionInfo } from './contracts';

type PageGroup = { blocks: Array<{block:Snapshot['blocks'][number];index:number}>; sectionId?:string };

export function previewPages(blocks:Snapshot['blocks']): PageGroup[] {
  let page:PageGroup = {blocks:[]};
  const pages = [page];
  let nextPage = false;
  blocks.forEach((block,index)=>{
    const paragraph = block.type==='paragraph' ? block.paragraph : undefined;
    const sectionId = block.type==='table' ? block.rows[0]?.cells[0]?.paragraphs[0]?.sectionId : paragraph?.sectionId;
    if ((nextPage || paragraph?.pageBreakBefore || sectionId !== page.sectionId) && page.blocks.length) {
      page = {blocks:[],sectionId};
      pages.push(page);
    } else if (!page.blocks.length) {
      page.sectionId = sectionId;
    }
    page.blocks.push({block,index});
    nextPage = Boolean(paragraph?.pageBreakAfter);
  });
  return pages;
}

export function layoutPreviewPages(pages:PageGroup[], sections:SectionInfo[]|undefined, evenAndOddHeaders?:boolean): PreviewPageLayout[] {
  const byId = new Map(sections?.map(section=>[section.id,section]));
  return previewPageLayout(pages.map(page=>page.sectionId===undefined?undefined:byId.get(page.sectionId)),evenAndOddHeaders);
}

export function DocumentContent(props: { pages: PageGroup[]; layout: PreviewPageLayout[]; onListLevel?: (selection:TextSelection,delta:number)=>boolean; onHistory: (direction: 'undo' | 'redo') => void; pasteMode: PasteMode; onPasteRich: (selection: TextSelection, paragraphs: ClipboardParagraph[]) => boolean; richSelection: (selection: TextSelection) => ClipboardParagraph[] | undefined; snapshot: Snapshot; saving: boolean; onRange: (selection: TextSelection, text: string, inline?: boolean) => boolean; onDelete: (selection: TextSelection, direction: 'backward' | 'forward', word: boolean) => boolean; onComposition: (active: boolean) => void; onRefresh: (selection: TextSelection) => void; selectionText: (selection: TextSelection) => string | undefined; onEdit: (operation: TextOperation, selection?: TextSelection) => void; onSplitNote: (run:Run, offset:number) => boolean; onStructure: (action: StructureAction, runId: string, start: number, end: number, insertion?: string) => boolean; changes: ReadonlyMap<string, ActivityEntry>; selected?: ActivityEntry; onChangeSelect: (entry: ActivityEntry) => void; onFocusRun: (run: Run) => void }) {
  let editor!: HTMLDivElement;
  const [notice, setNotice] = createSignal('');
  let frame=0;
  const scheduleTabs=()=>{cancelAnimationFrame(frame);frame=requestAnimationFrame(()=>{if(editor&&!editor.matches(':active'))layoutTabCharacters(editor, props.snapshot.defaultTabStop);});};
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
  const sectionStyle=(info?:SectionInfo)=>{
   if(!info)return {'--docx-print-paper':printPaperFor(props.snapshot)};
   const p=sectionPaper(info),width=`min(100cqi, ${p.width/15}px)`;
   const scaled=(twips:number)=>`calc(${width} * ${twips/p.width})`;
   return {
    '--docx-print-paper':printPaperFor(props.snapshot,info),
    width:`${p.width/15}px`,'max-width':'100%','min-block-size':scaled(p.height),
    '--paper-insets':[0,p.right,0,p.left].map(scaled).join(' '),
    '--docx-body-top':scaled(p.top),'--docx-body-bottom':scaled(p.bottom),
    '--docx-header-distance':scaled(info.headerDistance??defaultHeaderFooterDistance),
    '--docx-footer-distance':scaled(info.footerDistance??defaultHeaderFooterDistance),
   };
  };
  const headerText=(layout:PreviewPageLayout|undefined,kind:'header'|'footer')=>{
   const section=layout?.section,variant=layout?.variant;
   const slot=section?.headerFooters?.find(slot=>slot.kind===kind&&slot.variant===variant);
   const text=resolveHeaderText(props.snapshot.sections??[],slot);
   const fields=slot?.compactPageNumbers?['쪽 번호 / 전체 쪽 수']:[
    slot?.pageNumber?'쪽 번호 필드':'',slot?.totalPages?'전체 쪽 수 필드':'',
   ];
   return [text,...fields].filter(Boolean).join('\n');
  };
  const goToBookmark=(name:string)=>{
    const id=props.snapshot.bookmarks?.find(bookmark=>bookmark.name===name)?.paragraphId;
    if(!id)return;
    const target=[...editor.querySelectorAll<HTMLElement>('[data-docx-paragraph]')].find(node=>node.dataset.docxParagraph===id);
    target?.focus({preventScroll:true});
    target?.scrollIntoView({block:'center'});
  };
  const paragraph = (p: Paragraph, kind: TextOperation['type'], key: string) => <section
    data-docx-tab-stops={JSON.stringify(p.layout?.tabs??[])} contentEditable={p.structureEditable ? undefined : false} class="docx-paragraph relative min-h-[1.35em] min-w-0" id={`docx-paragraph-${key}`} data-docx-paragraph={p.id} tabindex={-1}
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
    <div class="min-w-0" classList={{ 'pr-12': props.changes.has(key) }} style={{ 'text-align': paragraphAlignment(p) }}>
    <Show when={p.listLabel}><span data-docx-list-marker contentEditable={false} aria-label={`목록 ${p.listLabel}`} class="inline-block min-w-[18pt] select-none" style={{'text-indent':'0'}}>{p.listLabel}</span></Show>
      <Show when={p.runs.length} fallback={<span aria-hidden="true">&nbsp;</span>}>
        <For each={p.runs}>{(run) => <Show when={run.editable} fallback={<Show when={run.picture?.preview} fallback={<Show when={run.link} fallback={<Show when={run.object} fallback={<span data-docx-text class="whitespace-pre-wrap" style={runStyle(run)} title={t('readOnlyPreservingTheOriginalStructure')}>{run.text || t('unrenderedStructure')}</span>}>{object=>
          object().kind==='tracked_format'
            ? <span data-docx-text class="whitespace-pre-wrap" style={runStyle(run)} title="검토할 서식 변경">{run.text}</span>
            : <ObjectPreview object={object()}/>
        }</Show>}>{link=>{
          const pendingPageReference=()=>{
            const current=link();
            return 'pageReference' in current && current.pageReference && run.text==='?';
          };
          return <a data-docx-text contentEditable={false} class="whitespace-pre-wrap text-blue-800 underline focus-visible:outline-2 focus-visible:outline-teal-700"
            href={link().url??`#${link().bookmark}`} target={link().url?'_blank':undefined}
            rel={link().url?'noopener noreferrer':undefined}
            title={pendingPageReference() ? '문서 편집기에서 필드를 업데이트하면 쪽 번호가 표시됩니다.' : undefined}
            onClick={event=>{const name=link().bookmark;if(name){event.preventDefault();goToBookmark(name);}}}>{pendingPageReference() ? '쪽 번호 업데이트 필요' : run.text}</a>;
        }}</Show>}>
          <Show when={run.picture}>{picture=><PicturePreview picture={picture()} id={run.id}/>}</Show>
        </Show>}>
          <Show when={p.structureEditable} fallback={<><span aria-hidden="true" data-docx-print-text class="hidden whitespace-pre-wrap break-words" style={runStyle(run)}>{run.text}</span><RunField run={run} fullWidth={p.runs.length === 1} align={p.layout?.align}
            label={t('runLabel', { kind: kind === 'replace_cell_text' ? t('table') : t('body'), count: positions().get(run.id)! })} saving={props.saving}
            onFocus={() => props.onFocusRun(run)} onComposition={props.onComposition} onHistory={props.onHistory}
            onSplitNote={run.splitNoteParagraph ? offset => props.onSplitNote(run,offset) : undefined}
            onStructure={p.structureEditable ? (action, start, end, insertion) => props.onStructure(action, run.id, start, end, insertion) : undefined}
            onEdit={(text) => props.onEdit({ type: kind, targetId: run.id, expectedText: run.text, text })}/></> }>
            <span data-docx-run={run.id} class="cursor-text whitespace-pre-wrap break-words" style={runStyle(run)}>{run.text ? <RunText run={run}/> : (p.runs.every(r => !r.text) && p.runs[0] === run ? <br data-docx-empty/> : '')}</span>
          </Show>
        </Show>}</For>
      </Show>
    </div>
    <Show when={props.changes.get(key)}>{(entry) => <button contentEditable={false} type="button" class="editor-action change-marker" aria-label={t('viewAI', { location: localizeMessage(entry().location ?? '') })} aria-expanded={props.selected?.documentChange?.key === key}
      onClick={() => props.onChangeSelect(entry())}>AI</button>}</Show>
    <Show when={props.selected?.documentChange?.key === key ? props.selected : undefined}>{(entry) =>
      <div contentEditable={false} data-docx-print-hide class="mt-3 space-y-2 rounded-md border border-teal-200 bg-teal-50 p-3 text-sm leading-6 text-stone-800" role="region" aria-label={t('aiChangesBeforeAndAfter')}>
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
      onKeyDown={input.keydown} onKeyUp={()=>{input.endPasteShortcut();input.focus();}}
      onPointerUp={input.focus} onFocusIn={input.focus} onFocusOut={input.endPasteShortcut}
      onBeforeInput={input.beforeinput} onInput={event=>{input.input(event);scheduleTabs();}} onCompositionStart={input.compositionstart} onCompositionEnd={input.compositionend}
      onPaste={input.paste} onCopy={input.copy} onCut={input.cut} onDrop={input.drop}>
    <For each={props.pages}>{(page,index) => {
      const layout=()=>props.layout[index()];
      const section=()=>layout()?.section;
      const paperStyle=()=>sectionStyle(section());
      return <>
    <Show when={layout()?.blankBefore}>
      <div class="docx-paper mx-auto w-full border border-slate-200 bg-white shadow-sm" style={paperStyle()}
        data-docx-section={section()?.id} data-docx-page={(layout()?.number??0)-1} data-docx-blank-page
        contentEditable={false} role="group" aria-label="구역 시작을 위한 빈 쪽" tabindex={-1}/>
    </Show>
    <div class="docx-paper mx-auto flex w-full flex-col border border-slate-200 bg-white shadow-sm" style={paperStyle()}
      data-docx-section={section()?.id} data-docx-page={layout()?.number??index()+1} tabindex={-1}>
    <div contentEditable={false} class="docx-page-edge min-h-[var(--docx-body-top,0px)]">
      <Show when={headerText(layout(),'header')}>{text=><div aria-label="머리글 미리보기" class="whitespace-pre-wrap border-b border-dashed border-slate-200 pt-[var(--docx-header-distance,0px)] pb-2 text-xs break-words text-stone-600 print:mb-4 print:pt-0">{text()}</div>}</Show>
    </div>
    <Show when={page.blocks.length} fallback={<p>{t('noBodyTextCanBeDisplayedTheOriginal')}</p>}>
    <Show when={section()}>{section=><p contentEditable={false} data-docx-print-hide class="mb-3 select-none text-xs text-stone-500">구역 {(props.snapshot.sections?.findIndex(s=>s.id===section().id)??0)+1} · {section().orientation==='landscape'?'가로 용지':'세로 용지'} · {section().columns}단</p>}</Show>
    <div style={{'column-count':section()?.columns,'column-gap':section()?`${section()!.columnSpace/20}pt`:undefined}}>
    <For each={page.blocks}>{({ block, index }) => block.type === 'paragraph' ? paragraph(block.paragraph, 'replace_run_text', `${index}`) :
      <div class="docx-table overflow-x-auto" tabindex={0} aria-label={t('documentTableScrollHorizontallyOnNarrowScreens')}>
        <table class="w-full border-collapse text-left"><tbody><For each={block.rows}>{(row, rowIndex) => <tr style={{height:row.height ? `${row.height/20}pt` : undefined}}><For each={row.cells}>{(cell, cellIndex) =>
          <td class="min-w-24 border border-slate-300 px-2 py-1 align-top"><For each={cell.paragraphs}>{(p, pIndex) => paragraph(p, 'replace_cell_text', `${index}-${rowIndex()}-${cellIndex()}-${pIndex()}`)}</For></td>
        }</For></tr>}</For></tbody></table>
      </div>
    }</For>
    </div>
    </Show>
    <div contentEditable={false} class="docx-page-edge mt-auto flex min-h-[var(--docx-body-bottom,0px)] flex-col justify-end">
      <Show when={headerText(layout(),'footer')}>{text=><div aria-label="바닥글 미리보기" class="whitespace-pre-wrap border-t border-dashed border-slate-200 pt-2 pb-[var(--docx-footer-distance,0px)] text-xs break-words text-stone-600 print:mt-4 print:pb-0">{text()}</div>}</Show>
    </div>
    </div></>;
    }}</For></div></>;
}

function paragraphAlignment(paragraph:Paragraph) {
  const placement=paragraph.runs.find(run=>run.picture?.placement?.startsWith('floating_'))?.picture?.placement;
  switch(placement){
    case 'floating_left':
    case 'floating_offset':return 'start';
    case 'floating_center':return 'center';
    case 'floating_right':return 'end';
    default:return paragraph.layout?.align;
  }
}

function PicturePreview(props:{picture:NonNullable<Run['picture']>;id:string}) {
  const wrapSide=()=>props.picture.placement==='square_left'?'left':props.picture.placement==='square_right'?'right':undefined;
  const rotation=()=>props.picture.rotation??0;
  const quarterTurn=()=>rotation()===90||rotation()===270;
  const width=()=>quarterTurn()?props.picture.heightEmu:props.picture.widthEmu;
  const height=()=>quarterTurn()?props.picture.widthEmu:props.picture.heightEmu;
  const widthPx=()=>Math.max(1,Math.round(width()/9525));
  const margin=()=>{
    const offset=props.picture.horizontalOffsetEmu;
    if(props.picture.placement!=='floating_offset'||offset===undefined)return undefined;
    return `min(${offset/9525}px, max(0px, calc(100% - ${widthPx()}px)))`;
  };
  const crop=()=>props.picture.crop;
  const wide=()=>100000-(crop()?.left??0)-(crop()?.right??0);
  const high=()=>100000-(crop()?.top??0)-(crop()?.bottom??0);
  return <span data-docx-picture contentEditable={false} class="relative inline-block max-w-full align-middle"
    style={{width:`${widthPx()}px`,'aspect-ratio':`${width()}/${height()}`,float:wrapSide(),
      'max-width':wrapSide()?'50%':undefined,'margin-left':margin(),
      'margin-top':props.picture.placement==='floating_offset'&&props.picture.verticalOffsetEmu?
        `${props.picture.verticalOffsetEmu/9525}px`:undefined}}>
    <span data-docx-crop={crop()?'':undefined} class="absolute left-1/2 top-1/2 overflow-hidden"
      style={{width:quarterTurn()?`${100*height()/width()}%`:'100%',height:quarterTurn()?`${100*width()/height()}%`:'100%',
        transform:`translate(-50%, -50%) rotate(${rotation()}deg)`}}>
      <img loading="lazy" class="absolute" style={{'max-width':'none',
        width:`${10000000/wide()}%`,height:`${10000000/high()}%`,left:`${-100*(crop()?.left??0)/wide()}%`,top:`${-100*(crop()?.top??0)/high()}%`}}
        src={scopedDocumentURL(`/api/docx/picture?id=${encodeURIComponent(props.id)}`)} alt={props.picture.alt}/>
    </span>
  </span>;
}

function RunText(props: {run: Run}) {
  if (!props.run.eastAsiaFont || props.run.eastAsiaFont === props.run.font) return <TabText text={props.run.text}/>;
  // Display segments share the enclosing run's text offsets.
  const base = runFont(props.run);
  return <For each={splitRunFonts(props.run)}>{segment => segment.font && segment.font !== base
    ? <span style={{'font-family': fontFamily(segment.font, base)}}>{segment.text}</span>
    : <TabText text={segment.text}/>}</For>;
}

function fontFamily(font: string, fallback?: string): string {
  return `${JSON.stringify(font)}${fallback ? `, ${JSON.stringify(fallback)}` : ''}, system-ui, sans-serif`;
}

function runStyle(run: Run) {
  const font = runFont(run);
  return {
    'font-size': run.sizeHalfPoints === undefined ? undefined : `${run.sizeHalfPoints / 2}pt`,
    'font-family': font ? fontFamily(font) : undefined,
    color: run.color,
    'text-decoration': [run.underline?'underline':'',run.strike?'line-through':''].filter(Boolean).join(' ') || undefined,
    'background-color': ({yellow:'#ffff00',green:'#00ff00',cyan:'#00ffff',magenta:'#ff00ff',blue:'#0000ff',red:'#ff0000',darkBlue:'#000080',darkCyan:'#008080',darkGreen:'#008000',darkMagenta:'#800080',darkRed:'#800000',darkYellow:'#808000',darkGray:'#808080',lightGray:'#c0c0c0',black:'#000000',white:'#ffffff'} as Record<string,string>)[run.highlight??''],
    'vertical-align': run.vertical==='superscript'?'super':run.vertical==='subscript'?'sub':undefined,
    'font-weight': run.bold ? '700' : undefined,
    'font-style': run.italic ? 'italic' : undefined,
  };
}

function RunField(props: { onComposition: (active: boolean) => void; onHistory: (direction: 'undo' | 'redo') => void; run: Run; fullWidth: boolean; align?: 'start' | 'center' | 'end' | 'justify'; label: string; saving: boolean; onFocus: () => void; onSplitNote?: (offset:number) => boolean; onStructure?: (action: StructureAction, start: number, end: number, insertion?: string) => boolean; onEdit: (text: string) => void }) {
  const [error, setError] = createSignal('');
  const [hint, setHint] = createSignal('');
  const [composing, setComposing] = createSignal(false);
  let softBreak = false;
  const splitNote = (field:HTMLTextAreaElement) => {
    const start=field.selectionStart;
    if (start!==field.selectionEnd||start===0||start===field.value.length) {
      setHint('각주·미주가 붙은 문단을 나누려면 글자 사이에 커서를 놓으세요.');
      return;
    }
    if (props.onSplitNote?.(start)) setHint('');
    else setHint('현재 초안을 저장하거나 실행 취소한 뒤 다시 나누세요.');
  };
  const descriptionId = `input-error-${props.run.id.replace(/[^a-zA-Z0-9_-]/g, '-')}`;
  return <>
    <textarea value={props.run.text} rows={Math.max(1, Math.ceil(props.run.text.length / 72))} readOnly={props.saving}
      title={props.onSplitNote ? '글자 사이에서 Enter를 눌러 나누고 저장하세요.' : props.onStructure ? undefined : t('paragraphStructureUnavailable')} placeholder={props.run.text ? undefined : t('typeHere')} name={props.run.id} autocomplete="off" aria-label={props.label} aria-invalid={Boolean(error())} aria-describedby={error() || hint() ? descriptionId : undefined}
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
        if (action==='split' && props.onSplitNote) { event.preventDefault(); splitNote(field); return; }
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
        if (props.onSplitNote) { event.preventDefault(); splitNote(field); return; }
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
