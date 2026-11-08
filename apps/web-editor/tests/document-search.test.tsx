import { afterEach, expect, it, vi } from 'vitest';
import { render } from 'solid-js/web';
import { createSignal } from 'solid-js';
import { DocumentSearch, type SearchResults } from '../src/ui/document-search';
import { searchDocument } from '../src/docx/search';
let cleanup = () => {};
afterEach(() => { cleanup(); document.body.replaceChildren(); });
const settle = () => new Promise(resolve => setTimeout(resolve, 0));
it('searches draft text across formatting runs and tables without changing inputs', () => {
  document.body.innerHTML = '<article><section class="docx-paragraph"><textarea>원본</textarea><span data-docx-text> 보고서</span></section><section class="docx-paragraph"><textarea>표 매출</textarea></section></article>';
  const field = document.querySelector('textarea')!; field.value = '매출 초안';
  const results = searchDocument(document.querySelector('article'), '초안 보고서');
  expect(results.items).toHaveLength(1);
  const paragraph = document.querySelector('section')!; paragraph.scrollIntoView = vi.fn();
  results.items[0]!.locate();
  expect(document.activeElement).toBe(field); expect(field.selectionStart).toBe(3);
  expect(field.value).toBe('매출 초안');
  expect(searchDocument(document.querySelector('article'), '매출').items).toHaveLength(2);
});
it('opens by shortcut, ignores stale responses and returns focus with Escape', async () => {
  const [version, setVersion] = createSignal('1');
  let resolve!: (results: SearchResults) => void;
  const search = vi.fn(() => new Promise<SearchResults>(r => { resolve = r; }));
  cleanup = render(() => <DocumentSearch version={version()} search={search} hint="검색 안내"/>, document.body);
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', ctrlKey: true, cancelable: true })); await settle();
  const input = document.querySelector('input')!;
  expect(document.activeElement).toBe(input);
  input.value = '매출'; input.dispatchEvent(new Event('input', { bubbles: true }));
  document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  setVersion('2'); resolve({ items: [{ label: 'stale', text: 'old', locate: vi.fn() }] }); await settle();
  expect(document.body.textContent).not.toContain('stale');
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
  expect(document.querySelector('input')).toBeNull(); expect(document.activeElement?.textContent).toBe('검색');
});
it('reports an empty result and navigates a result without changing the document', async () => {
  const locate = vi.fn(); let found: SearchResults = { items: [] };
  cleanup = render(() => <DocumentSearch version="1" search={async () => found} hint="안내"/>, document.body);
  (document.querySelector('button') as HTMLButtonElement).click(); await settle();
  const input = document.querySelector('input')!; input.value = 'a'; input.dispatchEvent(new Event('input', { bubbles: true }));
  const submit = () => document.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true }));
  submit(); await settle(); expect(document.body.textContent).toContain('0개 위치');
  found = { items: [{ label: 'Sheet1!A1', text: '<script>', locate }], truncated: true };
  submit(); await settle(); expect(document.querySelector('script')).toBeNull(); expect(document.body.textContent).toContain('일부 결과');
  [...document.querySelectorAll('button')].find(b => b.textContent?.includes('Sheet1!A1'))!.click(); expect(locate).toHaveBeenCalledOnce();
});
it('finds repeated literal results and rejects stale replacement targets', () => {
  document.body.innerHTML='<article><section class="docx-paragraph"><span data-docx-run="a">ab</span><span data-docx-run="b">c ABC abcd</span></section><table><tbody><tr><td><section class="docx-paragraph"><textarea>abc</textarea></section></td></tr></tbody></table></article>';
  const replace=vi.fn(()=>true), root=document.querySelector('article')!;
  const found=searchDocument(root,'abc',{wholeWord:true},replace);
  expect(found.items).toHaveLength(3);
  expect(found.protectedCount).toBe(1);
  expect(found.replaceAll?.('new')).toBe(true);
  expect(replace).toHaveBeenCalledWith([
    {anchor:{runId:'a',offset:0},focus:{runId:'b',offset:1}},
    {anchor:{runId:'b',offset:2},focus:{runId:'b',offset:5}},
  ],'new');
  expect(searchDocument(root,'abc',{matchCase:true,scope:'body'}).items).toHaveLength(2);
  expect(searchDocument(root,'abc',{scope:'table'}).items).toHaveLength(1);
  root.querySelector('[data-docx-run="b"]')!.textContent='c changed';
  expect(found.items[0]!.replace?.('stale')).toBe(false);
  expect(replace).toHaveBeenCalledOnce();
});
