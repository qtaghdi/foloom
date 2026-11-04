import { createSignal } from 'solid-js';
import type { FeatureWarning, WorkbookEvent, SelectionChange } from '../contracts';
import { featureMessage, problemMessage } from './messages';
import { createActivityModel, type ActivityEntry } from './activity';

export type RevisionState = 'ready' | 'pending' | 'success' | 'error' | 'conflict' | 'recovery';
export interface StatusPanelState { title: string; message: string; dismissible: boolean; reload?: () => void }
export interface WorkbookViewState {
  name: string; message: string; revision: number; status: RevisionState;
  presence?: string;
  warning?: StatusPanelState; error?: StatusPanelState;
}

// Controllers publish presentation state. Only the engine owns the mounted grid DOM.
export class AppView {
  spreadsheet!: HTMLElement;
  readonly activity = createActivityModel();
  recordActivity(entry: ActivityEntry): void { if (!this.disposed) this.activity.record(entry); }
  private readonly signal = createSignal<WorkbookViewState>({ name: '문서를 여는 중', message: '연결 중', revision: 0, status: 'pending' });
  readonly state = this.signal[0];
  private disposed = false;
  private presenceTimer: number | undefined;
  private patch(update: Partial<WorkbookViewState>): void { if (this.disposed) return; this.signal[1]((state) => ({ ...state, ...update })); }
  private locateListener?: (selection: SelectionChange) => boolean;
  onLocate(listener: (selection: SelectionChange) => boolean): void { this.locateListener = listener; }
  locate(target: NonNullable<ActivityEntry['sheetTarget']>): void {
    if (['pending', 'conflict', 'recovery'].includes(this.state().status)) return;
    const narrow = window.matchMedia('(max-width: 640px)').matches;
    if (narrow && this.activity.open()) this.activity.toggle();
    queueMicrotask(() => {
      if (this.disposed) return;
      if (this.locateListener?.({ ...target, state: 'selecting' })) this.spreadsheet.focus({ preventScroll: true });
      else this.showError('기록된 셀 범위를 열 수 없습니다.', this.state().revision);
    });
  }
  setWorkbookName(name: string): void { this.patch({ name }); }
  showWarnings(warnings: FeatureWarning[]): void {
    this.patch({ warning: warnings.length ? { title: '호환성 안내', message: warnings.map(featureMessage).join('\n\n'), dismissible: true } : undefined });
  }
  dismiss(kind: 'warning' | 'error'): void {
    if (this.state()[kind]?.dismissible) { this.patch({ [kind]: undefined }); this.spreadsheet.focus({ preventScroll: true }); }
  }
  showRevision(message: string, revision: number, status: RevisionState = 'ready'): void {
    this.patch({ message: translateStatus(message), revision, status });
  }
  showAIPresence(event: WorkbookEvent, revision: number): void {
    if (event.type === 'workbook.reload') { this.showRevision('AI 변경 반영됨', revision); return; }
    window.clearTimeout(this.presenceTimer);
    const selection = event.type === 'cell.typing' || event.type === 'cell.commit' ? event.cell
      : 'range' in event ? event.range : `${event.index}:${event.index + event.count - 1}`;
    const location = `${event.sheet}!${selection}`;
    this.patch({ presence: event.type === 'presence.update' ? `AI 선택: ${location}` : event.type === 'cell.typing' ? `AI 편집 중: ${location}` : `AI 변경 저장: ${location}` });
    if (event.type !== 'presence.update' && event.type !== 'cell.typing') {
      this.showRevision('AI 변경 저장됨', revision, 'success');
      this.presenceTimer = window.setTimeout(() => this.patch({ presence: undefined }), 4000);
    }
  }
  showError(error: unknown, revision: number): void {
    this.patch({ error: { title: '작업 실패', message: `${problemMessage(error instanceof Error ? error.message : String(error))} 연결과 문서 접근 상태를 확인하세요.`, dismissible: true } });
    this.showRevision('작업 실패', revision, 'error'); this.focusError();
  }
  showRecovery(message: string, revision: number): void {
    this.activity.markUncertain();
    const conflict = /conflict|changed|external|newer|busy/i.test(message);
    const title = conflict ? '충돌 감지' : '복구 필요';
    this.patch({ error: { title, message: `${problemMessage(message)} 편집을 중단했습니다. 미저장 입력을 복사한 뒤 저장된 문서를 다시 불러오세요. 결과가 불확실한 저장은 반복하지 마세요.`, dismissible: false, reload: () => window.location.reload() } });
    this.showRevision(title, revision, conflict ? 'conflict' : 'recovery'); this.focusError();
  }
  private focusError(): void { queueMicrotask(() => { if (!this.disposed) document.getElementById('fatal-error')?.focus({ preventScroll: true }); }); }
  dispose(): void { this.disposed = true; window.clearTimeout(this.presenceTimer); }
}
function translateStatus(message: string): string {
  const labels: Record<string, string> = { Ready: '편집 준비됨', Saved: '저장됨', 'Remote change saved': '다른 편집 저장됨', 'Nothing to undo': '실행 취소할 변경 없음', 'Nothing to redo': '다시 실행할 변경 없음', 'Undoing change…': '실행 취소 중', 'Redoing change…': '다시 실행 중', 'Ready with a bounded initial data window': '필요한 범위부터 불러왔습니다' };
  return labels[message] ?? message.replace(/^Saving (.*)…$/, '$1 저장 중').replace(/^Loaded (.*)$/, '$1 불러옴');
}
