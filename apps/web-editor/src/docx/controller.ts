import { confirmSavedDocument, downloadSavedDocument, DocumentFileError, type DownloadedFile } from '../transport/document-files';
import { source } from '../i18n/index';
import { errorResponseSchema } from '../transport/schemas';
import { documentEvents } from '../transport/document-events';
import { scopedDocumentURL } from '../transport/document-scope';
import type { TextSelection } from './selection';
import { StructureDraft } from './structure-draft';
import { compareParagraphs } from './paragraphs';
import { validateDrafts } from './validation';
import type { ActivityEntry } from '../ui/activity';
import { eventSchema, snapshotSchema, type Snapshot, type Operation, type TextOperation, type DraftFocus, type StructureAction } from './contracts';

export interface DocumentView {
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
  private blocked = true;
  private events?: Pick<EventSource, 'addEventListener' | 'onerror' | 'close'>;
  private disposed = false;
  private loading = false;

  constructor(private view: DocumentView) {}

  hasDrafts(): boolean { return this.drafts.size > 0 || Boolean(this.structureDraft?.hasChanges()) || this.busy || this.composing; }

  edit(operation: TextOperation): void {
    if (this.busy || this.loading) return;
    if (!this.structureDraft?.edit(operation.targetId, operation.text)) {
      const expectedText = this.expectedText.get(operation.targetId) ?? operation.expectedText;
      if (operation.text === expectedText) this.drafts.delete(operation.targetId);
      else this.drafts.set(operation.targetId, { ...operation, expectedText });
    }
    if (!this.blocked) this.view.status(this.hasDrafts() ? '미저장 변경' : '저장됨', this.hasDrafts() ? 'pending' : 'committed');
    this.updateSave();
  }

  changeStructure(action: StructureAction, runId: string, start: number, end: number, insertion?: string): boolean {
    if (this.busy || this.loading || this.disposed) return false;
    const focus = this.structureDraft?.change(action, runId, start, end, insertion);
    if (!focus || !this.structureDraft) return false;
    this.view.render(this.structureDraft.snapshot(), focus);
    if (!this.blocked) this.view.status('미저장 변경', 'pending');
    this.updateSave();
    return true;
  }

  composition(active: boolean): void { this.composing = active; this.updateSave(); }
  replaceSelection(selection: TextSelection, text: string, inline = false): boolean {
    if (this.busy || this.loading || this.disposed) return false;
    const focus = this.structureDraft?.replace(selection, text, inline);
    if (!focus || !this.structureDraft) return false;
    this.view.render(this.structureDraft.snapshot(), focus);
    if (!this.blocked) this.view.status('미저장 변경', 'pending');
    this.updateSave(); return true;
  }
  deleteSelection(selection: TextSelection, direction: 'backward' | 'forward', word: boolean): boolean {
    const range = this.structureDraft?.deletion(selection, direction, word ? 'word' : 'grapheme');
    return Boolean(range && this.replaceSelection(range, ''));
  }
  refreshSelection(selection: TextSelection): void {
    if (this.structureDraft) this.view.render(this.structureDraft.snapshot(), { runId: selection.focus.runId, start: selection.focus.offset, end: selection.focus.offset });
  }
  selectionText(selection: TextSelection): string | undefined { return this.structureDraft?.selectedText(selection); }

  private pendingOperations(): Operation[] {
    const text = [...this.drafts.values()].filter(op => !this.structureDraft?.ownsOriginal(op.targetId));
    return [...text, ...this.structureDraft?.operations() ?? []];
  }
  private resetDraft(snapshot: Snapshot): void {
    this.drafts.clear();
    this.structureDraft = new StructureDraft(snapshot);
    this.expectedText.clear();
    for (const block of snapshot.blocks) {
      const paragraphs = block.type === 'paragraph' ? [block.paragraph] : block.rows.flatMap(r => r.cells.flatMap(c => c.paragraphs));
      for (const p of paragraphs) for (const run of p.runs) this.expectedText.set(run.id, run.text);
    }
  }

  async load(observed?: { sequence: number; revision: number; actor?: string }): Promise<void> {
    if (this.busy || this.loading || this.disposed) return;
    this.loading = true;
    this.disconnect();
    this.blocked = true;
    this.updateSave();
    this.view.status('문서를 불러오는 중', 'pending');
    try {
      const snapshot = await this.request('/api/docx');
      if (this.disposed) return;
      if (observed && this.snapshot && snapshot.revision === observed.revision && observed.actor === 'ai') {
        const changes = compareParagraphs(this.snapshot, snapshot).slice(0, 60);
        for (const [index, change] of changes.entries()) {
          this.view.activity?.({ id: index === 0 ? `event:${observed.sequence}` : `event:${observed.sequence}:${change.key}`, actor: 'ai', state: 'committed',
            title: '텍스트 변경', location: change.label, revision: observed.revision,
            documentChange: { session: snapshot.id, key: change.key, before: change.before, after: change.after } });
        }
      }
      this.snapshot = snapshot;
      this.resetDraft(snapshot);
      this.blocked = false;
      this.view.error('');
      this.view.render(snapshot);
      this.view.status('저장됨', 'committed');
      this.connect(snapshot);
    } catch (error) { this.fail(error); }
    finally { this.loading = false; this.updateSave(); }
  }

  async save(): Promise<boolean> {
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
      : '문서를 읽거나 저장하지 못했습니다. 입력 내용을 복사한 뒤 다시 불러오세요.') as Error & { details?: string };
      error.details = details; throw error;
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
            this.fail(new Error('다른 편집이 저장되었습니다. 입력 내용을 복사한 뒤 다시 불러오세요.'));
          } else { void this.load(event.type === 'docx.committed' ? event : undefined); }
        }
      } catch (error) { this.fail(error); }
    });
    events.onerror = () => {
      if (this.events === events && !this.disposed) this.fail(new Error('문서 연결이 끊겼습니다. 입력 내용을 복사한 뒤 다시 불러오세요.'));
    };
  }

  private fail(error: unknown): void {
    this.blocked = true;
    this.disconnect();
    this.view.status('편집 중단 · 다시 불러오기 필요', 'failed');
    this.view.error(error instanceof Error ? error.message : '문서 처리에 실패했습니다.', error instanceof Error ? ((error as Error & { details?: string }).details || error.message) : String(error));
    this.updateSave();
  }
  private disconnect(): void { this.events?.close(); this.events = undefined; }
  private draftError(): string {
    return validateDrafts(this.snapshot?.id ?? '', this.snapshot?.revision ?? 0, this.pendingOperations());
  }
  private updateSave(): void {
    const error = this.draftError();
    this.view.validation?.(error);
    this.view.canSave(!this.blocked && !this.busy && this.hasDrafts() && !this.composing && !error);
  }
  dispose(): void { this.disposed = true; this.disconnect(); }
}
