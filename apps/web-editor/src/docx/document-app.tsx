import { DocumentTabsToolbar } from './document-tabs-toolbar';
import { DocumentListToolbar } from './document-list-toolbar';
import { SaveDOCXCopyAction } from './save-copy-action';
import { RecoveryControls } from './recovery-controls';
import type { RecoveryStatus } from './recovery';
import { DocumentHeaderFooter } from './document-header-footer';
import { DocumentSectionToolbar } from './document-section-toolbar';
import { FileActions } from '../ui/file-actions';
import { warningMessage } from '../ui/messages';
import { DiagnosticDetails } from '../i18n/diagnostic-details';
import { t, localizeMessage } from '../i18n';
import { connectWorkspaceBridge, publishEditorState } from '../ui/workspace-bridge';
import { DocumentSearch } from '../ui/document-search';
import { readTextFieldSelection, readTextSelection, restoreTextSelection, revealTextSelection } from './selection';
import { searchDocument } from './search';
import { For, Show, createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { DocumentController } from './controller';
import { createDocumentView } from './view';
import { StatusPanel } from '../ui/status-panel';
import type { ActivityEntry } from '../ui/activity';
import { ActivityPanel } from '../ui/activity-panel';
import { DocumentContent, layoutPreviewPages, previewPages } from './document-content';
import { DocumentProofingToolbar } from './document-proofing-toolbar';
import { DocumentToolbar } from './document-toolbar';
import type { PasteMode } from './clipboard';
import type { BoundaryMode, Run } from './contracts';
import { DocumentBoundaryToolbar } from './document-boundary-toolbar';
import { DocumentImageToolbar } from './document-image-toolbar';
import { printDocument, printPaperRules } from './print-layout';
import { DocumentOutline } from './document-outline';
import { DocumentParagraphMoveToolbar } from './document-paragraph-move-toolbar';
import { DocumentReferenceToolbar } from './document-reference-toolbar';
import { DocumentComments } from './document-comments';
import { DocumentMathToolbar, DocumentShapeToolbar } from './document-objects';
import { DocumentObjectTextToolbar } from './document-object-text-toolbar';
import { paragraphs, restoreAnchoredPoint } from './paragraphs';
import { DocumentTableToolbar } from './document-table-toolbar';
import { EditorHeader, buttonClass, primaryButtonClass } from '../ui/editor-chrome';



const defaultPaper = { width: 11906, height: 16838, top: 1440, right: 1440, bottom: 1440, left: 1440 };

export function DocumentApp() {
  const toolGroups = [
    {id:'format',label:'서식'},
    {id:'insert',label:'삽입'},
    {id:'layout',label:'페이지'},
    {id:'recovery',label:'복구'},
  ] as const;
  const [toolGroup,setToolGroup] = createSignal<(typeof toolGroups)[number]['id']>();
  const { state, view, activity } = createDocumentView();
  const recoveryAvailable = new URLSearchParams(window.location.search).has('document');
  const [recovery, setRecovery] = createSignal<RecoveryStatus>({});

  const [printing,setPrinting] = createSignal(false);
  const [printError,setPrintError] = createSignal('');
  const [draftVersion, setDraftVersion] = createSignal(0);
  const [formattingSelection,setFormattingSelection]=createSignal<import('./selection').TextSelection>();
  const [pasteMode, setPasteMode] = createSignal<PasteMode>('text');
  const [zoom, setZoom] = createSignal(100);
  const [focusedRun, setFocusedRun] = createSignal<Run>();
  const paper = () => {const pages=state().snapshot?.sections?.map(section=>section.page)??[];return pages.length?pages.reduce((largest,page)=>page.width>largest.width?page:largest):state().snapshot?.page??defaultPaper;};
  const paperStyle = () => {
    const { width, height, top, right, bottom, left } = paper();
    const inset = (twips: number) => `${100 * twips / width}%`;
    return {
      zoom: String(zoom() / 100),
      '--paper-width': `${width / 15}px`,
      '--paper-ratio': String(height / width),
      '--paper-insets': [top, right, bottom, left].map(inset).join(' '),
    };
  };
  const [structuralDraft, setStructuralDraft] = createSignal(false);
  const [selectedChange, setSelectedChange] = createSignal<ActivityEntry>();
  const anchors=createMemo(()=>{
    const snapshot=state().snapshot;
    return new Map(snapshot?paragraphs(snapshot).flatMap(entry=>entry.paragraph.anchor?[[entry.paragraph.anchor,entry.key] as const]:[]):[]);
  });
  const resolvedEntry=(entry:ActivityEntry|undefined):ActivityEntry|undefined=>{
    const change=entry?.documentChange,snapshot=state().snapshot;
    if(structuralDraft()||!entry||!change||!snapshot||change.session!==snapshot.id)return;
    const key=change.anchor?anchors().get(change.anchor):entry.revision===snapshot.revision?change.key:undefined;
    return key===undefined?undefined:{...entry,documentChange:{...change,key}};
  };
  const canLocate=(entry:ActivityEntry)=>Boolean(resolvedEntry(entry));
  const changes=createMemo(()=>{
    const latest=new Map<string,ActivityEntry>();
    for(const entry of activity.entries()){
      const resolved=resolvedEntry(entry),key=resolved?.documentChange?.key;
      if(resolved&&key!==undefined&&!latest.has(key))latest.set(key,resolved);
    }
    return latest;
  });
  const locate=(entry:ActivityEntry)=>{
    const resolved=resolvedEntry(entry);
    if(!resolved)return;
    if(window.matchMedia('(max-width: 640px)').matches)activity.toggle();
    setSelectedChange(entry);
    queueMicrotask(()=>{
      const current=resolvedEntry(entry);
      if(current)document.getElementById(`docx-paragraph-${current.documentChange!.key}`)?.focus();
    });
  };
  const noteSplitPending = () => { state().snapshot; draftVersion(); return controller.pendingNoteSplit(); };
  const standalonePending = () => { state().snapshot; draftVersion(); return controller.pendingStandalone(); };
  const documentLinks = createMemo(() => {
    const snapshot = state().snapshot;
    return snapshot ? paragraphs(snapshot).flatMap(({paragraph}) => paragraph.runs.filter(run => run.link && !('field' in run.link && run.link.field))) : [];
  });
  const documentNotes = createMemo(() => {
    const snapshot = state().snapshot;
    return snapshot ? paragraphs(snapshot).flatMap(({paragraph}) =>
      paragraph.runs.filter(run => (run.object?.kind==='footnote'||run.object?.kind==='endnote')&&run.object.editable)) : [];
  });
  const documentReferenceLists = createMemo(() => state().snapshot?.blocks.flatMap(block=>block.type==='paragraph'
    ? block.paragraph.runs.filter(run=>run.object?.editable && ['toc','figure_list','table_list'].includes(run.object.kind))
    : [])??[]);
  const documentDateFields = createMemo(() => state().snapshot?.blocks.flatMap(block=>block.type==='paragraph'
    ? block.paragraph.runs.filter(run=>run.object?.kind==='date_field'&&run.object.editable) : [])??[]);
  const pendingBookmarkReferences = createMemo(() => state().snapshot?.blocks.flatMap(block=>block.type==='paragraph'
    ? block.paragraph.runs.filter(run=>run.link && 'refreshNeeded' in run.link && run.link.refreshNeeded) : [])??[]);
  const referenceParagraph = () => boundaryParagraph()?.paragraph ??
    state().snapshot?.blocks.flatMap(block=>block.type==='paragraph'&&block.paragraph.structureEditable?[block.paragraph]:[]).at(-1);
  const commentRange = () => {
    const target = referenceParagraph();
    const selection = formattingSelection();
    if (!target?.structureEditable || !selection) return undefined;
    const a = target.runs.findIndex(run=>run.id===selection.anchor.runId);
    const b = target.runs.findIndex(run=>run.id===selection.focus.runId);
    if (a < 0 || b < 0) return undefined;
    const forwards = a < b || a === b && selection.anchor.offset <= selection.focus.offset;
    const [start,end] = forwards ? [selection.anchor,selection.focus] : [selection.focus,selection.anchor];
    const selected = target.runs.slice(Math.min(a,b),Math.max(a,b)+1)
      .map((run,index,runs)=>{
        const startOffset=index===0?start.offset:0;
        const endOffset=index===runs.length-1?end.offset:run.text.length;
        return {run,startOffset,endOffset,text:run.text.slice(startOffset,endOffset)};
      });
    if (!selected.every(part=>part.run.editable) || !selected.some(part=>part.text.trim())) return undefined;
    const parts=selected.filter(part=>part.text.length>0);
    const first=parts[0]!,last=parts.at(-1)!;
    return {startRunId:first.run.id,endRunId:last.run.id,startOffset:first.startOffset,endOffset:last.endOffset};
  };
  const imagePending = () => { state().snapshot; draftVersion(); return controller.pendingImage(); };
  const pictures = createMemo(() => {
    const snapshot = state().snapshot;
    return snapshot ? paragraphs(snapshot).flatMap(({paragraph}) => paragraph.runs.filter(run => run.picture?.preview)) : [];
  });
  const imageParagraph = () => {
    const blocks = state().snapshot?.blocks??[];
    const paragraphs = blocks.flatMap(block=>block.type==='paragraph'?[block.paragraph]:block.rows.flatMap(row=>row.cells.flatMap(cell=>cell.paragraphs)))
      .filter(paragraph=>paragraph.structureEditable);
    const runId = formattingSelection()?.focus.runId??focusedRun()?.id;
    return paragraphs.find(paragraph=>paragraph.runs.some(run=>run.id===runId))??boundaryParagraph()?.paragraph
      ??blocks.flatMap(block=>block.type==='paragraph'&&block.paragraph.structureEditable?[block.paragraph]:[]).at(-1);
  };
  const boundaryPending = () => { state().snapshot; draftVersion(); return Boolean(controller.pendingBoundary()); };
  const boundaryParagraph = () => {
    const runId = formattingSelection()?.focus.runId ?? focusedRun()?.id;
    if (!runId) return undefined;
    const snapshot = state().snapshot;
    const index = snapshot?.blocks.findIndex(block => block.type === 'paragraph' && block.paragraph.runs.some(run => run.id === runId)) ?? -1;
    const block = snapshot?.blocks[index];
    if (block?.type !== 'paragraph') return undefined;
    const section = snapshot?.sections?.find(section => section.id === block.paragraph.sectionId);
    return {paragraph: block.paragraph, canSplit: Boolean(section?.editable && index < section.endBlock)};
  };
  const sectionBreaks = createMemo(() => {
    const snapshot = state().snapshot;
    if (!snapshot?.sections) return [];
    return snapshot.sections.slice(0,-1).flatMap((section,index)=>{
      const block=snapshot.blocks[section.endBlock];
      const next=snapshot.sections?.[index+1];
      if (!section.editable||!next?.editable||next.startBlock>next.endBlock||block?.type!=='paragraph') return [];
      const canMove=(position:number)=>{
        const destination=snapshot.blocks[position];
        return destination?.type==='paragraph'&&destination.paragraph.structureEditable&&
          !destination.paragraph.pageBreakBefore&&!destination.paragraph.pageBreakAfter;
      };
      return [{paragraph:block.paragraph,
        earlier:section.endBlock>section.startBlock&&canMove(section.endBlock-1),
        later:next.endBlock>next.startBlock&&canMove(section.endBlock+1)}];
    });
  });
  const stageBoundary = (targetId: string, boundary: BoundaryMode) => {
    const changed = controller.editBoundary({type:'update_boundary', targetId, boundary});
    if (changed) setDraftVersion(version => version + 1);
    return changed;
  };
  const printReady = () => {
    state().message; draftVersion();
    return Boolean(state().snapshot) && !state().error && !state().loading && !controller.hasDrafts() && !printing();
  };
  const requestPrint = async () => {
    const article = document.getElementById('document-content');
    if (!article || !printReady()) return;
    setPrintError('');
    setPrinting(true);
    const snapshot = state().snapshot;
    try {
      const started = await printDocument(article, () => state().snapshot === snapshot && !controller.hasDrafts());
      if (!started) setPrintError('문서가 바뀌었습니다. 저장 상태를 확인하고 다시 인쇄하세요.');
    }
    catch { setPrintError('그림이나 글꼴을 불러오지 못했습니다. 문서를 다시 연 뒤 인쇄하세요.'); }
    finally { setPrinting(false); }
  };
  const previewBlocks = createMemo(() => state().snapshot?.blocks);
  const pages = createMemo(() => previewPages(previewBlocks() ?? []));
  const pageLayout = createMemo(() => layoutPreviewPages(pages(),state().snapshot?.sections,state().snapshot?.evenAndOddHeaders));
  const locatePage = (number: number) => {
    const page = document.querySelector<HTMLElement>(`#document-content [data-docx-page="${number}"]`);
    if (!page) return;
    page.focus({preventScroll:true});
    page.scrollIntoView({block:'start'});
  };
  const locateHeading = (id:string) => {
    const paragraph=[...document.querySelectorAll<HTMLElement>('#document-content [data-docx-paragraph]')]
      .find(element=>element.dataset.docxParagraph===id);
    if(!paragraph)return;
    const runId=paragraph.querySelector<HTMLElement>('[data-docx-run]')?.dataset.docxRun;
    const editor=paragraph.closest<HTMLElement>('[data-continuous-editor]');
    if(editor&&runId){
      const point={runId,offset:0};
      const selection={anchor:point,focus:point};
      if(restoreTextSelection(editor,selection))setFormattingSelection(selection);
    }else{
      setFormattingSelection(undefined);
      setFocusedRun(undefined);
      paragraph.focus({preventScroll:true});
    }
    paragraph.scrollIntoView({block:'start'});
  };
  const tablePending = () => { state().snapshot; draftVersion(); return controller.pendingTable(); };
  const insertTableAfter = () => {
    const paragraphs = state().snapshot?.blocks.flatMap(block=>block.type==='paragraph'&&block.paragraph.structureEditable?[block.paragraph]:[])??[];
    const runId = formattingSelection()?.focus.runId??focusedRun()?.id;
    return paragraphs.find(paragraph=>paragraph.runs.some(run=>run.id===runId))??paragraphs.at(-1);
  };
  const controller = new DocumentController({ ...view, recovery: setRecovery, render: (snapshot, focus, preserveRemoteSelection) => {
    setStructuralDraft(controller.hasStructuralDraft() || (Boolean(focus) && controller.hasDrafts()));
    if(focus)setFormattingSelection(focus.selection??{anchor:{runId:focus.runId,offset:focus.start},focus:{runId:focus.runId,offset:focus.end}});
    const article = document.getElementById('document-content');
    const active = document.activeElement;
    const previousEditor = article?.querySelector<HTMLElement>('[data-continuous-editor]');
    const previousSnapshot = state().snapshot;
    const currentSelection = previousEditor && readTextSelection(previousEditor);
    const activeField = active instanceof HTMLTextAreaElement && article?.contains(active) ? active : undefined;
    const remoteSelection = activeField ? readTextFieldSelection(activeField)
      : active === previousEditor ? currentSelection : undefined;
    const richSelection = preserveRemoteSelection ? remoteSelection
      : currentSelection ?? (previousSnapshot?.id === snapshot.id ? formattingSelection() : undefined);
    const previousRuns = [...article?.querySelectorAll<HTMLElement>('[data-docx-run]') ?? []];
    const fields = [...article?.querySelectorAll('textarea') ?? []];
    const previousField = activeField ?? (state().saving ? fields.find(field => field.name === focusedRun()?.id) : undefined);
    const index = previousField && state().snapshot?.id === snapshot.id ? fields.indexOf(previousField) : -1;
    const selection = index >= 0 && previousField ? [previousField.selectionStart, previousField.selectionEnd] as const : undefined;
    const restoringSave = state().saving;
    if(!focus && !restoringSave){setFormattingSelection(undefined);setFocusedRun(undefined);}
    if (state().snapshot?.id !== snapshot.id) setFocusedRun(undefined);
    view.render(snapshot);
    if (focus) {
      queueMicrotask(() => {
        const field = [...article?.querySelectorAll('textarea') ?? []].find(field => field.name === focus.runId);
        if (field) { field.focus({ preventScroll: true }); field.setSelectionRange(focus.start, focus.end); }
        else {
          const editor = article?.querySelector<HTMLElement>('[data-continuous-editor]');
          if (editor) restoreTextSelection(editor, focus.selection ?? { anchor: { runId: focus.runId, offset: focus.start }, focus: { runId: focus.runId, offset: focus.end } });
        }
      });
    } else if (richSelection && (restoringSave || preserveRemoteSelection)) {
      queueMicrotask(() => {
        const editor = article?.querySelector<HTMLElement>('[data-continuous-editor]');
        const currentRuns = [...article?.querySelectorAll<HTMLElement>('[data-docx-run]') ?? []];
        const remap = (point: typeof richSelection.anchor) => {
          if (previousSnapshot) {
            const anchored = restoreAnchoredPoint(previousSnapshot,snapshot,point);
            if (anchored !== undefined) return anchored;
          }
          if (preserveRemoteSelection) return undefined;
          const index = previousRuns.findIndex(run => run.dataset.docxRun === point.runId);
          const run = index >= 0 ? currentRuns[Math.min(index, currentRuns.length - 1)] : undefined;
          return run ? { runId: run.dataset.docxRun!, offset: Math.min(point.offset, run.textContent?.length ?? 0) } : undefined;
        };
        const anchor = remap(richSelection.anchor), focus = remap(richSelection.focus);
        if (anchor && focus && anchor.runId === focus.runId) {
          const field = [...article?.querySelectorAll('textarea') ?? []].find(candidate => candidate.name === anchor.runId);
          if (field) {
            field.focus({preventScroll:true});
            field.setSelectionRange(Math.min(anchor.offset, focus.offset), Math.max(anchor.offset, focus.offset),
              focus.offset < anchor.offset ? 'backward' : 'forward');
            setFormattingSelection(undefined);
            return;
          }
        }
        setFormattingSelection(editor && anchor && focus && restoreTextSelection(editor, {anchor, focus})
          ? {anchor, focus} : undefined);
      });
    } else if (selection && (restoringSave || document.activeElement === document.body)) {
      const field = article?.querySelectorAll('textarea')[index];
      field?.focus({ preventScroll: true });
      field?.setSelectionRange(...selection);
    }
  } });
  const selectionChanged = () => {
    const editor=document.querySelector<HTMLElement>('[data-continuous-editor]');
    const range=editor && readTextSelection(editor);if(range)setFormattingSelection(range);
  };
  const selectedStyles = createMemo(() => {
    draftVersion();state().snapshot;const selection=formattingSelection();
    return selection ? controller.selectedRuns(selection).map(run=>({...run,underline:Boolean(run.underline),strike:Boolean(run.strike),highlight:run.highlight??'none',vertical:run.vertical??'baseline'})) : [];
  });
  const selectedParagraphFormats=createMemo(()=>{draftVersion();state().snapshot;const selection=formattingSelection();return selection?controller.selectedParagraphs(selection).map(p=>p.layout??{}):[];});
  const applyParagraphFormat=(format:import('./contracts').ParagraphFormat)=>{const selection=formattingSelection();if(selection&&controller.formatParagraph(selection,format))setDraftVersion(v=>v+1);};
  const stylesCatalog=createMemo(()=>{draftVersion();state().snapshot;return controller.styles();});
  const selectedParagraphStyleIds=createMemo(()=>{draftVersion();state().snapshot;const selection=formattingSelection();return selection?controller.selectedParagraphs(selection).map(p=>p.styleId??''):[];});
  const selectedCharacterStyleIds=createMemo(()=>{draftVersion();state().snapshot;const selection=formattingSelection();return selection?controller.selectedRuns(selection).map(run=>run.characterStyleId??''):[];});
  const stylePending=createMemo(()=>{draftVersion();state().snapshot;return controller.hasStyleDrafts();});
  const applyFormat = (style: import('./contracts').InlineStyle) => {
    const selection=formattingSelection();if(selection && controller.format(selection,style))setDraftVersion(v=>v+1);
  };
  const [confirmReload, setConfirmReload] = createSignal(false);
  const cancelReload = () => { setConfirmReload(false); document.getElementById('reload-document')?.focus(); };
  const requestReload = () => {
    if (!controller.hasDrafts()) { reload(); return; }
    setConfirmReload(true);
    queueMicrotask(() => document.getElementById('discard-draft')?.focus());
  };
  const reload = () => {
    setConfirmReload(false);
    void controller.load().then(() => {
      document.getElementById(state().error ? 'document-error' : 'document-content')?.focus();
    });
  };
  const resized = () => {window.requestAnimationFrame(()=>{const editor=document.querySelector<HTMLElement>('[data-continuous-editor]');if(editor?.contains(document.activeElement))revealTextSelection(editor);});};
  const beforeUnload = (event: BeforeUnloadEvent) => {
    if (controller.hasDrafts()) { event.preventDefault(); event.returnValue = ''; }
  };
  const restoreHistory = (direction: 'undo' | 'redo') => {
    const editor = document.querySelector<HTMLElement>('[data-continuous-editor]');
    void controller.restore(direction, editor ? readTextSelection(editor) : undefined).then(changed => { if (changed) setDraftVersion(v => v + 1); });
  };
  const keydown = (event: KeyboardEvent) => {
    if (event.key === 'Escape' && confirmReload()) { event.preventDefault(); cancelReload(); return; }
    const key = event.key.toLowerCase();
    const styleKey = ({b:'bold',i:'italic',u:'underline'} as const)[key as 'b'|'i'|'u'];
    if ((event.ctrlKey || event.metaKey) && !event.altKey && styleKey && (event.target as HTMLElement | null)?.closest('[data-continuous-editor]')) {
      if(event.isComposing)return;
      event.preventDefault();selectionChanged();
      applyFormat({[styleKey]:!selectedStyles().every(style=>style[styleKey]===true)});return;
    }
    if ((event.ctrlKey || event.metaKey) && (key === 'z' || key === 'y')) {
      const target = event.target as HTMLElement | null;
      if (event.isComposing || target?.closest('input, select') || target?.closest('textarea') && !target.closest('#document-content')) return;
      event.preventDefault(); restoreHistory(key === 'y' || event.shiftKey ? 'redo' : 'undo'); return;
    }
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
      event.preventDefault();
      if (state().validation) { document.querySelector<HTMLTextAreaElement>('textarea[aria-invalid="true"]')?.focus(); return; }
      void controller.save();
    }
  };
  const editorState = () => ({ message: state().message, dirty: controller.hasDrafts(), busy: state().saving || state().loading });
  createEffect(() => { state(); draftVersion(); queueMicrotask(() => publishEditorState(editorState())); });
  let disconnectWorkspace = () => {};
  let disposed = false;
  const dispose = () => {
    if (disposed) return;
    disposed = true;
    window.removeEventListener('online', retryRecovery);
    window.removeEventListener('resize',resized);
    document.removeEventListener('selectionchange',selectionChanged);
    window.removeEventListener('beforeunload', beforeUnload);
    window.removeEventListener('keydown', keydown);
    window.removeEventListener('pagehide', dispose);
    disconnectWorkspace();
    controller.dispose();
  };
  const retryRecovery = () => controller.retryRecovery();
  onMount(() => {
    window.addEventListener('online', retryRecovery);
    disconnectWorkspace = connectWorkspaceBridge(editorState, async () => { await controller.save(); });
    window.addEventListener('resize',resized);
    document.addEventListener('selectionchange',selectionChanged);
    window.addEventListener('beforeunload', beforeUnload);
    window.addEventListener('keydown', keydown);
    window.addEventListener('pagehide', dispose, { once: true });
    void controller.load();
  });
  onCleanup(dispose);
  return <>
    <a class="skip-link" href="#document-content">{t('skipToContent')}</a>
    <div data-docx-app class="flex h-full min-h-0 flex-col overflow-hidden">
    <Show when={state().snapshot}>{snapshot=><style media="print">{printPaperRules(snapshot())}</style>}</Show>
    <EditorHeader name={state().snapshot?.name ?? t('openingDocument')} kind={t('document')} nameId="document-name" statusId="save-status"
      message={state().message} status={state().status} onActivity={activity.toggle} activityOpen={activity.open()}
      actions={<><SaveDOCXCopyAction name={state().snapshot?.name??'document.docx'} disabled={!state().snapshot||state().saving||state().loading||Boolean(state().composing||state().validation)} onExport={name=>controller.exportCopy(name)}/><FileActions saveID="save-document" disabled={!state().snapshot || state().saving || state().loading || Boolean(state().error || state().validation)} downloadAvailable onSave={() => controller.save()} onDownload={() => controller.download()}/><button type="button" class={buttonClass} disabled={!printReady()} onClick={()=>void requestPrint()} title="저장된 문서를 인쇄하거나 PDF로 저장합니다.">인쇄·PDF</button><DocumentSearch version={`${state().snapshot?.id}:${state().snapshot?.revision}:${state().message}:${state().saving}:${draftVersion()}`} docxEditing search={async (query,options) => searchDocument(document.getElementById('document-content'),query,options,(ranges,text)=>{
      const changed=controller.replaceSearchMatches(ranges,text);
      if(changed)setDraftVersion(version=>version+1);
      return changed;
    })} hint={t('searchBodyTextAndTablesIncludingUnsavedInput')}/><DocumentComments snapshot={state().snapshot} target={referenceParagraph()} range={commentRange()} pending={standalonePending()}
      disabled={state().saving||state().loading||boundaryPending()||imagePending()||Boolean(state().error)||Boolean(state().composing)}
      onApply={operation=>{const changed=controller.editStandalone(operation);if(changed)setDraftVersion(v=>v+1);return changed;}}
      onLocate={locateHeading}/><DocumentOutline snapshot={state().snapshot} styles={stylesCatalog()} activeRun={formattingSelection()?.focus.runId??focusedRun()?.id} onLocate={locateHeading} pageCount={pageLayout().at(-1)?.number??1} onLocatePage={locatePage}/></>}
    />
    <section aria-label="편집 도구 모음" tabindex={0}
      class="min-h-0 max-h-[45dvh] shrink overflow-y-auto overscroll-contain focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-teal-700">
    <DocumentToolbar stylesCatalog={stylesCatalog()} paragraphStyleIds={selectedParagraphStyleIds()} characterStyleIds={selectedCharacterStyleIds()} definitionDisabled={tablePending()||state().saving||state().loading||Boolean(state().error)||Boolean(state().composing)} stylePending={stylePending()} onApplyStyle={(kind,id)=>kind==='paragraph'?applyParagraphFormat({styleId:id}):applyFormat({styleId:id})} onDefineStyle={definition=>{if(controller.editStyle(definition))setDraftVersion(v=>v+1);}} paragraphs={selectedParagraphFormats()} onParagraphFormat={applyParagraphFormat} styles={selectedStyles()} formatDisabled={!selectedStyles().length || state().composing || state().saving || state().loading || Boolean(state().error)} onFormat={applyFormat} history={state().history} onHistory={restoreHistory} pasteMode={pasteMode()} onPasteMode={setPasteMode} saving={state().saving} loading={state().loading}
      needsRecovery={Boolean(state().error)} run={focusedRun()} zoom={zoom()} onZoom={setZoom}
      onReload={requestReload}/>
    <div role="group" aria-label="추가 편집 도구" class="flex flex-wrap gap-1 border-b border-slate-200 bg-white px-3 py-1 sm:px-6">
      <For each={toolGroups}>{group=>
        <Show when={group.id!=='recovery' || recoveryAvailable}>
          <button type="button" class="editor-action min-h-11 rounded-md px-3 text-sm font-medium text-slate-700 aria-pressed:bg-teal-50 aria-pressed:text-teal-900"
            aria-pressed={toolGroup()===group.id} aria-controls={`docx-tools-${group.id}`}
            onClick={()=>setToolGroup(current=>current===group.id?undefined:group.id)}>{group.label}</button>
        </Show>
      }</For>
    </div>
    <div id="docx-tools-format" hidden={toolGroup()!=='format'}>
    <DocumentProofingToolbar styles={selectedStyles()} disabled={!selectedStyles().length||state().composing||state().saving||state().loading||Boolean(state().error)} onFormat={applyFormat}/>
    <DocumentListToolbar paragraphs={(() => {draftVersion();state().snapshot;return formattingSelection()?controller.selectedParagraphs(formattingSelection()!):[];})()}
      lists={state().snapshot?.lists??[]} disabled={state().saving||state().loading||Boolean(state().error||state().composing)}
      onList={list=>{const selection=formattingSelection();if(selection&&controller.applyList(selection,list))setDraftVersion(v=>v+1);}}
      onCompoundRestart={(targetId,endTargetId,listStart)=>{
        if(controller.editStandalone({type:'restart_compound_sublevel',targetId,endTargetId,listStart}))setDraftVersion(v=>v+1);
      }}
      onLevel={delta=>{const selection=formattingSelection();if(selection&&controller.changeListLevel(selection,delta))setDraftVersion(v=>v+1);}}/>
    <DocumentTabsToolbar formats={selectedParagraphFormats()} disabled={!selectedStyles().length||state().composing||state().saving||state().loading||Boolean(state().error)} onFormat={applyParagraphFormat} onInsert={()=>{const selection=formattingSelection();if(selection&&controller.replaceSelection(selection,'\t',true))setDraftVersion(v=>v+1);}}/>
    </div>
    <div id="docx-tools-insert" hidden={toolGroup()!=='insert'}>
    <DocumentImageToolbar pictures={pictures()} target={imageParagraph()} pending={imagePending()}
      disabled={state().saving||state().loading||boundaryPending()||imagePending()||Boolean(state().error)||Boolean(state().composing)}
      onApply={operation=>{const changed=controller.editImage(operation);if(changed)setDraftVersion(v=>v+1);return changed;}}/>
    <DocumentReferenceToolbar figures={pictures().filter(run=>run.picture?.captionReference)} links={documentLinks()} notes={documentNotes()} referenceLists={documentReferenceLists()} dateFields={documentDateFields()} pendingReferences={pendingBookmarkReferences()} target={referenceParagraph()} bookmarks={state().snapshot?.bookmarks??[]} pending={standalonePending()}
      disabled={state().saving||state().loading||boundaryPending()||imagePending()||Boolean(state().error)||Boolean(state().composing)}
      onApply={operation=>{const changed=controller.editStandalone(operation);if(changed)setDraftVersion(v=>v+1);return changed;}}/>
    <DocumentMathToolbar snapshot={state().snapshot} target={imageParagraph()} pending={standalonePending()}
      disabled={state().saving||state().loading||boundaryPending()||imagePending()||Boolean(state().error)||Boolean(state().composing)}
      onApply={operation=>{const changed=controller.editStandalone(operation);if(changed)setDraftVersion(v=>v+1);return changed;}}/>
    <DocumentShapeToolbar snapshot={state().snapshot} target={referenceParagraph()} pending={standalonePending()}
      disabled={state().saving||state().loading||boundaryPending()||imagePending()||Boolean(state().error)||Boolean(state().composing)}
      onApply={operation=>{const changed=controller.editStandalone(operation);if(changed)setDraftVersion(v=>v+1);return changed;}}/>
    <For each={['control','textbox','chart'] as const}>{kind=>
      <DocumentObjectTextToolbar kind={kind} target={referenceParagraph()} snapshot={state().snapshot} pending={standalonePending()}
      disabled={state().saving||state().loading||boundaryPending()||imagePending()||Boolean(state().error)||Boolean(state().composing)}
      onApply={operation=>{const changed=controller.editStandalone(operation);if(changed)setDraftVersion(v=>v+1);return changed;}}/>
    }</For>
    <DocumentTableToolbar blocks={state().snapshot?.blocks??[]} insertAfter={insertTableAfter()} pending={tablePending()}
      disabled={state().saving||state().loading||Boolean(state().error)||Boolean(state().composing)}
      onApply={operation=>{const applied=controller.editTable(operation);if(applied)setDraftVersion(v=>v+1);return applied;}}/>
    </div>
    <div id="docx-tools-layout" hidden={toolGroup()!=='layout'}>
    <DocumentHeaderFooter evenAndOddHeaders={state().snapshot?.evenAndOddHeaders} onParityChange={enabled=>{const changed=controller.editHeaderParity(enabled);if(changed)setDraftVersion(v=>v+1);return changed;}} sections={state().snapshot?.sections??[]} disabled={state().saving||state().loading||boundaryPending()||imagePending()||Boolean(state().error)||Boolean(state().composing)} onApply={(id,format)=>{const changed=controller.editSection(id,format);if(changed)setDraftVersion(v=>v+1);return changed;}}/>
    <DocumentSectionToolbar sections={state().snapshot?.sections??[]} disabled={state().saving||state().loading||boundaryPending()||imagePending()||Boolean(state().error)||Boolean(state().composing)} onApply={(id,format)=>{const changed=controller.editSection(id,format);if(changed)setDraftVersion(v=>v+1);return changed;}}/>
    <DocumentParagraphMoveToolbar kind="bookmark" paragraphs={state().snapshot?.blocks.flatMap(block=>
      block.type==='paragraph'&&(block.paragraph.moveBookmarkUp||block.paragraph.moveBookmarkDown)?[block.paragraph]:[])??[]}
      pending={standalonePending()} disabled={state().saving||state().loading||Boolean(state().error)||Boolean(state().composing)}
      onApply={operation=>{const changed=controller.editStandalone(operation);if(changed)setDraftVersion(v=>v+1);return changed;}}/>
    <DocumentParagraphMoveToolbar kind="hyperlink" paragraphs={state().snapshot?.blocks.flatMap(block=>
      block.type==='paragraph'&&(block.paragraph.moveHyperlinkUp||block.paragraph.moveHyperlinkDown)?[block.paragraph]:[])??[]}
      pending={standalonePending()} disabled={state().saving||state().loading||Boolean(state().error)||Boolean(state().composing)}
      onApply={operation=>{const changed=controller.editStandalone(operation);if(changed)setDraftVersion(v=>v+1);return changed;}}/>
    <DocumentParagraphMoveToolbar kind="note" paragraphs={state().snapshot?.blocks.flatMap(block=>
      block.type==='paragraph'&&(block.paragraph.moveNoteUp||block.paragraph.moveNoteDown)?[block.paragraph]:[])??[]}
      pending={standalonePending()} disabled={state().saving||state().loading||Boolean(state().error)||Boolean(state().composing)}
      onApply={operation=>{const changed=controller.editStandalone(operation);if(changed)setDraftVersion(v=>v+1);return changed;}}/>
    <DocumentBoundaryToolbar sectionBreaks={sectionBreaks()} target={boundaryParagraph()} trailingBreaks={state().snapshot?.blocks.flatMap(block=>block.type==='paragraph' && block.paragraph.pageBreakAfter ? [block.paragraph] : []) ?? []} pending={boundaryPending()}
      disabled={state().saving||state().loading||Boolean(state().error)||Boolean(state().composing)} onApply={stageBoundary}/>
    </div>
    <div id="docx-tools-recovery" hidden={toolGroup()!=='recovery'}>
    <Show when={recoveryAvailable && state().snapshot}>
      <RecoveryControls status={recovery()} disabled={state().saving || state().loading || Boolean(state().composing || state().validation)}
        onEnable={() => controller.enableRecovery()} onDelete={() => controller.removeRecovery()} onRetry={retryRecovery}/>
    </Show>
    </div>
    </section>
    <Show when={printError()}><p role="alert" class="border-b border-red-200 px-4 py-2 text-sm text-red-800">{printError()}</p></Show>
    <Show when={state().error}>{(message) => <StatusPanel id="document-error" panel={{ title: t('reviewRequired'), message: message(), details: state().errorDetails, dismissible: false }} danger onDismiss={() => {}}/>}</Show>
    <Show when={state().validation}>{(message) => <p class="shrink-0 border-b border-red-200 bg-red-50 px-4 py-2 text-xs leading-6 text-red-800 sm:px-6" role="alert">{localizeMessage(message())}</p>}</Show>
    <Show when={confirmReload()}><section class="shrink-0 border-b border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-950 sm:px-6" role="region" aria-label={t('confirmDiscardingUnsavedInput')}>
      <p>{t('discardUnsavedInputAndReloadTheSavedDocument')}</p>
      <div class="mt-3 flex flex-wrap gap-2"><button id="discard-draft" type="button" class={primaryButtonClass} onClick={reload}>{t('discardReload')}</button><button type="button" class={buttonClass} onClick={cancelReload}>{t('cancel')}</button></div>
    </section></Show>
    <div class="editor-workspace flex min-h-32 flex-1 overflow-hidden" data-activity-open={activity.open()}>
    <main class="editor-primary min-h-0 min-w-0 flex-1 space-y-4 overflow-auto bg-[#e9ebed] px-3 py-4 sm:px-8 sm:py-8">
      <div class="docx-preview mx-auto" style={paperStyle()}>
      <article id="document-content" aria-label={t('docxBodyAndTableEditor')} aria-busy={!state().snapshot} class="space-y-5 text-[11pt] leading-[1.35] text-stone-900" tabindex={-1}>
        <Show when={state().snapshot}>{(snapshot) => <DocumentContent pages={pages()} layout={pageLayout()} onHistory={restoreHistory} pasteMode={pasteMode()} onPasteRich={(selection, paragraphs) => { const changed=controller.pasteRich(selection,paragraphs);if(changed)setDraftVersion(v=>v+1);return changed; }} richSelection={selection=>controller.richSelection(selection)} snapshot={snapshot()} saving={state().saving||noteSplitPending()} changes={changes()}
          selected={resolvedEntry(selectedChange())}
          onListLevel={(selection,delta)=>{const changed=controller.changeListLevel(selection,delta);if(changed)setDraftVersion(v=>v+1);return changed;}}
          onRange={(selection, text, inline) => { const changed = controller.replaceSelection(selection, text, inline); if (changed) setDraftVersion(v => v + 1); return changed; }}
          onDelete={(selection, direction, word) => { const changed = controller.deleteSelection(selection, direction, word); if (changed) setDraftVersion(v => v + 1); return changed; }}
          onComposition={active => controller.composition(active)} onRefresh={selection => controller.refreshSelection(selection)} selectionText={selection => controller.selectionText(selection)}
          onSplitNote={(run, offset) => {
            const staged = controller.splitNoteParagraph(run, offset);
            if (staged) setDraftVersion(v => v + 1);
            return staged;
          }}
          onStructure={(action, id, start, end, insertion) => { const changed = controller.changeStructure(action, id, start, end, insertion); if (changed) setDraftVersion(v => v + 1); return changed; }}
          onFocusRun={setFocusedRun} onChangeSelect={(entry) => setSelectedChange(selectedChange()?.id === entry.id ? undefined : entry)} onEdit={(operation, selection) => { controller.edit(operation, selection); setDraftVersion((version) => version + 1); }}/>}</Show>
      </article>
      <details class="mt-2 rounded-md px-2 py-2 text-xs leading-6 text-stone-600">
        <summary class="editor-action min-h-11 w-fit rounded px-1 py-2 font-medium">{t('editingSupportAndPreservation')}</summary>
        <p>{t('previewPageSizeSomeStylesAndExplicitPage')}</p><p>{t('paragraphEditingHelp')}</p>
        <Show when={state().snapshot?.warnings.length}><p class="whitespace-pre-wrap">{state().snapshot?.warnings.map(warningMessage).join('\n')}</p></Show>
      <DiagnosticDetails value={state().snapshot?.warnings.join('\n')}/>
      </details>
      </div>
    </main>
    <Show when={activity.open()}><ActivityPanel entries={activity.entries()} onClose={activity.toggle} canLocate={canLocate} onLocate={locate}/></Show>
    </div>
    </div>
  </>;
}
