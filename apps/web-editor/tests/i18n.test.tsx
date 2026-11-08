import { afterEach, expect, it, vi } from 'vitest';
import { render } from 'solid-js/web';
import { catalog, initializeI18n, knownMessage, locale, localizeMessage, resolveLocale, setLocale, source, t, type MessageKey } from '../src/i18n';
import { LocaleSelect } from '../src/i18n/locale-select';
import { DocumentApp } from '../src/docx/document-app';
import { AppView } from '../src/ui/app-view';
import { WorkbookShell } from '../src/ui/workbook-shell';
import { problemMessage, warningMessage } from '../src/ui/messages';
let cleanup = () => {};
afterEach(() => { cleanup(); document.body.replaceChildren(); localStorage.clear(); vi.unstubAllGlobals(); });
const settle = () => new Promise(resolve => setTimeout(resolve, 0));

it('provides all four locales with identical interpolation parameters', () => {
  for (const entry of Object.values(catalog)) {
    const placeholders = (value: string) => [...value.matchAll(/\{(\w+)\}/g)].map(match => match[1]).sort();
    expect(Object.keys(entry).sort()).toEqual(['en', 'ja', 'ko', 'zh-CN']);
    for (const value of Object.values(entry)) {
      expect(value.trim()).not.toBe('');
      expect(placeholders(value)).toEqual(placeholders(entry.ko));
    }
  }
});
it('detects supported browser preferences and safely falls back for unsupported languages', () => {
  expect(resolveLocale(['fr-FR', 'ja-JP'])).toBe('ja');
  expect(resolveLocale(['ko-KR', 'en-US'])).toBe('ko');
  expect(resolveLocale(['zh-Hans-SG'])).toBe('zh-CN');
  expect(resolveLocale(['zh-TW'])).toBe('en');
  expect(resolveLocale([])).toBe('en');
});
it('updates messages reactively while preserving file names and diagnostic content', () => {
  cleanup = render(() => <><LocaleSelect/><p>{localizeMessage(source('savingTarget', { location: '저장됨!A1' }))}</p></>, document.body);
  setLocale('en');
  expect(document.body.textContent).toContain('Saving 저장됨!A1');
  expect(t('closeNamed', { name: '저장됨' })).toBe('Close 저장됨');
  expect(localStorage.getItem('document-workspace:locale:v1')).toBe('en');
  expect(document.documentElement.lang).toBe('en');
  const before = locale();
  expect(() => setLocale('unsupported' as any)).toThrow(); expect(locale()).toBe(before);
  const combined = source('errorWithHelp', { problem: source('anotherEditWasSavedFirstOrAnExternal') });
  // Nested problem summaries and multi-paragraph compatibility notices share the same locale.
  expect(localizeMessage(combined)).toContain('Check the connection');
  expect(localizeMessage(source('formulasArePreservedServerValuesMayBeCached') + '\n\n' + source('externalLinksArePreservedTheyAreNotOpened'))).not.toMatch(/[가-힣]/u);
});
it('shows a localized recovery explanation and keeps the exact server diagnostic separately', async () => {
  const view = new AppView(); cleanup = render(() => <WorkbookShell view={view}/>, document.body);
  setLocale('ja');
  view.showRecovery('revision conflict: expected 2, got 1', 2); await settle();
  const panel = document.querySelector('#fatal-error')!;
  expect(panel.querySelector('p')!.textContent).toContain('編集を停止しました');
  expect(panel.querySelector('pre')!.textContent).toBe('revision conflict: expected 2, got 1');
  setLocale('en'); expect(panel.querySelector('p')!.textContent).toContain('Editing is paused');
  expect(problemMessage('unknown diagnostic <script>')).toBe(source('theDocumentCouldNotBeProcessed'));
  expect(warningMessage('unrecognized feature')).toContain('not fully supported');
  expect(document.querySelector('script')).toBeNull(); view.dispose();
});
it('changes DOCX messages and validation without replacing the draft or translating the editing toolbar', async () => {
  const snapshot = { id: 'docx', name: 'Saved.docx', revision: 1, sequence: 0, warnings: [source('docxNative')], blocks: [
    { id: 'p1', type: 'paragraph', paragraph: { id: 'p1', runs: [{ id: 'run1', text: '저장됨', editable: true, bold: false, italic: false }] } },
  ] };
  class Events { addEventListener() {} close() {} onerror: any; }
  vi.stubGlobal('EventSource', Events); vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, json: async () => snapshot })));
  cleanup = render(() => <DocumentApp/>, document.body); await settle();
  const field = document.querySelector('textarea')!; field.value = '보존할 초안'; field.dispatchEvent(new Event('input', { bubbles: true }));
  setLocale('en');
  expect(document.querySelector('textarea')).toBe(field); expect(field.value).toBe('보존할 초안');
  expect(document.querySelector('#save-status')!.textContent).toBe('Unsaved changes');
  expect(document.querySelector('#save-document')!.textContent).toBe('Save');
  expect(document.querySelector('#reload-document')!.getAttribute('aria-label')).toBe('다시 불러오기');
  expect(document.querySelector('#document-name')!.textContent).toBe('Saved.docx');
  expect(document.body.textContent).toContain('Native opening and visual compatibility');
  field.value = '제어\x01문자'; field.dispatchEvent(new Event('input', { bubbles: true }));
  expect(document.body.textContent).toContain('unsupported characters');
  setLocale('ja'); expect(document.querySelector('#save-status')!.textContent).toBe('未保存の変更');
});
it('validates parent messages and synchronizes all editor frames without navigation', () => {
  const parent = {};
  const listeners = new Map<string, (event: any) => void>();
  const remove = vi.fn();
  vi.stubGlobal('window', { parent, location: { origin: 'http://localhost' }, addEventListener: (type: string, listener: any) => listeners.set(type, listener), removeEventListener: remove });
  const dispose = initializeI18n();
  const deliver = (origin: string, sender: unknown, selected: unknown) => listeners.get('message')!({ origin, source: sender, data: { type: 'workspace:locale', locale: selected } });
  deliver('https://evil.example', parent, 'en'); expect(locale()).toBe('ko');
  deliver('http://localhost', {}, 'en'); expect(locale()).toBe('ko');
  deliver('http://localhost', parent, 'unsupported'); expect(locale()).toBe('ko');
  deliver('http://localhost', parent, 'zh-CN'); expect(locale()).toBe('zh-CN');
  listeners.get('storage')!({ key: 'document-workspace:locale:v1', newValue: 'ja' }); expect(locale()).toBe('ja');
  dispose(); expect(remove).toHaveBeenCalledTimes(3);
});

it('keeps failed DOCX write diagnostics and the draft without allowing an automatic retry', async () => {
  const snapshot = { id: 'docx', name: 'report.docx', revision: 1, sequence: 0, warnings: [], blocks: [
    { id: 'p1', type: 'paragraph', paragraph: { id: 'p1', runs: [{ id: 'r1', text: 'original', editable: true, bold: false, italic: false }] } },
  ] };
  class Events { addEventListener() {} close() {} onerror: any; }
  vi.stubGlobal('EventSource', Events);
  vi.stubGlobal('fetch', vi.fn(async (_url: string, init?: RequestInit) => init?.method === 'POST'
    ? { ok: false, status: 400, json: async () => ({ error: 'unsupported DOCX target <img>' }) }
    : { ok: true, json: async () => snapshot }));
  cleanup = render(() => <DocumentApp/>, document.body); await settle(); setLocale('en');
  const field = document.querySelector('textarea')!; field.value = 'draft'; field.dispatchEvent(new Event('input', { bubbles: true }));
  (document.querySelector('#save-document') as HTMLButtonElement).click(); await settle();
  expect(document.querySelector('#document-error p')!.textContent).toContain('Copy your input');
  expect(document.querySelector('#document-error pre')!.textContent).toBe('unsupported DOCX target <img>');
  expect(document.querySelector('#document-error img')).toBeNull(); expect(field.value).toBe('draft');
  expect((document.querySelector('#save-document') as HTMLButtonElement).disabled).toBe(true);
});
