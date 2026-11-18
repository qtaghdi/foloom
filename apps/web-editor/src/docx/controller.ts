import { exportDOCXCopy } from '../transport/save-docx-copy';
import { DraftRecovery, type RecoveryStatus } from './recovery';
import { previewSection,previewSections,validateSection } from './sections';
import { confirmSavedDocument, downloadSavedDocument, DocumentFileError, type DownloadedFile } from '../transport/document-files';
import { source } from '../i18n/index';
import { errorResponseSchema } from '../transport/schemas';
import { documentEvents } from '../transport/document-events';
import { scopedDocumentURL } from '../transport/document-scope';
import type { ClipboardParagraph } from './clipboard';
import type { TextSelection } from './selection';
import { StructureDraft } from './structure-draft';
import { compareParagraphs } from './paragraphs';
import { validateDrafts,validateRunText } from './validation';
import type { ActivityEntry } from '../ui/activity';
import { eventSchema, snapshotSchema, sectionFormatSchema, type SectionFormat, type BoundaryOperation, type ParagraphMoveOperation, type ImageOperation, type ReferenceOperation, type RevisionReviewOperation, type CommentOperation, type ObjectTextOperation, type RectangleOperation, type TrackTextOperation, type MathOperation, type TableOperation, type Snapshot, type ListDefinition, type ParagraphList, type Operation, type TextOperation, type StyleInfo, type StyleDefinition, type ParagraphFormat, type InlineStyle, type DraftFocus, type StructureAction } from './contracts';

export type HistoryControls = { undo: boolean; redo: boolean; undoLabel: string; redoLabel: string };
type DraftCheckpoint = { lists:Array<[string,ListDefinition]>; standalone?:ReferenceOperation|CommentOperation|MathOperation|TableOperation|ParagraphMoveOperation|RevisionReviewOperation|ObjectTextOperation|RectangleOperation|TrackTextOperation; image?:ImageOperation; boundary?: BoundaryOperation; headerParity?: boolean; sections:Array<[string,SectionFormat]>; styles: Array<[string,StyleDefinition]>; structure: string; drafts: Array<[string, TextOperation]>; selection?: TextSelection };
export interface DocumentView {
  recovery?(status: RecoveryStatus): void;
  composition?(active: boolean): void;
  history?(controls: HistoryControls): void;
  validation?(message: string): void;
  activity?(entry: ActivityEntry): void;
  render(snapshot: Snapshot, focus?: DraftFocus): void;
  status(message: string, state: 'pending' | 'committed' | 'failed' | 'conflicted'): void;
  error(message: string, details?: string): void;
  canSave(enabled: boolean): void;
}

// A failed write is never retried automatically. Keep drafts until the user
// explicitly reloads, including when the connection fails after a disk commit.
export class DocumentController {
  private snapshot?: Snapshot;
  private activitySequence = 0;
  private drafts = new Map<string, TextOperation>();
  private structureDraft?: StructureDraft;
  private expectedText = new Map<string, string>();
  private busy = false;
  private composing = false;
  private compositionSaves: Array<(saved: boolean) => void> = [];
  private blocked = true;
  private events?: Pick<EventSource, 'addEventListener' | 'onerror' | 'close'>;
  private disposed = false;
  private loading = false;
  private listDrafts=new Map<string,ListDefinition>();
  private standaloneDraft?:ReferenceOperation|CommentOperation|MathOperation|TableOperation|ParagraphMoveOperation|RevisionReviewOperation|ObjectTextOperation|RectangleOperation|TrackTextOperation;
  private imageDraft?:ImageOperation;
  private boundaryDraft?: BoundaryOperation;
  private headerParityDraft?: boolean;
  private sectionDrafts=new Map<string,SectionFormat>();
  private styleDrafts=new Map<string,StyleDefinition>();
  private undoDraft: DraftCheckpoint[] = [];
  private redoDraft: DraftCheckpoint[] = [];
  private lastTyping?: { key: string; time: number };

  private recovery?: DraftRecovery;

  constructor(private view: DocumentView) {}

  private connectRecovery(snapshot: Snapshot): void {
    if (!this.view.recovery || new URLSearchParams(window.location.search).get('document') !== snapshot.id) return;
    this.recovery ??= new DraftRecovery(snapshot.id, () => {
      if (!this.snapshot || this.composing || this.draftError()) return;
      return JSON.stringify({ sessionId: this.snapshot.id, baseRevision: this.snapshot.revision, operations: this.pendingOperations() });
    }, this.view.recovery);
    void this.recovery.load();
  }

  enableRecovery(): void { void this.recovery?.enable(); }
  removeRecovery(): void { void this.recovery?.remove(); }
  retryRecovery(): void { void this.recovery?.flush(); }

  hasStructuralDraft(): boolean { return Boolean(this.structureDraft?.hasChanges()); }

  hasDrafts(): boolean { return this.listDrafts.size>0 || this.standaloneDraft !== undefined || this.imageDraft !== undefined || this.boundaryDraft !== undefined || this.headerParityDraft !== undefined || this.sectionDrafts.size>0 || this.styleDrafts.size > 0 || this.drafts.size > 0 || Boolean(this.structureDraft?.hasChanges()) || this.busy || this.composing; }

  private checkpoint(selection?: TextSelection): DraftCheckpoint {
    return { lists:[...this.listDrafts], standalone:this.standaloneDraft, image:this.imageDraft, boundary: this.boundaryDraft, headerParity: this.headerParityDraft, sections:[...this.sectionDrafts],styles:[...this.styleDrafts],structure: this.structureDraft!.capture(), drafts: [...this.drafts], selection };
  }
  private remember(before: DraftCheckpoint | undefined, key?: string, now = Date.now()): void {
    if (before) this.undoDraft.push(before);
    this.lastTyping = key ? { key, time: now } : undefined;
    this.redoDraft = [];
    if (before) this.trimDraftHistory();
  }
  private trimDraftHistory(): void {
    while (this.undoDraft.length + this.redoDraft.length > 10 || [...this.undoDraft, ...this.redoDraft].reduce((n, item) => n + (item.structure.length + JSON.stringify(item.drafts).length + JSON.stringify(item.styles).length + JSON.stringify(item.sections).length) * 2, 0) > 32 * 1024 * 1024) {
      if (this.undoDraft.length) this.undoDraft.shift(); else this.redoDraft.shift();
    }
  }
  edit(operation: TextOperation, selection?: TextSelection): void {
    if (this.busy || this.loading || this.disposed || !this.structureDraft) return;
    if ((this.structureDraft.currentText(operation.targetId) ?? this.expectedText.get(operation.targetId)) === operation.text) return;
    const now = Date.now();
    const grouped = this.lastTyping?.key === operation.targetId && now - this.lastTyping.time <= 750;
    // The first edit already holds the undo state for this typing group.
    const before = grouped ? undefined : this.checkpoint(selection ?? {anchor:{runId:operation.targetId,offset:0},focus:{runId:operation.targetId,offset:0}});
    if (!this.structureDraft?.edit(operation.targetId, operation.text)) {
      const expectedText = this.expectedText.get(operation.targetId) ?? operation.expectedText;
      if (operation.text === expectedText) this.drafts.delete(operation.targetId);
      else this.drafts.set(operation.targetId, { ...operation, expectedText });
    }
    this.remember(before, operation.targetId, now);
    if (!this.blocked) this.view.status(this.hasDrafts() ? '미저장 변경' : '저장됨', this.hasDrafts() ? 'pending' : 'committed');
    this.updateSave();
  }

  changeStructure(action: StructureAction, runId: string, start: number, end: number, insertion?: string): boolean {
    if (this.busy || this.loading || this.disposed || this.boundaryDraft || this.imageDraft || this.standaloneDraft) return false;
    const before = this.structureDraft && this.checkpoint({anchor:{runId,offset:start},focus:{runId,offset:end}});
    const focus = this.structureDraft?.change(action, runId, start, end, insertion);
    if (!focus || !this.structureDraft) return false;
    this.remember(before!);
    this.view.render(this.preview(), focus);
    if (!this.blocked) this.view.status('미저장 변경', 'pending');
    this.updateSave();
    return true;
  }

  composition(active: boolean): void {
    this.composing = active;
    this.view.composition?.(active);
    this.updateSave();
    if (!active && this.compositionSaves.length) {
      const waiting = this.compositionSaves.splice(0);
      void this.save().then(saved => { for (const finish of waiting) finish(saved); });
    }
  }
  replaceSelection(selection: TextSelection, text: string, inline = false): boolean {
    if (this.busy || this.loading || this.disposed || this.boundaryDraft || this.imageDraft || this.standaloneDraft) return false;
    if (!inline && text==='\n' && selection.anchor.runId===selection.focus.runId && selection.anchor.offset===selection.focus.offset) {
      const paragraphs=this.structureDraft?.selectedParagraphs(selection)??[];
      const paragraph=paragraphs.length===1?paragraphs[0]:undefined;
      if(paragraph?.list && (paragraph.list.id || paragraph.list.key) && paragraph.runs.every(run=>run.text==='')) {
        return this.applyList(selection,{id:0,level:0});
      }
    }
    const before = this.structureDraft && this.checkpoint(selection);
    const focus = this.structureDraft?.replace(selection, text, inline);
    if (!focus || !this.structureDraft) return false;
    this.remember(before!);
    this.view.render(this.preview(), focus);
    if (!this.blocked) this.view.status('미저장 변경', 'pending');
    this.updateSave(); return true;
  }
  pasteRich(selection: TextSelection, paragraphs: ClipboardParagraph[]): boolean {
    if (this.busy || this.loading || this.disposed || this.composing || this.boundaryDraft || this.imageDraft || this.standaloneDraft) return false;
    const before = this.structureDraft && this.checkpoint(selection);
    const focus=this.structureDraft?.replaceRich(selection,paragraphs);
    if (!focus || !this.structureDraft) return false;
    this.remember(before!);
    this.view.render(this.preview(),focus);
    if (!this.blocked) this.view.status('미저장 변경','pending');
    this.updateSave();return true;
  }
  private preview():Snapshot {return {...this.structureDraft!.snapshot([...this.listDrafts.values()]),evenAndOddHeaders:this.headerParityDraft??this.snapshot?.evenAndOddHeaders,sections:this.sections()};}
  sections(){return previewSections(this.snapshot?.sections??[],this.sectionDrafts);}
  replaceSearchMatches(ranges:TextSelection[],replacement:string):boolean {
    if(this.blocked||this.loading||this.busy||this.disposed||this.composing||this.boundaryDraft||this.imageDraft||this.standaloneDraft||
      !this.structureDraft||!this.snapshot||ranges.length===0||ranges.length>200||replacement.length>200||validateRunText(replacement)) return false;
    const before=this.checkpoint();
    let focus:DraftFocus|undefined;
    // Search provides non-overlapping ranges in document order. Reversing keeps
    // offsets in preceding portions of a run valid until they are replaced.
    for(const range of [...ranges].reverse()){
      focus=this.structureDraft.replace(range,replacement,true);
      if(!focus){this.structureDraft.restore(before.structure);return false;}
    }
    if(validateDrafts(this.snapshot.id,this.snapshot.revision,this.pendingOperations())){
      this.structureDraft.restore(before.structure);
      return false;
    }
    this.remember(before);
    this.view.render(this.preview(),focus);
    this.view.status('미저장 변경','pending');
    this.updateSave();
    return true;
  }
  pendingTable(): boolean { return this.standaloneDraft?.type === 'update_table'; }
  editTable(operation: TableOperation): boolean { return this.editStandalone(operation); }
  pendingStandalone(): boolean { return Boolean(this.standaloneDraft); }
  editStandalone(operation:ReferenceOperation|CommentOperation|MathOperation|TableOperation|ParagraphMoveOperation|RevisionReviewOperation|ObjectTextOperation|RectangleOperation|TrackTextOperation): boolean {
    if(this.blocked||this.loading||this.disposed||!this.structureDraft||this.hasDrafts())return false;
    this.remember(this.checkpoint());
    this.standaloneDraft=operation;
    this.view.render(this.preview());
    this.view.status('미저장 변경','pending');
    this.updateSave();
    return true;
  }
  pendingImage(): boolean { return Boolean(this.imageDraft); }
  editImage(operation:ImageOperation): boolean {
    if(this.blocked||this.loading||this.disposed||!this.structureDraft||this.hasDrafts())return false;
    this.remember(this.checkpoint());
    this.imageDraft=operation;
    this.view.render(this.preview());
    this.view.status('미저장 변경','pending');
    this.updateSave();
    return true;
  }
  pendingBoundary(): BoundaryOperation | undefined { return this.boundaryDraft; }
  editBoundary(operation: BoundaryOperation): boolean {
    if (this.blocked || this.busy || this.loading || this.disposed || this.composing || !this.structureDraft ||
        this.hasDrafts()) return false;
    const before = this.checkpoint();
    this.boundaryDraft = operation;
    this.remember(before);
    this.view.render(this.preview());
    this.view.status('미저장 변경', 'pending');
    this.updateSave();
    return true;
  }
  editHeaderParity(enabled: boolean): boolean {
    if (this.blocked || this.busy || this.loading || this.disposed || this.composing ||
        !this.structureDraft || this.snapshot?.evenAndOddHeaders === undefined ||
        enabled === (this.headerParityDraft ?? this.snapshot.evenAndOddHeaders)) return false;
    const before = this.checkpoint();
    this.headerParityDraft = enabled === this.snapshot.evenAndOddHeaders ? undefined : enabled;
    this.remember(before);
    this.view.render(this.preview());
    this.view.status(this.hasDrafts() ? '미저장 변경' : '저장됨', this.hasDrafts() ? 'pending' : 'committed');
    this.updateSave();
    return true;
  }
  editSection(id:string,format:SectionFormat):boolean {
   if(this.blocked||this.busy||this.loading||this.disposed||this.composing||this.boundaryDraft||this.imageDraft||this.standaloneDraft||!this.structureDraft)return false;
   const original=this.snapshot?.sections?.find(section=>section.id===id);if(!original?.editable||!Object.keys(format).length||!sectionFormatSchema.safeParse(format).success)return false;
   const previous=this.sectionDrafts.get(id),merged={...previous,...format};
   if(format.headerFooters){const slots=new Map((previous?.headerFooters??[]).map(slot=>[slot.kind+':'+slot.variant,slot]));for(const slot of format.headerFooters)slots.set(slot.kind+':'+slot.variant,slot);merged.headerFooters=[...slots.values()];}
   if(validateSection(previewSection(original,merged)))return false;
   const before=this.checkpoint();this.sectionDrafts.set(id,merged);this.remember(before);this.view.render(this.preview());this.view.status('미저장 변경','pending');this.updateSave();return true;
  }
  styles():StyleInfo[] {
    const styles=new Map((this.snapshot?.styles??[]).map(style=>[style.id,style]));
    for(const definition of this.styleDrafts.values()) {
      const original=styles.get(definition.id),paragraph={...original?.paragraph,...definition.paragraph};
      if(definition.paragraph?.firstLine!==undefined)delete paragraph.hanging;
      if(definition.paragraph?.hanging!==undefined)delete paragraph.firstLine;
      styles.set(definition.id,{id:definition.id,outlineLevel:original?.outlineLevel,name:definition.name??original?.name??definition.id,kind:definition.kind,editable:original?.editable??true,
        basedOn:definition.basedOn??original?.basedOn,run:{...(definition.run?.reset?{}:original?.run),...definition.run},paragraph});
    }
    return [...styles.values()];
  }
  editStyle(definition:StyleDefinition):boolean {
    if(this.busy||this.loading||this.disposed||this.composing||!this.structureDraft)return false;
    const before=this.checkpoint();const previous=this.styleDrafts.get(definition.id);
    const paragraph=definition.paragraph?{...previous?.paragraph,...definition.paragraph}:previous?.paragraph;
    if(definition.paragraph?.firstLine!==undefined&&paragraph)delete paragraph.hanging;
    if(definition.paragraph?.hanging!==undefined&&paragraph)delete paragraph.firstLine;
    this.styleDrafts.set(definition.id,{...previous,...definition,run:definition.run?.reset?definition.run:definition.run?{...previous?.run,...definition.run}:previous?.run,paragraph});
    this.remember(before);if(!this.blocked)this.view.status('미저장 변경','pending');this.updateSave();return true;
  }
  hasStyleDrafts():boolean {return this.styleDrafts.size>0 || this.pendingOperations().some(op=>op.type==='replace_paragraphs'&&op.paragraphs.some(p=>p.format?.styleId!==undefined||p.runs.some(r=>r.style?.styleId!==undefined)));}
  formatParagraph(selection:TextSelection,format:ParagraphFormat):boolean {
    if(this.busy||this.loading||this.disposed||this.composing||!this.structureDraft)return false;
    const before=this.checkpoint(selection),focus=this.structureDraft.formatParagraph(selection,format);
    if(!focus)return false;this.remember(before);this.view.render(this.preview(),focus);
    if(!this.blocked)this.view.status('미저장 변경','pending');this.updateSave();return true;
  }
  applyList(selection: TextSelection, list: ParagraphList | Omit<ListDefinition,'key'>): boolean {
    if(this.busy||this.loading||this.disposed||this.composing||!this.structureDraft)return false;
    const before=this.checkpoint(selection);
    let reference:ParagraphList;
    let definition:ListDefinition|undefined;
    if ('kind' in list) {
      definition={key:`list_${++this.activitySequence}`,kind:list.kind,start:list.start??1};
      reference={key:definition.key,level:0};
    } else reference=list;
    const focus=this.structureDraft.formatParagraph(selection,{list:reference});
    if(!focus)return false;
    if(definition)this.listDrafts.set(definition.key,definition);
    this.remember(before);
    this.view.render(this.preview(),focus);
    if(!this.blocked)this.view.status('미저장 변경','pending');
    this.updateSave();return true;
  }
  changeListLevel(selection: TextSelection, delta: number): boolean {
    if(this.busy||this.loading||this.disposed||this.composing||!this.structureDraft)return false;
    const paragraphs=this.structureDraft.selectedParagraphs(selection);
    if(!paragraphs.length || paragraphs.some(p=>!p.list||!p.list.id&&!p.list.key||(p.list.level??0)+delta<0||(p.list.level??0)+delta>8))return false;
    const before=this.checkpoint(selection);
    for(const p of paragraphs) {
      const point={runId:p.runs[0]!.id,offset:0};
      this.structureDraft.formatParagraph({anchor:point,focus:point},{list:{...p.list,level:(p.list!.level??0)+delta}});
    }
    this.remember(before);
    this.view.render(this.preview(),{runId:selection.focus.runId,start:selection.focus.offset,end:selection.focus.offset,selection});
    if(!this.blocked)this.view.status('미저장 변경','pending');
    this.updateSave();return true;
  }
  selectedParagraphs(selection:TextSelection) {return this.structureDraft?.selectedParagraphs(selection)??[];}
  format(selection: TextSelection, style: InlineStyle): boolean {
    if(this.busy||this.loading||this.disposed||this.composing||!this.structureDraft)return false;
    const before=this.checkpoint(selection),focus=this.structureDraft.format(selection,style);
    if(!focus)return false;
    this.remember(before);this.view.render(this.preview(),focus);
    if(!this.blocked)this.view.status('미저장 변경','pending');this.updateSave();return true;
  }
  selectedRuns(selection:TextSelection){return this.structureDraft?.selectedRuns(selection)??[];}
  richSelection(selection: TextSelection): ClipboardParagraph[] | undefined {return this.structureDraft?.selectedRich(selection);}
  deleteSelection(selection: TextSelection, direction: 'backward' | 'forward', word: boolean): boolean {
    const range = this.structureDraft?.deletion(selection, direction, word ? 'word' : 'grapheme');
    return Boolean(range && this.replaceSelection(range, ''));
  }
  refreshSelection(selection: TextSelection): void {
    if (this.structureDraft) this.view.render(this.preview(), { runId: selection.focus.runId, start: selection.focus.offset, end: selection.focus.offset, selection });
  }
  selectionText(selection: TextSelection): string | undefined { return this.structureDraft?.selectedText(selection); }

  private pendingOperations(): Operation[] {
    const text = [...this.drafts.values()].filter(op => !this.structureDraft?.ownsOriginal(op.targetId));
    const parity: Operation[] = this.headerParityDraft === undefined ? [] :
      [{type:'update_header_parity',evenAndOddHeaders:this.headerParityDraft}];
    return [...(this.standaloneDraft?[this.standaloneDraft]:[]),...(this.imageDraft?[this.imageDraft]:[]),...this.boundaryDraft ? [this.boundaryDraft] : [],...parity,...[...this.listDrafts.values()].map(listDefinition=>({type:"define_list" as const,listDefinition})),...[...this.sectionDrafts].map(([sectionId,section])=>({type:'update_section' as const,sectionId,section})),...[...this.styleDrafts.values()].map(styleDefinition=>({type:'define_style' as const,styleDefinition})),...text, ...this.structureDraft?.operations() ?? []];
  }
  private resetDraft(snapshot: Snapshot): void {
    this.listDrafts.clear();this.drafts.clear();this.styleDrafts.clear();this.sectionDrafts.clear();this.headerParityDraft=undefined;this.boundaryDraft=undefined;this.imageDraft=undefined;this.standaloneDraft=undefined;
    this.undoDraft = []; this.redoDraft = []; this.lastTyping = undefined;
    this.structureDraft = new StructureDraft(snapshot);
    this.expectedText.clear();
    for (const block of snapshot.blocks) {
      const paragraphs = block.type === 'paragraph' ? [block.paragraph] : block.rows.flatMap(r => r.cells.flatMap(c => c.paragraphs));
      for (const p of paragraphs) for (const run of p.runs) this.expectedText.set(run.id, run.text);
    }
  }

  async load(observed?: { sequence: number; revision: number; actor?: string; textOnly?:boolean }): Promise<void> {
    if (this.busy || this.loading || this.disposed || this.composing) return;
    this.loading = true;
    this.disconnect();
    this.blocked = true;
    this.updateSave();
    this.view.status('문서를 불러오는 중', 'pending');
    try {
      const snapshot = await this.request('/api/docx');
      if (this.disposed) return;
      if (observed && this.snapshot && snapshot.revision === observed.revision && observed.actor === 'ai') {
        const changes = compareParagraphs(this.snapshot, snapshot, observed.textOnly===true).slice(0, 60);
        for (const [index, change] of changes.entries()) {
          this.view.activity?.({ id: index === 0 ? `event:${observed.sequence}` : `event:${observed.sequence}:${change.key}`, actor: 'ai', state: 'committed',
            title: '텍스트 변경', location: change.label, revision: observed.revision,
            documentChange: { session: snapshot.id, key: change.key, anchor:change.anchor, before: change.before, after: change.after } });
        }
      }
      this.snapshot = snapshot;
      this.resetDraft(snapshot);
      this.blocked = false;
      this.view.error('');
      this.view.render(snapshot);
      this.view.status('저장됨', 'committed');
      this.connect(snapshot);
      this.connectRecovery(snapshot);
    } catch (error) { this.fail(error); }
    finally { this.loading = false; this.updateSave(); }
  }

  async save(): Promise<boolean> {
    if (this.composing && !this.blocked && !this.disposed) {
      return new Promise(resolve => { this.compositionSaves.push(resolve); });
    }
    if (this.blocked || this.busy || this.composing || this.loading || this.disposed || !this.snapshot || this.draftError()) return false;
    const hasChanges = this.hasDrafts();
    this.busy = true;
    this.disconnect();
    this.updateSave();
    this.view.status(hasChanges ? '저장 중' : source('checkingSave'), 'pending');
    const activityId = `local:${++this.activitySequence}`;
    if (hasChanges) this.view.activity?.({ id: activityId, actor: 'human', state: 'pending', title: `본문·표 ${this.pendingOperations().length}개 수정` });
    try {
      let snapshot = this.snapshot;
      if (hasChanges) {
        if (this.recovery) {
          await this.recovery.flush();
          if (this.disposed) return false;
        }
        snapshot = await this.request('/api/docx/operations', {
          method: 'POST', headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ sessionId: snapshot.id, baseRevision: snapshot.revision, operations: this.pendingOperations() }),
        });
      } else { await confirmSavedDocument(snapshot); }
      if (this.disposed) return false;
      this.snapshot = snapshot;
      if (hasChanges) {
        this.view.activity?.({ id: activityId, actor: 'human', state: 'committed', title: '본문·표 텍스트 저장', revision: snapshot.revision });
        this.resetDraft(snapshot);
        this.view.render(snapshot);
      }
      this.view.status('저장됨', 'committed');
      this.connect(snapshot);
      return true;
    } catch (error) {
      if (hasChanges) this.view.activity?.({ id: activityId, actor: 'human', state: 'uncertain', title: '저장 결과 확인 필요' });
      this.fail(error);
      return false;
    }
    finally { this.busy = false; this.updateSave(); }
  }

  async restore(direction: 'undo' | 'redo', selection?: TextSelection): Promise<boolean> {
    if (this.blocked || this.busy || this.loading || this.composing || this.disposed || !this.snapshot || !this.structureDraft) return false;
    const from = direction === 'undo' ? this.undoDraft : this.redoDraft;
    const to = direction === 'undo' ? this.redoDraft : this.undoDraft;
    if (from.length) {
      const saved = from.pop()!;
      to.push(this.checkpoint(selection)); this.trimDraftHistory();
      this.structureDraft.restore(saved.structure); this.drafts = new Map(saved.drafts); this.styleDrafts=new Map(saved.styles); this.sectionDrafts=new Map(saved.sections); this.headerParityDraft=saved.headerParity; this.boundaryDraft=saved.boundary; this.imageDraft=saved.image; this.standaloneDraft=saved.standalone; this.listDrafts=new Map(saved.lists); this.lastTyping = undefined;
      const point = saved.selection?.focus;
      this.view.render(this.preview(), point ? {runId:point.runId,start:point.offset,end:point.offset,selection:saved.selection} : undefined);
      this.view.status(this.hasDrafts() ? '미저장 변경' : '저장됨', this.hasDrafts() ? 'pending' : 'committed');
      this.updateSave(); return true;
    }
    // A local draft must never cause a second undo against the saved document.
    if (this.hasDrafts() || !this.snapshot.history?.[direction]) return false;
    this.busy = true; this.disconnect(); this.updateSave(); this.view.status('저장 중','pending');
    const activityId = `local:${++this.activitySequence}`;
    this.view.activity?.({id:activityId,actor:'human',state:'pending',title:direction==='undo'?'실행 취소':'다시 실행'});
    try {
      const next = await this.request('/api/docx/history',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({sessionId:this.snapshot.id,baseRevision:this.snapshot.revision,direction})});
      if (this.disposed) return false;
      this.snapshot=next;this.resetDraft(next);this.view.render(next);this.view.status('저장됨','committed');
      this.view.activity?.({id:activityId,actor:'human',state:'committed',title:direction==='undo'?'실행 취소':'다시 실행',revision:next.revision});
      this.connect(next);return true;
    } catch(error) {
      this.view.activity?.({id:activityId,actor:'human',state:'uncertain',title:'이력 복원 결과 확인 필요'});this.fail(error);return false;
    } finally {this.busy=false;this.updateSave();}
  }

  async exportCopy(name: string): Promise<DownloadedFile | undefined> {
    if (this.disposed || this.busy || this.loading || this.composing || !this.snapshot || this.draftError()) return;
    this.busy=true; this.updateSave();
    try {
      return await exportDOCXCopy({sessionId:this.snapshot.id,baseRevision:this.snapshot.revision,operations:this.pendingOperations(),name});
    } finally {this.busy=false;this.updateSave();}
  }

  async download(): Promise<DownloadedFile | undefined> {
    if (!await this.save() || !this.snapshot || this.disposed || this.blocked || this.hasDrafts()) return;
    const checkpoint = this.snapshot;
    this.busy = true;
    this.disconnect();
    this.updateSave();
    this.view.status(source('preparingDownload'), 'pending');
    try {
      const file = await downloadSavedDocument(checkpoint, 'docx');
      if (this.disposed || this.blocked || this.drafts.size || this.snapshot.id !== checkpoint.id || this.snapshot.revision !== checkpoint.revision) return;
      this.view.status('저장됨', 'committed');
      return file;
    } catch (error) {
      if (error instanceof DocumentFileError && error.status === 409) this.fail(error);
      else { this.view.status('저장됨', 'committed'); throw error; }
    } finally {
      this.busy = false;
      this.updateSave();
      if (!this.blocked && !this.disposed && this.snapshot) this.connect(this.snapshot);
    }
  }

  private async request(url: string, init?: RequestInit): Promise<Snapshot> {
    const response = await fetch(scopedDocumentURL(url), { ...init, credentials: 'same-origin', cache: 'no-store' });
    if (!response.ok) {
      let details = '';
      try { const parsed = errorResponseSchema.safeParse(await response.json()); if (parsed.success) details = parsed.data.error; } catch { /* Keep the recovery message if the response is unreadable. */ }
      const error = new Error(response.status === 409
      ? '문서 리비전 충돌 또는 외부 변경이 있습니다. 입력 내용을 복사한 뒤 다시 불러오세요.'
      : '문서를 읽거나 저장하지 못했습니다. 입력 내용을 복사한 뒤 다시 불러오세요.') as Error & { details?: string; conflict?: boolean };
      error.details = details; error.conflict = response.status === 409; throw error;
    }
    return snapshotSchema.parse(await response.json());
  }

  private connect(snapshot: Snapshot): void {
    this.disconnect();
    const events = documentEvents(scopedDocumentURL(`/api/docx/events?session=${encodeURIComponent(snapshot.id)}&after=${snapshot.sequence}`));
    this.events = events;
    events.addEventListener('docx', (raw) => {
      if (this.events !== events || this.disposed) return;
      try {
        const event = eventSchema.parse(JSON.parse((raw as MessageEvent<string>).data));
        if (event.type === 'docx.committed' && event.revision > (this.snapshot?.revision ?? 0)) {
          this.view.activity?.({ id: `event:${event.sequence}`, actor: event.actor === 'ai' ? 'ai' : 'human', state: 'committed', title: '본문·표 텍스트 저장', revision: event.revision });
        }
        if (event.type === 'docx.reload' || event.revision > (this.snapshot?.revision ?? 0)) {
          if (this.hasDrafts()) {
            this.fail(new Error('다른 편집이 저장되었습니다. 입력 내용을 복사한 뒤 다시 불러오세요.'), true);
          } else { void this.load(event.type === 'docx.committed' ? event : undefined); }
        }
      } catch (error) { this.fail(error); }
    });
    events.onerror = () => {
      if (this.events === events && !this.disposed) this.fail(new Error('문서 연결이 끊겼습니다. 입력 내용을 복사한 뒤 다시 불러오세요.'));
    };
  }

  private fail(error: unknown, conflict = false): void {
    this.blocked = true;
    this.cancelCompositionSaves();
    this.disconnect();
    const conflicted = conflict || error instanceof Error && Boolean((error as Error & { conflict?: boolean }).conflict);
    this.view.status(conflicted ? '편집 충돌 · 다시 불러오기 필요' : '편집 중단 · 다시 불러오기 필요', conflicted ? 'conflicted' : 'failed');
    this.view.error(error instanceof Error ? error.message : '문서 처리에 실패했습니다.', error instanceof Error ? ((error as Error & { details?: string }).details || error.message) : String(error));
    this.updateSave();
  }
  private disconnect(): void { this.events?.close(); this.events = undefined; }
  private draftError(): string {
    const styles=new Map(this.styles().map(style=>[style.id,style]));
    for(const changed of this.styleDrafts.values()) {
      const visited=new Set<string>();let id:string|undefined=changed.id;
      while(id&&styles.has(id)) {
        if(visited.has(id)||visited.size>=32)return '스타일 상속이 순환하거나 너무 깊습니다. 기준 스타일을 수정하세요.';
        visited.add(id);id=styles.get(id)?.basedOn;
      }
    }
    return validateDrafts(this.snapshot?.id ?? '', this.snapshot?.revision ?? 0, this.pendingOperations());
  }
  private updateSave(): void {
    const error = this.draftError();
    const available = !this.blocked && !this.busy && !this.loading && !this.composing && !this.disposed;
    const actor = (value?: string) => value === 'ai' ? 'AI 확정 작업' : '사람 확정 작업';
    this.view.history?.({
      undo: available && (this.undoDraft.length > 0 || !this.hasDrafts() && Boolean(this.snapshot?.history?.undo)),
      redo: available && (this.redoDraft.length > 0 || !this.hasDrafts() && Boolean(this.snapshot?.history?.redo)),
      undoLabel: this.undoDraft.length ? '미저장 초안' : actor(this.snapshot?.history?.undoActor),
      redoLabel: this.redoDraft.length ? '미저장 초안' : actor(this.snapshot?.history?.redoActor),
    });
    if (!this.composing && !this.loading && !this.busy && !error) this.recovery?.schedule();
    this.view.validation?.(error);
    this.view.canSave(!this.blocked && !this.busy && this.hasDrafts() && !this.composing && !error);
  }
  private cancelCompositionSaves(): void {
    for (const finish of this.compositionSaves.splice(0)) finish(false);
  }
  dispose(): void { if (this.disposed) return; this.recovery?.dispose(); this.disposed = true; this.cancelCompositionSaves(); this.disconnect(); }
}
