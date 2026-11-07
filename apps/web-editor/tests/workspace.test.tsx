import { afterEach, expect, it, vi } from 'vitest';
import type { Window as HappyWindow } from 'happy-dom';
import { render } from 'solid-js/web';
import { WorkspaceApp } from '../src/workspace/workspace-app';
import { scopedDocumentURL } from '../src/transport/document-scope';
let cleanup = () => {};
afterEach(() => { cleanup(); document.body.replaceChildren(); vi.unstubAllGlobals(); localStorage.clear(); window.history.replaceState({}, '', '/'); });
const settle = () => new Promise(resolve => setTimeout(resolve, 0));
const first = { id: 'first', path: '/tmp/a/report.docx', name: 'report.docx', kind: 'docx' };
const second = { id: 'second', path: '/tmp/b/book.xlsx', name: 'book.xlsx', kind: 'xlsx' };
function fixture(initial: typeof first & { uploaded?: boolean } = first) {
  const report = console.error.bind(console);
  vi.spyOn(console, 'error').mockImplementation((...args) => {
    if (!String(args[0]).includes('Iframe page loading is disabled')) report(...args);
  });
  (window as unknown as HappyWindow).happyDOM.settings.disableIframePageLoading = true;
  let documents = [initial];
  const fetcher = vi.fn(async (_path: string, init?: RequestInit) => {
    if (init?.method === 'POST') { documents = [first, second]; return { ok: true, status: 200, json: async () => second }; }
    if (init?.method === 'DELETE') return { ok: true, status: 204 };
    return { ok: true, status: 200, json: async () => ({ documents, maxOpen: 8 }) };
  });
  vi.stubGlobal('fetch', fetcher);
  cleanup = render(() => <WorkspaceApp/>, document.body);
  return fetcher;
}
function button(name: string) { return [...document.querySelectorAll<HTMLButtonElement>('button')].find(item => item.textContent === name || item.getAttribute('aria-label') === name)!; }
it('opens another document without replacing an existing frame and supports keyboard tab navigation', async () => {
  fixture(); await settle();
  const frame = document.querySelector('iframe')!;
  const paths = document.querySelector('textarea')!; paths.value = second.path; paths.dispatchEvent(new Event('input', { bubbles: true }));
  button('파일 열기').click(); await settle();
  expect(document.querySelectorAll('iframe')).toHaveLength(2);
  expect(document.querySelector('iframe')).toBe(frame);
  const tabs = [...document.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
  expect(tabs[1]!.getAttribute('aria-selected')).toBe('true');
  tabs[1]!.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
  expect(tabs[0]!.getAttribute('aria-selected')).toBe('true');
  expect(document.querySelector('iframe')).toBe(frame);
  expect(localStorage.getItem('document-workspace:recent:v1')).toContain(second.path);
});
it('does not close a mounted editor without a response from that frame', async () => {
  const fetcher = fixture(); await settle();
  button('report.docx 닫기').click();
  window.dispatchEvent(new MessageEvent('message', { origin: 'https://evil.example', data: { type: 'workspace:checked', request: '1', id: first.id, busy: false, dirty: false, message: '저장됨' } }));
  await settle();
  expect(fetcher.mock.calls.some(([, init]) => init?.method === 'DELETE')).toBe(false);
  expect(document.querySelector('iframe')).not.toBeNull();
});
it('pins URLs to the document ID rather than the current workspace tab', () => {
  window.history.replaceState({}, '', '/?document=first&embedded=1');
  expect(scopedDocumentURL('/api/range?sheet=Sheet1&range=A1')).toBe('/api/range?sheet=Sheet1&range=A1&document=first');
  expect(scopedDocumentURL('/api/docx/operations')).toBe('/api/docx/operations?document=first');
});

it('requires explicit draft discard and accepts only the owning frame response', async () => {
  const fetcher = fixture(); await settle();
  const frame = document.querySelector('iframe')!;
  Object.defineProperty(frame, 'contentWindow', { configurable: true, value: window });
  const reply = (request: string, dirty: boolean) => window.dispatchEvent(new MessageEvent('message', {
    origin: window.location.origin, source: frame.contentWindow,
    data: { type: 'workspace:checked', id: first.id, request, dirty, busy: false, message: dirty ? '미저장 변경' : '저장됨' },
  }));
  button('report.docx 닫기').click(); reply('1', true); await settle();
  expect(document.querySelector('[aria-label="파일 닫기 확인"]')).not.toBeNull();
  expect(fetcher.mock.calls.some(([, init]) => init?.method === 'DELETE')).toBe(false);
  button('취소').click();
  expect(document.querySelector('iframe')).toBe(frame);
  button('report.docx 닫기').click(); reply('2', true); await settle();
  button('입력 버리고 닫기').click(); reply('3', true); await settle();
  expect(fetcher.mock.calls.filter(([, init]) => init?.method === 'DELETE')).toHaveLength(1);
  expect(document.querySelector('iframe')).toBeNull();
  const unload = new Event('beforeunload', { cancelable: true });
  window.dispatchEvent(unload);
  expect(unload.defaultPrevented).toBe(false);
});

it('opens browser-selected files as uploads and keeps local paths out of recent history', async () => {
  const fetcher = fixture(); await settle();
  const input = document.querySelector<HTMLInputElement>('input[type="file"]')!;
  expect(input.multiple).toBe(true);
  expect(input.accept).toBe('.xlsx,.docx');
  const file = new File(['test package'], '선택 문서.xlsx');
  Object.defineProperty(input, 'files', { configurable: true, value: [file] });
  input.dispatchEvent(new Event('change', { bubbles: true })); await settle();
  const call = fetcher.mock.calls.find(([path]) => path === '/api/documents/upload')!;
  expect(call[1]?.body).toBeInstanceOf(FormData);
  expect((call[1]?.body as FormData).get('file')).toBeInstanceOf(File);
  expect(document.querySelectorAll('[role="tab"]')).toHaveLength(2);
  expect(localStorage.getItem('document-workspace:recent:v1')).toBeNull();
});

it('rejects unsupported selected files before uploading', async () => {
  const fetcher = fixture(); await settle();
  const input = document.querySelector<HTMLInputElement>('input[type="file"]')!;
  Object.defineProperty(input, 'files', { configurable: true, value: [new File(['bad'], 'file.exe')] });
  input.dispatchEvent(new Event('change', { bubbles: true })); await settle();
  expect(fetcher.mock.calls.some(([, init]) => init?.method === 'POST')).toBe(false);
  expect(document.body.textContent).toContain('XLSX는 100 MB');
});

it('confirms closing a saved working copy so downloaded edits cannot be silently lost', async () => {
  const fetcher = fixture({ ...first, uploaded: true, path: '' }); await settle();
  const frame = document.querySelector('iframe')!;
  Object.defineProperty(frame, 'contentWindow', { configurable: true, value: window });
  const reply = (request: string) => window.dispatchEvent(new MessageEvent('message', { origin: window.location.origin, source: frame.contentWindow,
    data: { type: 'workspace:checked', id: first.id, request, dirty: false, busy: false, message: '저장됨' } }));
  button('report.docx 닫기').click(); reply('1'); await settle();
  expect(document.body.textContent).toContain('필요한 수정본을 먼저 다운로드하세요');
  expect(fetcher.mock.calls.some(([, init]) => init?.method === 'DELETE')).toBe(false);
  expect(button('저장 후 닫기')).toBeUndefined();
  button('복사본 닫기').click(); reply('2'); await settle();
  expect(fetcher.mock.calls.filter(([, init]) => init?.method === 'DELETE')).toHaveLength(1);
});

it('creates a named empty DOCX without replacing the existing editor', async () => {
  const fetcher = fixture(); await settle();
  const existing = document.querySelector('iframe');
  button('새 DOCX 문서').click(); await settle();
  const dialog = document.querySelector('dialog')!;
  const name = dialog.querySelector('input')!;
  const enterName = (value: string) => {
    name.value = value;
    name.dispatchEvent(new Event('input', { bubbles: true }));
  };
  enterName('../original.docx');
  expect(button('문서 만들기').disabled).toBe(true);
  enterName('REPORT.DOCX');
  expect(button('문서 만들기').disabled).toBe(true);
  enterName(' 회의록 ');
  const opened = { id: 'new-doc', name: '회의록.docx', path: '', kind: 'docx', origin: 'new', uploaded: true };
  fetcher.mockResolvedValueOnce({ ok: true, status: 201, json: async () => opened });
  dialog.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  await settle();
  expect(fetcher).toHaveBeenCalledWith('/api/documents/new-docx', expect.objectContaining({
    method: 'POST', body: JSON.stringify({ name: '회의록.docx' }),
  }));
  expect(document.querySelector('dialog')).toBeNull();
  expect(document.querySelector('iframe')).toBe(existing);
  expect(document.querySelector('[role="tab"][aria-selected="true"]')?.textContent).toContain('회의록.docx');
  expect(localStorage.getItem('document-workspace:recent:v1')).toBeNull();
});

it('opens a recovered copy as a separate tab without replacing the source', async () => {
  const fetcher = fixture(); await settle();
  const existing = document.querySelector('iframe');
  const id = '1234567890abcdef1234567890abcdef';
  fetcher.mockResolvedValueOnce({ ok: true, status: 200, json: async () => [
    { id, name: first.name, updatedAt: '2026-11-07T12:00:00Z', hasDraft: true },
  ] });
  const summary = [...document.querySelectorAll('summary')].find(node => node.textContent === '복구 사본')!;
  const details = summary.parentElement as HTMLDetailsElement;
  details.open = true;
  details.dispatchEvent(new Event('toggle'));
  await settle();
  expect(details.textContent).toContain('초안 포함');
  fetcher.mockResolvedValueOnce({ ok: true, status: 201, json: async () => ({
    id: 'recovered', name: 'report (복구).docx', path: '', kind: 'docx', origin: 'recovery', uploaded: true,
  }) });
  button('별도 복사본으로 열기').click();
  await settle();
  expect(fetcher).toHaveBeenCalledWith(`/api/documents/docx-recovery/${id}/open`, expect.objectContaining({ method: 'POST' }));
  expect(document.querySelector('iframe')).toBe(existing);
  expect(document.querySelector('[aria-selected="true"]')?.textContent).toContain('report (복구).docx');
});
