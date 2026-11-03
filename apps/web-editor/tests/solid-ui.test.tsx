import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'solid-js/web';
import { AppView } from '../src/ui/app-view';
import { WorkbookShell } from '../src/ui/workbook-shell';
import { DocumentApp } from '../src/docx/document-app';
import { WorkbookApp } from '../src/workbook-app';
import type { Snapshot } from '../src/docx/contracts';
import type { WorkbookClient } from '../src/transport/workbook-client';
import type { SpreadsheetEngine } from '../src/spreadsheet-engine';

let cleanup = () => {};
afterEach(() => { cleanup(); document.body.replaceChildren(); vi.unstubAllGlobals(); });
const settle = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
function mount(component: () => any) {
  const root = document.createElement('div'); document.body.append(root);
  cleanup = render(component, root); return root;
}
function button(root: ParentNode, label: string): HTMLButtonElement {
  const result = [...root.querySelectorAll('button')].find((element) => element.textContent === label);
  if (!result) throw new Error(`Missing button: ${label}`);
  return result;
}

describe('Solid workbook shell', () => {
  it('updates history, presence and statuses without replacing the engine container', async () => {
    const view = new AppView(); const root = mount(() => <WorkbookShell view={view}/>);
    const grid = view.spreadsheet; const sentinel = document.createElement('canvas'); grid.append(sentinel);
    const history = vi.fn(); view.onHistoryAction(history);
    expect(button(root, '실행 취소').disabled).toBe(true);
    view.showRevision('Ready', 1); view.setHistoryState(true, false); button(root, '실행 취소').click();
    expect(history).toHaveBeenCalledWith('undo');
    view.showRevision('Saved', 3, 'success');
    expect(root.querySelector('#revision-status')?.textContent).toContain('저장됨 · 리비전 3');
    expect(grid.getAttribute('aria-busy')).toBe('false');
    view.showAIPresence({ type: 'presence.update', actor: 'ai', sheet: 'Sheet1', range: 'A1', revision: 3, sequence: 1 }, 3);
    expect(root.querySelector('#ai-presence')?.textContent).toContain('AI 선택');
    expect(root.querySelector('#spreadsheet')).toBe(grid);
    expect(grid.firstChild).toBe(sentinel);
    view.dispose();
  });

  it('renders document text safely and supports dismissible warnings and protected recovery', async () => {
    const view = new AppView(); const root = mount(() => <WorkbookShell view={view}/>);
    view.showWarnings([{ feature: 'test', message: '<img src=x onerror=alert(1)>' }]);
    expect(root.querySelector('#feature-warnings img')).toBeNull();
    const warning = root.querySelector('#feature-warnings')!;
    button(warning, '자세히').click(); button(warning, '접기').click();
    expect(warning.querySelector('[aria-expanded]')?.getAttribute('aria-expanded')).toBe('false');
    button(warning, '자세히').click(); button(warning, '닫기').click();
    expect(root.querySelector('#feature-warnings')).toBeNull();
    view.showRecovery('Revision conflict', 4); await settle();
    expect(button(root, '실행 취소').disabled).toBe(true);
    expect(document.activeElement?.id).toBe('fatal-error');
    const error = root.querySelector('#fatal-error')!;
    expect(error.getAttribute('role')).toBe('alert');
    expect([...error.querySelectorAll('button')].some((b) => b.textContent === '닫기')).toBe(false);
    button(error, '접기').click();
    expect(root.querySelector('#fatal-error')).not.toBeNull();
    view.dispose();
  });

  it('cleans up the engine and subscription when the Solid root unmounts', async () => {
    const close = vi.fn(); const dispose = vi.fn();
    const snapshot = { id: 'workbook', name: 'test.xlsx', revision: 1, sheets: [], sheetDimensions: [], canUndo: false, canRedo: false };
    const client = { connect: async () => {}, getWorkbook: async () => snapshot, subscribe: () => ({ close }) } as unknown as WorkbookClient;
    const createEngine = vi.fn(() => new Proxy({ dispose }, { get(target, key) { return key in target ? target[key as keyof typeof target] : () => {}; } }) as unknown as SpreadsheetEngine);
    mount(() => <WorkbookApp client={client} createEngine={createEngine}/>); await settle();
    expect(createEngine).toHaveBeenCalledTimes(1);
    cleanup(); expect(dispose).toHaveBeenCalledTimes(1); expect(close).toHaveBeenCalledTimes(1);
  });
});

const snapshot: Snapshot = {
  id: 'session', name: 'test.docx', revision: 1, sequence: 0, warnings: ['호환성 안내'],
  blocks: [{ id: 'p1', type: 'paragraph', paragraph: { id: 'p1', runs: [{ id: 'r1', text: '원본', editable: true, bold: false, italic: false }] } },
    { id: 'table1', type: 'table', rows: [{ cells: [{ id: 'c1', paragraphs: [{ id: 'p2', runs: [{ id: 'r2', text: '표 원본', editable: true, bold: true, italic: false }] }] }] }] }],
};
function docxFixture() {
  const sources: any[] = [];
  class Source {
    listener?: (event: { data: string }) => void;
    close = vi.fn(); onerror?: () => void;
    constructor() { sources.push(this); }
    addEventListener(_type: string, listener: any) { this.listener = listener; }
  }
  vi.stubGlobal('EventSource', Source);
  const fetcher = vi.fn(async (_url: string, init?: RequestInit) => ({ ok: true, json: async () => init?.method === 'POST' ? { ...snapshot, revision: 2 } : snapshot }));
  vi.stubGlobal('fetch', fetcher);
  return { sources, fetcher };
}
function edit(input: HTMLTextAreaElement, value: string) { input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); }

describe('Solid DOCX editor', () => {
  it('retains input nodes while draft/status changes and saves body and table in one batch', async () => {
    const f = docxFixture(); const root = mount(() => <DocumentApp/>); await settle();
    const input = root.querySelector('textarea')!;
    input.focus(); edit(input, '초안');
    expect(root.querySelector('textarea')).toBe(input); expect(document.activeElement).toBe(input);
    expect(input.value).toBe('초안');
    edit(root.querySelectorAll('textarea')[1]!, '표 초안');
    button(root, '변경 저장').click(); await settle();
    const body = JSON.parse(f.fetcher.mock.calls.find(([, init]) => init?.method === 'POST')![1]!.body as string);
    expect(body.baseRevision).toBe(1); expect(body.sessionId).toBe('session');
    expect(body.operations.map((op: any) => op.type)).toEqual(['replace_run_text', 'replace_cell_text']);
    expect(root.querySelector('#save-status')?.textContent).toContain('리비전 2');
  });

  it('preserves drafts on remote commits and requires explicit discard before reloading', async () => {
    const f = docxFixture(); const root = mount(() => <DocumentApp/>); await settle();
    const input = root.querySelector('textarea')!; edit(input, '남길 초안');
    f.sources.at(-1).listener({ data: JSON.stringify({ type: 'docx.committed', sequence: 1, revision: 2, actor: 'ai' }) });
    expect(input.value).toBe('남길 초안'); expect(button(root, '변경 저장').disabled).toBe(true);
    button(root, '다시 불러오기').click(); await settle();
    expect(document.activeElement?.id).toBe('discard-draft');
    expect(f.fetcher).toHaveBeenCalledTimes(1);
    button(root, '취소').click(); expect(input.value).toBe('남길 초안');
    expect(document.activeElement?.id).toBe('reload-document');
    button(root, '다시 불러오기').click(); button(root, '입력 버리고 불러오기').click(); await settle();
    expect(f.fetcher).toHaveBeenCalledTimes(2);
    expect(root.querySelector('textarea')?.value).toBe('원본');
    const source = f.sources.at(-1); cleanup(); expect(source.close).toHaveBeenCalled();
  });

  it('keeps drafts after a lost response and never reports a successful save', async () => {
    const f = docxFixture(); const root = mount(() => <DocumentApp/>); await settle();
    const input = root.querySelector('textarea')!; edit(input, '저장할 초안');
    f.fetcher.mockRejectedValueOnce(new Error('offline'));
    button(root, '변경 저장').click(); await settle();
    expect(input.value).toBe('저장할 초안'); expect(button(root, '변경 저장').disabled).toBe(true);
    expect(root.querySelector('#save-status')?.getAttribute('data-state')).toBe('failed');
  });
});

it('custom toolbar reflects selection and blocks formatting during a pending write', () => {
  const view = new AppView(); const root = mount(() => <WorkbookShell view={view}/>);
  const format = vi.fn(); view.onFormat(format);
  expect(button(root, '굵게').disabled).toBe(true);
  view.setSelection({ sheet: 'Sheet1', range: 'A1:B2', state: 'selecting', bold: false, italic: false, fontSize: 11 });
  view.showRevision('Ready', 1);
  button(root, '굵게').click();
  expect(format).toHaveBeenCalledWith({ bold: true }, expect.objectContaining({ range: 'A1:B2' }));
  view.showRevision('Saving A1…', 1, 'pending');
  expect(button(root, '기울임').disabled).toBe(true);
  button(root, '기울임').click(); expect(format).toHaveBeenCalledTimes(1);
  view.dispose();
});

it('activity shows observed commits only, deduplicates events, and bounds the connection log', () => {
  const view = new AppView(); const root = mount(() => <WorkbookShell view={view}/>);
  const grid = view.spreadsheet; const marker = document.createElement('canvas'); grid.append(marker);
  if (!view.activity.open()) button(root, '활동').click();
  expect(root.querySelector('#document-activity')?.textContent).toContain('아직 기록된 변경이 없습니다.');
  view.recordActivity({ id: 'write', actor: 'human', state: 'pending', title: '문서 변경' });
  view.showRecovery('connection lost', 1);
  expect(view.activity.entries()[0]?.state).toBe('uncertain');
  view.recordActivity({ id: 'write', actor: 'human', state: 'committed', title: '문서 변경 저장', revision: 2 });
  expect(view.activity.entries()).toHaveLength(1);
  for (let i = 0; i < 70; i++) view.recordActivity({ id: `event:${i}`, actor: 'ai', state: 'committed', title: '범위 서식 저장', revision: i + 3 });
  expect(view.activity.entries()).toHaveLength(60);
  expect(root.querySelector('#spreadsheet')).toBe(grid); expect(grid.firstChild).toBe(marker);
  expect(root.querySelector('#document-activity')?.textContent).toContain('AI');
  view.dispose();
});
