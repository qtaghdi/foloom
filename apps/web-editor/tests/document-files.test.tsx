import { afterEach, expect, it, vi } from 'vitest';
import { render } from 'solid-js/web';
import { confirmSavedDocument, downloadSavedDocument, DocumentFileError } from '../src/transport/document-files';
import { FileActions } from '../src/ui/file-actions';
import { DocumentController } from '../src/docx/controller';
import { WorkbookController } from '../src/app/workbook-controller';
import { AppView } from '../src/ui/app-view';
import type { WorkbookClient } from '../src/transport/workbook-client';
import type { SpreadsheetEngine } from '../src/spreadsheet-engine';
let cleanup = () => {};
afterEach(() => { cleanup(); document.body.replaceChildren(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
const settle = () => new Promise(resolve => setTimeout(resolve, 0));
const checkpoint = { id: 'document-one', revision: 2, name: '한글 보고서.xlsx' };
const mime = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
function downloadResponse(id = checkpoint.id, revision = 2, type = mime) {
  return new Response(new Blob(['file content'], { type }), { headers: { 'Content-Type': type, 'X-Document-ID': id, 'X-Document-Revision': String(revision) } });
}
it('pins confirmation and download to the same document and rejects wrong bytes metadata', async () => {
  const fetcher = vi.fn(async (url: string) => url.startsWith('/api/save') ? new Response(JSON.stringify(checkpoint), { headers: { 'Content-Type': 'application/json' } }) : downloadResponse());
  vi.stubGlobal('fetch', fetcher);
  await confirmSavedDocument(checkpoint); const file = await downloadSavedDocument(checkpoint, 'xlsx');
  expect(file.name).toBe(checkpoint.name); expect(await file.blob.text()).toBe('file content');
  expect(fetcher.mock.calls[0]![0]).toContain('document=document-one');
  expect(fetcher.mock.calls[1]![0]).toContain('session=document-one&revision=2');
  vi.stubGlobal('fetch', vi.fn(async () => downloadResponse('document-two')));
  await expect(downloadSavedDocument(checkpoint, 'xlsx')).rejects.toBeInstanceOf(DocumentFileError);
  vi.stubGlobal('fetch', vi.fn(async () => downloadResponse(checkpoint.id, 3)));
  await expect(downloadSavedDocument(checkpoint, 'xlsx')).rejects.toThrow('revision conflict');
  vi.stubGlobal('fetch', vi.fn(async () => downloadResponse(checkpoint.id, 2, 'text/html')));
  await expect(downloadSavedDocument(checkpoint, 'xlsx')).rejects.toThrow('content type');
});
it('does not initiate a browser download until the checked file is ready, or after unmount', async () => {
  const NativeURL = URL;
  const create = vi.fn(() => 'blob:checked-document'); const revoke = vi.fn();
  vi.stubGlobal('URL', class extends NativeURL { static createObjectURL = create; static revokeObjectURL = revoke; });
  const clicks: string[] = []; vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function() { clicks.push(this.download); });
  let resolve!: (value: any) => void;
  cleanup = render(() => <FileActions saveID="save" disabled={false} downloadAvailable onSave={async () => true} onDownload={() => new Promise(r => { resolve = r; })}/>, document.body);
  const button = [...document.querySelectorAll('button')].find(button => button.textContent === '다운로드')!;
  button.click(); expect(create).not.toHaveBeenCalled(); cleanup();
  resolve({ blob: new Blob(['saved']), name: 'report.docx' }); await settle();
  expect(clicks).toEqual([]); expect(create).not.toHaveBeenCalled();
});
it('hands the original filename to the browser once and reports a request rather than completion', async () => {
  const NativeURL = URL; const create = vi.fn(() => 'blob:checked-document');
  vi.stubGlobal('URL', class extends NativeURL { static createObjectURL = create; static revokeObjectURL = vi.fn(); });
  const clicks: string[] = []; vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function() { clicks.push(this.download); });
  cleanup = render(() => <FileActions saveID="save" disabled={false} downloadAvailable onSave={async () => true} onDownload={async () => ({ blob: new Blob(['saved']), name: checkpoint.name })}/>, document.body);
  [...document.querySelectorAll('button')].find(button => button.textContent === '다운로드')!.click(); await settle();
  expect(clicks).toEqual([checkpoint.name]); expect(document.querySelector('[role="status"]')!.textContent).toContain('다운로드를 요청');
});
const docxSnapshot = { id: 'docx-session', name: 'report.docx', revision: 1, sequence: 0, warnings: [], blocks: [{ id: 'p', type: 'paragraph' as const, paragraph: { id: 'p', runs: [{ id: 'run', text: 'original', editable: true, bold: false, italic: false }] } }] };
function documentController() {
  class Events { onerror: any; addEventListener() {} close() {} }
  vi.stubGlobal('EventSource', Events);
  const view = { render: vi.fn(), status: vi.fn(), error: vi.fn(), canSave: vi.fn(), validation: vi.fn() };
  const controller = new DocumentController(view); cleanup = () => controller.dispose();
  return { controller, view };
}
it('saves DOCX drafts before downloading and confirms clean saves without changing the document', async () => {
  const fetcher = vi.fn(async (url: string, init?: RequestInit) => {
    if (url.startsWith('/api/download')) return new Response(new Blob(['saved DOCX']), { headers: { 'Content-Type': 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'X-Document-ID': docxSnapshot.id, 'X-Document-Revision': '2' } });
    if (url.startsWith('/api/save')) return new Response(JSON.stringify(docxSnapshot));
    return new Response(JSON.stringify({ ...docxSnapshot, revision: init?.method === 'POST' ? 2 : 1 }));
  });
  vi.stubGlobal('fetch', fetcher); const { controller } = documentController();
  await controller.load(); expect(await controller.save()).toBe(true);
  expect(fetcher.mock.calls.some(([url]) => url.startsWith('/api/save'))).toBe(true);
  controller.edit({ type: 'replace_run_text', targetId: 'run', expectedText: 'original', text: 'saved draft' });
  const downloaded = await controller.download();
  expect(await downloaded!.blob.text()).toBe('saved DOCX'); expect(controller.hasDrafts()).toBe(false);
  const calls = fetcher.mock.calls.map(([url]) => url);
  expect(calls.findIndex(url => url === '/api/docx/operations')).toBeLessThan(calls.findIndex(url => url.startsWith('/api/download')));
});
it('keeps failed DOCX drafts and never requests a file after a lost save response', async () => {
  const fetcher = vi.fn(async (_url: string, init?: RequestInit) => { if (init?.method === 'POST') throw new Error('network unavailable'); return new Response(JSON.stringify(docxSnapshot)); });
  vi.stubGlobal('fetch', fetcher); const { controller } = documentController(); await controller.load();
  controller.edit({ type: 'replace_run_text', targetId: 'run', expectedText: 'original', text: 'keep draft' });
  expect(await controller.download()).toBeUndefined(); expect(controller.hasDrafts()).toBe(true);
  expect(fetcher.mock.calls.some(([url]) => url.startsWith('/api/download'))).toBe(false);
  const calls = fetcher.mock.calls.length; expect(await controller.download()).toBeUndefined(); expect(fetcher).toHaveBeenCalledTimes(calls);
});
it('finishes the XLSX cell editor and waits for its commit before confirming and downloading', async () => {
  const view = new AppView(); view.spreadsheet = document.createElement('div');
  let edit!: (value: any) => void; let commit!: (value: any) => void;
  const first = { id: 'xlsx-session', revision: 1, sequence: 0, name: 'book.xlsx', sheets: [], sheetDimensions: [], canUndo: false, canRedo: false };
  const second = { ...first, revision: 2 };
  const confirmSave = vi.fn(async (snapshot: any) => { expect(snapshot.revision).toBe(2); });
  const download = vi.fn(async () => ({ blob: new Blob(['saved XLSX']), name: first.name }));
  const client = { connect: async () => {}, getWorkbook: async () => first, subscribe: () => ({ close() {}, onerror: null }), confirmSave, download, applyOperations: () => new Promise(r => { commit = r; }) } as unknown as WorkbookClient;
  const engine = new Proxy({ onCellEdit: (listener: any) => { edit = listener; }, finishEditing: async () => { edit({ sheet: 'Sheet1', cell: 'A1', value: 'draft' }); } }, { get(target, key) { return key in target ? target[key as keyof typeof target] : () => {}; } }) as unknown as SpreadsheetEngine;
  const controller = new WorkbookController(client, view, () => engine); cleanup = () => controller.dispose(); await controller.start();
  const pending = controller.download(); await settle(); expect(confirmSave).not.toHaveBeenCalled(); expect(download).not.toHaveBeenCalled();
  commit({ workbook: second, applied: 1 }); const result = await pending;
  expect(result?.name).toBe('book.xlsx'); expect(confirmSave).toHaveBeenCalledOnce(); expect(download).toHaveBeenCalledOnce(); expect(controller.shouldWarnBeforeUnload()).toBe(false);
});
