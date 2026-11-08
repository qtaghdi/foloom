import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'solid-js/web';
import { AppView } from '../src/ui/app-view';
import { WorkbookShell } from '../src/ui/workbook-shell';
import { readTextSelection, restoreTextSelection } from '../src/docx/selection';
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
  const result = [...root.querySelectorAll('button')].find((element) => element.textContent === label || element.getAttribute('aria-label') === label);
  if (!result) throw new Error(`Missing button: ${label}`);
  return result;
}

describe('Solid workbook shell', () => {
  it('updates presence and statuses without replacing the engine container', async () => {
    const view = new AppView(); const root = mount(() => <WorkbookShell view={view}/>);
    const grid = view.spreadsheet; const sentinel = document.createElement('canvas'); grid.append(sentinel);
    view.showRevision('Saved', 3, 'success');
    expect(root.querySelector('#revision-status')?.textContent).toContain('저장됨');
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
    expect(root.querySelector('#workbook-notices')?.hasAttribute('hidden')).toBe(true);
    button(root, '호환성').click();
    expect(root.querySelector('#workbook-notices')?.hasAttribute('hidden')).toBe(false);
    button(warning, '접기').click();
    expect(warning.querySelector('[aria-expanded]')?.getAttribute('aria-expanded')).toBe('false');
    button(warning, '자세히').click(); button(warning, '닫기').click();
    expect(root.querySelector('#feature-warnings')).toBeNull();
    view.showRecovery('Revision conflict', 4); await settle();
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
function docxFixture(initial: Snapshot = snapshot) {
  const sources: any[] = [];
  class Source {
    listener?: (event: { data: string }) => void;
    close = vi.fn(); onerror?: () => void;
    constructor() { sources.push(this); }
    addEventListener(_type: string, listener: any) { this.listener = listener; }
  }
  vi.stubGlobal('EventSource', Source);
  const fetcher = vi.fn(async (_url: string, init?: RequestInit) => ({ ok: true, json: async () => init?.method === 'POST' ? { ...initial, revision: 2 } : initial }));
  vi.stubGlobal('fetch', fetcher);
  return { sources, fetcher };
}
function edit(input: HTMLTextAreaElement, value: string) { input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); }

describe('Solid DOCX editor', () => {
  it('keeps an empty text target writable throughout sequential draft input', async () => {
    const empty: Snapshot = { ...snapshot, blocks: [{ id: 'empty', type: 'paragraph', paragraph: { id: 'empty', runs: [{ id: 'empty-run', text: '', editable: true, bold: false, italic: false }] } }] };
    docxFixture(empty); const root = mount(() => <DocumentApp/>); await settle();
    const input = root.querySelector('textarea')!;
    expect(input.placeholder).toBe('여기에 입력하세요');
    input.focus();
    for (const text of ['한', '한글', '한글 typing']) {
      edit(input, text);
      expect(input.readOnly).toBe(false);
      expect(document.activeElement).toBe(input);
      expect(root.querySelector('textarea')).toBe(input);
    }
    expect(button(root, '저장').disabled).toBe(false);
  });

  it('retains input nodes while draft/status changes and saves body and table in one batch', async () => {
    const f = docxFixture(); const root = mount(() => <DocumentApp/>); await settle();
    const input = root.querySelector('textarea')!;
    input.focus(); edit(input, '초안');
    expect(root.querySelector('textarea')).toBe(input); expect(document.activeElement).toBe(input);
    expect(input.value).toBe('초안');
    edit(root.querySelectorAll('textarea')[1]!, '표 초안');
    button(root, '저장').click(); await settle();
    const body = JSON.parse(f.fetcher.mock.calls.find(([, init]) => init?.method === 'POST')![1]!.body as string);
    expect(body.baseRevision).toBe(1); expect(body.sessionId).toBe('session');
    expect(body.operations.map((op: any) => op.type)).toEqual(['replace_run_text', 'replace_cell_text']);
    expect(root.querySelector('#save-status')?.textContent).toContain('저장됨');
  });

  it('preserves drafts on remote commits and requires explicit discard before reloading', async () => {
    const f = docxFixture(); const root = mount(() => <DocumentApp/>); await settle();
    const input = root.querySelector('textarea')!; edit(input, '남길 초안');
    f.sources.at(-1).listener({ data: JSON.stringify({ type: 'docx.committed', sequence: 1, revision: 2, actor: 'ai' }) });
    expect(input.value).toBe('남길 초안'); expect(button(root, '저장').disabled).toBe(true);
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
    button(root, '저장').click(); await settle();
    expect(input.value).toBe('저장할 초안'); expect(button(root, '저장').disabled).toBe(true);
    expect(root.querySelector('#save-status')?.getAttribute('data-state')).toBe('failed');
  });
});

it('activity shows observed commits only, deduplicates events, and bounds the connection log', () => {
  const view = new AppView(); const root = mount(() => <WorkbookShell view={view}/>);
  const grid = view.spreadsheet; const marker = document.createElement('canvas'); grid.append(marker);
  if (!view.activity.open()) button(root, '활동').click();
  expect(root.querySelector('#document-activity')?.textContent).toContain('아직 변경이 없어요');
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

it('localizes known workbook notices while retaining unknown diagnostic text', () => {
  const view = new AppView(); const root = mount(() => <WorkbookShell view={view}/>);
  view.showWarnings([{ feature: 'formula calculation', message: 'cached results' }, { feature: 'future', message: 'unknown detail' }]);
  const warning = root.querySelector('#feature-warnings')!;
  expect(warning.textContent).toContain('미리보기'); expect(warning.textContent).toContain('unknown detail');
  view.showRecovery('The live connection was interrupted.', 1);
  expect(root.querySelector('#fatal-error')?.textContent).toContain('연결이 끊겼습니다');
  view.dispose();
});

it('keeps unsupported pasted text editable and restores saving after correction', async () => {
  const f = docxFixture(); const root = mount(() => <DocumentApp/>); await settle();
  const input = root.querySelector('textarea')!;
  edit(input, '붙여넣은\t탭 문자');
  expect(input.value).toBe('붙여넣은\t탭 문자'); expect(input.getAttribute('aria-invalid')).toBe('true');
  expect(button(root, '저장').disabled).toBe(true);
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 's', ctrlKey: true, cancelable: true }));
  expect(document.activeElement).toBe(input); expect(f.fetcher).toHaveBeenCalledTimes(1);
  edit(input, '수정한 한 줄');
  expect(input.getAttribute('aria-invalid')).toBe('false'); expect(button(root, '저장').disabled).toBe(false);
  button(root, '저장').click(); await settle(); expect(f.fetcher).toHaveBeenCalledTimes(2);
});


it('locates an observed AI change by paragraph and displays its verified before and after text', async () => {
  const fixture = docxFixture();
  const root = mount(() => <DocumentApp/>);
  await settle();
  const response = await fixture.fetcher.mock.results[0]!.value;
  const before = await response.json();
  const next = structuredClone(before);
  next.revision = 2;
  next.sequence = 1;
  next.blocks[0]!.paragraph!.runs[0]!.text = 'AI 수정';
  fixture.fetcher.mockResolvedValueOnce({ ok: true, json: async () => next });
  fixture.sources.at(-1).listener({ data: JSON.stringify({ type: 'docx.committed', sequence: 1, revision: 2, actor: 'ai', textOnly:true }) });
  await settle();
  const marker = root.querySelector<HTMLButtonElement>('.change-marker')!;
  expect(marker.closest('section')?.id).toBe('docx-paragraph-0');
  marker.click();
  expect(root.querySelector('[aria-label="AI 변경 전후"]')?.textContent).toContain('원본');
  expect(root.querySelector('[aria-label="AI 변경 전후"]')?.textContent).toContain('AI 수정');
  button(root, '활동').click();
  button(root, '변경 위치로 이동').click();
  await settle();
  expect(document.activeElement?.id).toBe('docx-paragraph-0');
});


it('opens the sheet and cell associated with a confirmed activity entry', async () => {
  const view = new AppView();
  const root = mount(() => <WorkbookShell view={view}/>);
  const locate = vi.fn(() => true);
  view.onLocate(locate);
  view.showRevision('Ready', 2);
  view.recordActivity({ id: 'event:1', actor: 'ai', title: '셀 수정', state: 'committed', sheetTarget: { sheet: '업무!현황', range: 'A4' } });
  button(root, '활동').click();
  button(root, '변경 위치로 이동').click();
  await settle();
  expect(locate).toHaveBeenCalledWith({ sheet: '업무!현황', range: 'A4', state: 'selecting' });
  expect(document.activeElement).toBe(view.spreadsheet);
  view.dispose();
});


it('returns keyboard focus to the activity trigger on Escape', async () => {
  const view = new AppView();
  const root = mount(() => <WorkbookShell view={view}/>);
  const trigger = button(root, '활동');
  trigger.click();
  await settle();
  expect(document.activeElement?.id).toBe('document-activity-heading');
  document.activeElement?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  await settle();
  expect(root.querySelector('#document-activity')).toBeNull();
  expect(document.activeElement).toBe(trigger);
  expect(trigger.hasAttribute('aria-controls')).toBe(false);
});

it('clears the visible AI presence when the session reports idle', () => {
  const view = new AppView();
  const root = mount(() => <WorkbookShell view={view}/>);
  const event = { type: 'presence.update' as const, actor: 'ai', sheet: 'Sheet1', range: 'A1', revision: 1, sequence: 1 };
  view.showAIPresence({ ...event, state: 'editing' }, 1);
  expect(root.querySelector('#ai-presence')?.textContent).toContain('AI 편집 중');
  view.showAIPresence({ ...event, sequence: 2, state: 'idle' }, 1);
  expect(root.querySelector('#ai-presence')).toBeNull();
  view.dispose();
});


function rich(root: ParentNode): HTMLElement { return root.querySelector<HTMLElement>('[data-continuous-editor]')!; }
function spans(root: ParentNode): HTMLElement[] { return [...root.querySelectorAll<HTMLElement>('[data-docx-run]')]; }
function selectRich(root: ParentNode, first: number, start: number, last = first, end = start) {
  const fields = spans(root);
  restoreTextSelection(rich(root), { anchor: { runId: fields[first]!.dataset.docxRun!, offset: start }, focus: { runId: fields[last]!.dataset.docxRun!, offset: end } });
}
function typeRich(root: ParentNode, index: number, text: string) {
  const span = spans(root)[index]!; span.textContent = text;
  span.dispatchEvent(new InputEvent('input', { inputType: 'insertText', bubbles: true }));
}
it('keeps split paragraphs as drafts, preserves typing and restores the caret without saving per Enter', async () => {
  const initial: Snapshot = { ...snapshot, blocks: [{ id: 'p', type: 'paragraph', paragraph: { id: 'p', structureEditable: true, runs: [{ id: 'r', text: '앞뒤', editable: true, bold: true, italic: false }] } }] };
  const f = docxFixture(initial); const root = mount(() => <DocumentApp/>); await settle();
  const original = spans(root)[0]; typeRich(root, 0, '앞중간뒤'); expect(spans(root)[0]).toBe(original);
  selectRich(root, 0, 1, 0, 3);
  rich(root).dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })); await settle();
  expect([...root.querySelectorAll('.docx-paragraph')].map(p => p.textContent)).toEqual(['앞', '뒤']);
  expect(readTextSelection(rich(root))?.focus.offset).toBe(0);
  selectRich(root, 1, 0, 2, 1);
  rich(root).dispatchEvent(new InputEvent('beforeinput', { inputType: 'insertText', data: '이어쓰기', bubbles: true, cancelable: true })); await settle();
  expect(f.fetcher.mock.calls.filter(([, init]) => init?.method === 'POST')).toHaveLength(0);
  button(root, '저장').click(); await settle();
  const body = JSON.parse(f.fetcher.mock.calls.find(([, init]) => init?.method === 'POST')![1]!.body as string);
  expect(body.operations).toEqual([{ type: 'replace_paragraphs', targetId: 'p', endTargetId: 'p', expectedText: '앞뒤', text: '', paragraphs: [
    { sourceId: 'p', runs: [{ sourceId: 'r', text: '앞' }] }, { sourceId: 'p', runs: [{ sourceId: 'r', text: '이어쓰기' }] },
  ] }]);
});
it('does not split a paragraph while IME composition owns Enter', async () => {
  const initial: Snapshot = { ...snapshot, blocks: [{ id: 'p', type: 'paragraph', paragraph: { id: 'p', structureEditable: true, runs: [{ id: 'r', text: '', editable: true, bold: false, italic: false }] } }] };
  docxFixture(initial); const root = mount(() => <DocumentApp/>); await settle(); selectRich(root, 0, 0);
  rich(root).dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
  const span = spans(root)[0]!; span.textContent = '한';
  rich(root).dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
  expect(spans(root)[0]).toBe(span); expect(span.textContent).toBe('한');
  rich(root).dispatchEvent(new CompositionEvent('compositionend', { data: '한', bubbles: true })); await settle();
  selectRich(root, 0, 1); rich(root).dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })); await settle();
  expect(root.querySelectorAll('.docx-paragraph')).toHaveLength(2);
});
it('uses mobile beforeinput paragraph intent and preserves structure drafts after a lost response', async () => {
  const initial: Snapshot = { ...snapshot, blocks: [{ id: 'p', type: 'paragraph', paragraph: { id: 'p', structureEditable: true, runs: [{ id: 'r', text: 'mobile', editable: true, bold: false, italic: false }] } }] };
  const f = docxFixture(initial); const root = mount(() => <DocumentApp/>); await settle(); selectRich(root, 0, 6);
  rich(root).dispatchEvent(new InputEvent('beforeinput', { inputType: 'insertLineBreak', bubbles: true, cancelable: true })); await settle();
  expect(root.querySelectorAll('.docx-paragraph')).toHaveLength(2);
  typeRich(root, 1, 'recover draft');
  expect(spans(root)[1]!.querySelector('[data-docx-empty]')).toBeNull();
  f.fetcher.mockRejectedValueOnce(new Error('response lost'));
  button(root, '저장').click(); await settle();
  expect([...root.querySelectorAll('.docx-paragraph')].map(p => p.textContent)).toEqual(['mobile','recover draft']);
  expect(button(root, '저장').disabled).toBe(true);
  expect(root.querySelector('#save-status')?.getAttribute('data-state')).toBe('failed');
});

it('offers plain and basic style paste and never deletes selected text for an image-only clipboard', async () => {
  const initial: Snapshot = { ...snapshot, blocks: [{ id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[{id:'r',text:'keep',editable:true,bold:true,italic:false}]}}] };
  const f=docxFixture(initial);const root=mount(()=><DocumentApp/>);await settle();
  const options=root.querySelector<HTMLSelectElement>('[aria-label="붙여넣기 서식"]')!;
  options.value='basic';options.dispatchEvent(new Event('change',{bubbles:true}));
  selectRich(root,0,0,0,4);
  const image=new DataTransfer(); image.items.add(new File(['bytes'],'image.png',{type:'image/png'}));
  rich(root).dispatchEvent(new ClipboardEvent('paste',{clipboardData:image,bubbles:true,cancelable:true}));
  expect(spans(root)[0]?.textContent).toBe('keep');
  const clipboard=new DataTransfer();clipboard.setData('text/plain','new');clipboard.setData('text/html','<p><em style="color:#112233">new</em></p>');
  rich(root).dispatchEvent(new ClipboardEvent('paste',{clipboardData:clipboard,bubbles:true,cancelable:true}));await settle();
  expect(spans(root)[0]?.textContent).toBe('new');expect(spans(root)[0]?.style.fontStyle).toBe('italic');
  button(root,'저장').click();await settle();
  const body=JSON.parse(f.fetcher.mock.calls.find(([,init])=>init?.method==='POST')![1]!.body as string);
  expect(body.operations[0].paragraphs[0].runs[0].style).toEqual({bold:false,italic:true,color:'#112233'});
});
it('preserves selected source text when writing a cut clipboard fails', async () => {
  const initial: Snapshot = { ...snapshot, blocks: [{ id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[{id:'r',text:'keep',editable:true,bold:false,italic:false}]}}] };
  docxFixture(initial);const root=mount(()=><DocumentApp/>);await settle();selectRich(root,0,0,0,4);
  const clipboard=new DataTransfer();clipboard.setData=()=>{throw new Error('clipboard denied');};
  rich(root).dispatchEvent(new ClipboardEvent('cut',{clipboardData:clipboard,bubbles:true,cancelable:true}));await settle();
  expect(spans(root)[0]?.textContent).toBe('keep');expect(root.querySelector('#save-status')?.textContent).toBe('저장됨');
});

it('restores the entire reversed selection when IME composition is cancelled', async () => {
  const initial: Snapshot = { ...snapshot, blocks: [{ id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[
    {id:'r1',text:'앞부분',editable:true,bold:true,italic:false}, {id:'r2',text:'뒷부분',editable:true,bold:false,italic:true},
  ]}}] };
  docxFixture(initial); const root=mount(()=><DocumentApp/>); await settle();
  selectRich(root,1,2,0,1);
  const before=readTextSelection(rich(root));
  rich(root).dispatchEvent(new CompositionEvent('compositionstart',{bubbles:true}));
  spans(root)[0]!.textContent='임시';
  rich(root).dispatchEvent(new CompositionEvent('compositionend',{data:'',bubbles:true})); await settle();
  expect(spans(root).map(s=>s.textContent)).toEqual(['앞부분','뒷부분']);
  expect(readTextSelection(rich(root))).toEqual(before);
});

it('uses one draft history for range edits, native undo intent and keyboard redo', async () => {
  const initial:Snapshot={...snapshot,blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[{id:'r',text:'앞뒤',editable:true,bold:true,italic:false}]}}]};
  const f=docxFixture(initial);const root=mount(()=><DocumentApp/>);await settle();selectRich(root,0,1);
  rich(root).dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true,cancelable:true}));await settle();
  expect(root.querySelectorAll('.docx-paragraph')).toHaveLength(2);
  const undo=new InputEvent('beforeinput',{inputType:'historyUndo',bubbles:true,cancelable:true});rich(root).dispatchEvent(undo);await settle();
  expect(undo.defaultPrevented).toBe(true);expect(root.querySelectorAll('.docx-paragraph')).toHaveLength(1);
  expect(readTextSelection(rich(root))?.focus.offset).toBe(1);
  rich(root).dispatchEvent(new KeyboardEvent('keydown',{key:'z',ctrlKey:true,shiftKey:true,bubbles:true,cancelable:true}));await settle();
  expect(root.querySelectorAll('.docx-paragraph')).toHaveLength(2);
  expect(f.fetcher.mock.calls.filter(([,init])=>init?.method==='POST')).toHaveLength(0);
  expect(button(root,'실행 취소').disabled).toBe(false);
});

it('reports mixed formatting and writes only the selected range through the toolbar',async()=>{
 const initial:Snapshot={...snapshot,blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[{id:'a',text:'AB',editable:true,bold:true,italic:false},{id:'b',text:'CD',editable:true,bold:false,italic:false}]}}]};
 const f=docxFixture(initial);const root=mount(()=><DocumentApp/>);await settle();selectRich(root,0,1,1,1);document.dispatchEvent(new Event('selectionchange'));await settle();
 expect(button(root,'굵게').getAttribute('aria-pressed')).toBe('mixed');
 button(root,'밑줄').click();await settle();
 expect(spans(root).map(s=>s.textContent)).toEqual(['A','B','C','D']);
 expect(spans(root)[1]?.style.textDecoration).toBe('underline');expect(spans(root)[2]?.style.textDecoration).toBe('underline');
 expect(button(root,'밑줄').getAttribute('aria-pressed')).toBe('true');
 button(root,'저장').click();await settle();
 const body=JSON.parse(f.fetcher.mock.calls.find(([,init])=>init?.method==='POST')![1]!.body as string);
 expect(body.operations[0].paragraphs[0].runs.map((r:any)=>r.style)).toEqual([undefined,{underline:true},{underline:true},undefined]);
});

it('saves paragraph alignment and indent through the document toolbar',async()=>{
 const initial:Snapshot={...snapshot,blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[{id:'r',text:'문단',editable:true,bold:false,italic:false}]}}]};
 const f=docxFixture(initial);const root=mount(()=><DocumentApp/>);await settle();selectRich(root,0,1);document.dispatchEvent(new Event('selectionchange'));await settle();
 const alignment=root.querySelector<HTMLSelectElement>('[aria-label="문단 정렬"]')!;alignment.value='center';alignment.dispatchEvent(new Event('change',{bubbles:true}));await settle();
 const indent=root.querySelector<HTMLInputElement>('[aria-label="첫 줄 들여쓰기"]')!;indent.value='-12';indent.dispatchEvent(new Event('change',{bubbles:true}));await settle();
 expect(root.querySelector<HTMLElement>('.docx-paragraph')?.style.textIndent).toBe('-12pt');
 button(root,'저장').click();await settle();const body=JSON.parse(f.fetcher.mock.calls.find(([,init])=>init?.method==='POST')![1]!.body as string);
 expect(body.operations[0].paragraphs[0].format).toEqual({align:'center',hanging:240});
});

it('remaps the cached toolbar selection after save when native selection was cleared',async()=>{
 const initial:Snapshot={...snapshot,blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[{id:'r',text:'문단',editable:true,bold:false,italic:false}]}}]};
 const f=docxFixture(initial);const root=mount(()=><DocumentApp/>);await settle();selectRich(root,0,1);document.dispatchEvent(new Event('selectionchange'));await settle();
 button(root,'밑줄').click();await settle();
 const saved:Snapshot={...initial,revision:2,blocks:[{id:'new-p',type:'paragraph',paragraph:{id:'new-p',structureEditable:true,runs:[{id:'new-r',text:'문단',editable:true,bold:false,italic:false}]}}]};
 f.fetcher.mockResolvedValueOnce({ok:true,json:async()=>saved});
 document.getSelection()!.removeAllRanges();button(root,'저장').click();await settle();
 expect(readTextSelection(rich(root))?.focus.runId).toBe('new-r');
 const align=root.querySelector<HTMLSelectElement>('[aria-label="문단 정렬"]')!;expect(align.disabled).toBe(false);
 align.value='center';align.dispatchEvent(new Event('change',{bubbles:true}));await settle();
 expect(root.querySelector<HTMLElement>('.docx-paragraph > div')?.style.textAlign).toBe('center');
});

it('creates a style definition draft in the real dialog and applies it in the same save',async()=>{
 const show=vi.spyOn(HTMLDialogElement.prototype,'showModal').mockImplementation(function(this:HTMLDialogElement){this.open=true;});
 const close=vi.spyOn(HTMLDialogElement.prototype,'close').mockImplementation(function(this:HTMLDialogElement){this.open=false;});
 try{
  const initial:Snapshot={...snapshot,styles:[],blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[{id:'r',text:'제목',editable:true,bold:false,italic:false}]}}]};
  const f=docxFixture(initial);const root=mount(()=><DocumentApp/>);await settle();selectRich(root,0,1);document.dispatchEvent(new Event('selectionchange'));await settle();
  button(root,'스타일 정의 편집').click();await settle();expect(show).toHaveBeenCalledOnce();
  button(root,'스타일 추가').click();await settle();
  const name=root.querySelector<HTMLInputElement>('[aria-label="스타일 이름"]')!;name.value='새 제목';name.dispatchEvent(new Event('change',{bubbles:true}));await settle();
  button(root,'상속 굵게 반전').click();await settle();button(root,'닫기').click();await settle();
  const apply=root.querySelector<HTMLSelectElement>('[aria-label="문단 스타일 적용"]')!;const option=[...apply.options].find(o=>o.textContent?.startsWith('새 제목'))!;
  apply.value=option.value;apply.dispatchEvent(new Event('change',{bubbles:true}));await settle();
  expect(root.textContent).toContain('스타일 미리 보기는 저장 후 갱신됩니다.');
  button(root,'저장').click();await settle();const body=JSON.parse(f.fetcher.mock.calls.find(([,init])=>init?.method==='POST')![1]!.body as string);
  expect(body.operations.map((op:any)=>op.type)).toEqual(['define_style','replace_paragraphs']);
  expect(body.operations[0].styleDefinition).toMatchObject({name:'새 제목',kind:'paragraph',run:{bold:true}});
  expect(body.operations[1].paragraphs[0].format.styleId).toBe(body.operations[0].styleDefinition.id);
 }finally{show.mockRestore();close.mockRestore();}
});

it('keeps the newly created style selected after a second definition is inserted',async()=>{
 const show=vi.spyOn(HTMLDialogElement.prototype,'showModal').mockImplementation(function(this:HTMLDialogElement){this.open=true;});
 const close=vi.spyOn(HTMLDialogElement.prototype,'close').mockImplementation(function(this:HTMLDialogElement){this.open=false;});
 try{
  docxFixture({...snapshot,styles:[]});const root=mount(()=><DocumentApp/>);await settle();button(root,'스타일 정의 편집').click();await settle();
  button(root,'스타일 추가').click();await settle();
  const name=root.querySelector<HTMLInputElement>('[aria-label="새 스타일 이름"]')!;name.value='두 번째';name.dispatchEvent(new Event('input',{bubbles:true}));
  button(root,'스타일 추가').click();await settle();
  const picker=root.querySelector<HTMLSelectElement>('[aria-label="편집할 스타일"]')!;
  expect(picker.selectedOptions[0]!.textContent).toContain('두 번째');
  button(root,'닫기').click();await settle();button(root,'스타일 정의 편집').click();await settle();
  expect(root.querySelector<HTMLSelectElement>('[aria-label="편집할 스타일"]')!.selectedOptions[0]!.textContent).toContain('두 번째');
 }finally{show.mockRestore();close.mockRestore();}
});
