import type { FeatureWarning, HistoryDirection, WorkbookEvent } from '../contracts';

type RevisionState = 'ready' | 'pending' | 'success' | 'error' | 'conflict' | 'recovery';

export class AppView {
  readonly spreadsheet: HTMLElement;
  private readonly workbookName: HTMLElement;
  private readonly revisionStatus: HTMLElement;
  private readonly presence: HTMLElement;
  private readonly presenceText: HTMLElement;
  private readonly errorPanel: HTMLElement;
  private readonly warningPanel: HTMLElement;
  private readonly undoButton: HTMLButtonElement;
  private readonly redoButton: HTMLButtonElement;
  private presenceTimer: number | undefined;
  private historyListener: ((direction: HistoryDirection) => void) | undefined;

  constructor(document: Document) {
    this.workbookName = requiredElement(document, 'workbook-name');
    this.revisionStatus = requiredElement(document, 'revision-status');
    this.presence = requiredElement(document, 'ai-presence');
    this.presenceText = requiredElement(document, 'ai-presence-text');
    this.errorPanel = requiredElement(document, 'fatal-error');
    this.warningPanel = requiredElement(document, 'feature-warnings');
    this.undoButton = requiredButton(document, 'history-undo');
    this.redoButton = requiredButton(document, 'history-redo');
    this.undoButton.addEventListener('click', this.handleUndo);
    this.redoButton.addEventListener('click', this.handleRedo);
    this.spreadsheet = requiredElement(document, 'spreadsheet');
  }

  onHistoryAction(listener: (direction: HistoryDirection) => void): void {
    this.historyListener = listener;
  }

  setHistoryState(canUndo: boolean, canRedo: boolean): void {
    this.undoButton.disabled = !canUndo;
    this.redoButton.disabled = !canRedo;
  }

  showWarnings(warnings: FeatureWarning[]): void {
    if (warnings.length === 0) {
      this.warningPanel.classList.add('hidden');
      return;
    }
    this.warningPanel.textContent = `Compatibility notice: ${warnings.map((warning) => warning.message).join('; ')}`;
    this.warningPanel.classList.remove('hidden');
  }

  setWorkbookName(name: string): void {
    this.workbookName.textContent = name;
  }

  showRevision(message: string, revision: number, state: RevisionState = 'ready'): void {
    this.revisionStatus.textContent = revision > 0 ? `${message} · revision ${revision}` : message;
    this.revisionStatus.dataset.state = state;
    this.spreadsheet.setAttribute('aria-busy', state === 'pending' ? 'true' : 'false');
  }

  showAIPresence(event: WorkbookEvent, revision: number): void {
    if (event.type === 'workbook.reload') {
      this.showRevision(`AI ${event.state} applied`, revision);
      return;
    }
    window.clearTimeout(this.presenceTimer);
    this.presence.classList.remove('hidden');
    this.presence.classList.add('flex');
    const selection = event.type === 'cell.typing' || event.type === 'cell.commit'
      ? event.cell
      : 'range' in event
        ? event.range
        : `${event.index}:${event.index + event.count - 1}`;
    const location = `${event.sheet}!${selection}`;
    if (event.type === 'presence.update') {
      this.presenceText.textContent = `AI selected ${location}`;
    } else if (event.type === 'cell.typing') {
      this.presenceText.textContent = `AI is editing ${location}`;
    } else {
      const action = event.type === 'cell.commit' || event.type === 'range.commit'
        ? 'committed'
        : 'updated';
      this.presenceText.textContent = `AI ${action} ${location}`;
      this.showRevision('AI change saved', revision, 'success');
      this.presenceTimer = window.setTimeout(() => {
        this.presence.classList.add('hidden');
        this.presence.classList.remove('flex');
      }, 4000);
    }
  }

  showError(error: unknown, revision: number): void {
    const message = error instanceof Error ? error.message : String(error);
    const details = `${message} Check the connection and workbook access, then try again.`;
    this.errorPanel.replaceChildren(this.createErrorHeading('Operation failed'), this.createErrorMessage(details));
    this.errorPanel.classList.remove('hidden');
    this.showRevision('Operation failed', revision, 'error');
    this.errorPanel.focus({ preventScroll: true });
  }

  showRecovery(message: string, revision: number): void {
    const conflict = /conflict|changed|external|newer|busy/i.test(message);
    const status = conflict ? 'Conflict detected' : 'Recovery required';
    const details = `${message} Editing is paused. Copy any unsaved drafts before reloading. Do not repeat an uncertain write until you have checked the saved workbook.`;
    const reload = this.errorPanel.ownerDocument.createElement('button');
    reload.type = 'button';
    reload.className = 'mt-3 min-h-11 touch-manipulation rounded border border-red-700 bg-white px-3 py-2 font-semibold underline hover:bg-red-100 active:bg-red-200 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-700';
    reload.textContent = 'Reload saved workbook';
    reload.onclick = () => window.location.reload();
    this.errorPanel.replaceChildren(this.createErrorHeading(status), this.createErrorMessage(details), reload);
    this.errorPanel.classList.remove('hidden');
    this.setHistoryState(false, false);
    this.showRevision(status, revision, conflict ? 'conflict' : 'recovery');
    this.errorPanel.focus({ preventScroll: true });
  }

  dispose(): void {
    window.clearTimeout(this.presenceTimer);
    this.undoButton.removeEventListener('click', this.handleUndo);
    this.redoButton.removeEventListener('click', this.handleRedo);
  }

  private readonly handleUndo = (): void => this.historyListener?.('undo');
  private readonly handleRedo = (): void => this.historyListener?.('redo');

  private createErrorHeading(message: string): HTMLElement {
    const heading = this.errorPanel.ownerDocument.createElement('strong');
    heading.className = 'block text-sm font-bold';
    heading.textContent = message;
    return heading;
  }

  private createErrorMessage(message: string): HTMLElement {
    const details = this.errorPanel.ownerDocument.createElement('p');
    details.className = 'mt-1 text-sm leading-5';
    details.textContent = message;
    return details;
  }
}

function requiredElement(document: Document, id: string): HTMLElement {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error(`Missing required element #${id}`);
  }
  return element;
}

function requiredButton(document: Document, id: string): HTMLButtonElement {
  const element = document.getElementById(id);
  if (!(element instanceof HTMLButtonElement)) {
    throw new Error(`Missing button #${id}`);
  }
  return element;
}
