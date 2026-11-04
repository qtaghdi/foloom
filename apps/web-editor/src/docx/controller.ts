import { compareParagraphs } from './paragraphs';
import { validateDrafts } from './validation';
import type { ActivityEntry } from '../ui/activity';
import { eventSchema, snapshotSchema, type Snapshot, type Operation } from './contracts';

export interface DocumentView {
  validation?(message: string): void;
  activity?(entry: ActivityEntry): void;
  render(snapshot: Snapshot): void;
  status(message: string, state: 'pending' | 'committed' | 'failed' | 'conflicted'): void;
  error(message: string): void;
  canSave(enabled: boolean): void;
}

// A failed write is never retried automatically. Keep drafts until the user
// explicitly reloads, including when the connection fails after a disk commit.
export class DocumentController {
  private snapshot?: Snapshot;
  private activitySequence = 0;
  private drafts = new Map<string, Operation>();
  private busy = false;
  private blocked = true;
  private events?: EventSource;
  private disposed = false;
  private loading = false;

  constructor(private view: DocumentView) {}

  hasDrafts(): boolean { return this.drafts.size > 0 || this.busy; }

  edit(operation: Operation): void {
    if (this.busy || this.loading) return;
    if (operation.text === operation.expectedText) this.drafts.delete(operation.targetId);
    else this.drafts.set(operation.targetId, operation);
    if (!this.blocked) this.view.status(this.drafts.size ? '미저장 변경' : '저장됨', this.drafts.size ? 'pending' : 'committed');
    this.updateSave();
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
      this.drafts.clear();
      this.blocked = false;
      this.view.error('');
      this.view.render(snapshot);
      this.view.status('저장됨', 'committed');
      this.connect(snapshot);
    } catch (error) { this.fail(error); }
    finally { this.loading = false; this.updateSave(); }
  }

  async save(): Promise<void> {
    if (this.blocked || this.busy || !this.snapshot || !this.drafts.size || this.draftError()) return;
    this.busy = true;
    this.disconnect();
    this.updateSave();
    this.view.status('저장 중', 'pending');
    const activityId = `local:${++this.activitySequence}`;
    this.view.activity?.({ id: activityId, actor: 'human', state: 'pending', title: `본문·표 ${this.drafts.size}개 수정` });
    try {
      const snapshot = await this.request('/api/docx/operations', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sessionId: this.snapshot.id, baseRevision: this.snapshot.revision, operations: [...this.drafts.values()] }),
      });
      if (this.disposed) return;
      this.snapshot = snapshot;
      this.view.activity?.({ id: activityId, actor: 'human', state: 'committed', title: '본문·표 텍스트 저장', revision: snapshot.revision });
      this.drafts.clear();
      this.view.render(snapshot);
      this.view.status('저장됨', 'committed');
      this.connect(snapshot);
    } catch (error) {
      this.view.activity?.({ id: activityId, actor: 'human', state: 'uncertain', title: '저장 결과 확인 필요' });
      this.fail(error);
    }
    finally { this.busy = false; this.updateSave(); }
  }

  private async request(url: string, init?: RequestInit): Promise<Snapshot> {
    const response = await fetch(url, { ...init, credentials: 'same-origin', cache: 'no-store' });
    if (!response.ok) throw new Error(response.status === 409
      ? '문서 리비전 충돌 또는 외부 변경이 있습니다. 입력 내용을 복사한 뒤 다시 불러오세요.'
      : '문서를 읽거나 저장하지 못했습니다. 입력 내용을 복사한 뒤 다시 불러오세요.');
    return snapshotSchema.parse(await response.json());
  }

  private connect(snapshot: Snapshot): void {
    this.disconnect();
    const events = new EventSource(`/api/docx/events?session=${encodeURIComponent(snapshot.id)}&after=${snapshot.sequence}`);
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
    this.view.error(error instanceof Error ? error.message : '문서 처리에 실패했습니다.');
    this.updateSave();
  }
  private disconnect(): void { this.events?.close(); this.events = undefined; }
  private draftError(): string {
    return validateDrafts(this.snapshot?.id ?? '', this.snapshot?.revision ?? 0, [...this.drafts.values()]);
  }
  private updateSave(): void {
    const error = this.draftError();
    this.view.validation?.(error);
    this.view.canSave(!this.blocked && !this.busy && this.drafts.size > 0 && !error);
  }
  dispose(): void { this.disposed = true; this.disconnect(); }
}
