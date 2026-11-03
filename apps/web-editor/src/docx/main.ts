import '../styles.css';
import { DocumentController, type DocumentView } from './controller';
import type { Snapshot, Run, Operation } from './contracts';

function element<T extends HTMLElement>(id: string): T {
  const value = document.getElementById(id);
  if (!value) throw new Error(`Missing DOCX element: ${id}`);
  return value as T;
}

const content = element('document-content');
const save = element<HTMLButtonElement>('save-document');
const reload = element<HTMLButtonElement>('reload-document');
const status = element('save-status');
const error = element('document-error');
const name = element('document-name');
const warnings = element('document-warnings');

const view: DocumentView = {
  render(snapshot: Snapshot) {
    name.textContent = snapshot.name;
    content.replaceChildren();
    warnings.replaceChildren(...snapshot.warnings.map((message) => {
      const item = document.createElement('li'); item.textContent = message; return item;
    }));
    let position = 0;
    const renderRuns = (runs: Run[], kind: Operation['type']): HTMLElement => {
      const paragraph = document.createElement('div');
      paragraph.className = 'flex flex-wrap items-start gap-1';
      if (!runs.length) paragraph.textContent = '빈 문단';
      for (const run of runs) {
        position++;
        if (!run.editable) {
          const text = document.createElement('span');
          text.className = 'rounded bg-slate-100 px-2 py-1 whitespace-pre-wrap text-slate-600';
          text.textContent = run.text || '표시되지 않는 구조';
          text.title = '읽기 전용: 원본 구조를 보존합니다.';
          paragraph.append(text); continue;
        }
        const input = document.createElement('textarea');
        input.value = run.text;
        input.rows = Math.min(10, Math.max(1, Math.ceil(run.text.length / 55)));
        input.className = 'min-h-11 min-w-0 flex-1 basis-40 resize-y rounded border border-slate-200 px-2 py-2 leading-relaxed hover:border-blue-300 focus:border-blue-500 focus:outline-2 focus:outline-blue-200 disabled:opacity-60';
        if (run.bold) input.classList.add('font-bold');
        if (run.italic) input.classList.add('italic');
        input.setAttribute('aria-label', `${kind === 'replace_cell_text' ? '표' : '본문'} 텍스트 ${position}`);
        input.addEventListener('input', () => controller.edit({ type: kind, targetId: run.id, expectedText: run.text, text: input.value }));
        paragraph.append(input);
      }
      return paragraph;
    };
    for (const block of snapshot.blocks) {
      if (block.type === 'paragraph') content.append(renderRuns(block.paragraph.runs, 'replace_run_text'));
      else {
        const wrapper = document.createElement('div');
        wrapper.className = 'overflow-x-auto rounded border border-slate-300';
        wrapper.tabIndex = 0;
        wrapper.setAttribute('aria-label', '문서 표. 좁은 화면에서는 좌우로 스크롤하세요.');
        const table = document.createElement('table');
        table.className = 'w-full border-collapse text-left';
        const body = document.createElement('tbody');
        for (const row of block.rows) {
          const tr = document.createElement('tr');
          for (const cell of row.cells) {
            const td = document.createElement('td');
            td.className = 'min-w-40 border border-slate-200 p-2 align-top';
            for (const paragraph of cell.paragraphs) td.append(renderRuns(paragraph.runs, 'replace_cell_text'));
            tr.append(td);
          }
          body.append(tr);
        }
        table.append(body); wrapper.append(table); content.append(wrapper);
      }
    }
    content.setAttribute('aria-busy', 'false');
    if (!snapshot.blocks.length) content.textContent = '표시할 수 있는 본문이 없습니다. 원본 문서는 보존됩니다.';
  },
  status(message, state) {
    status.textContent = message;
    status.dataset.state = state;
    const pending = state === 'pending' && message === '저장 중';
    reload.disabled = pending;
    for (const input of content.querySelectorAll('textarea')) input.disabled = pending;
  },
  error(message) { error.textContent = message; error.classList.toggle('hidden', !message); },
  canSave(enabled) { save.disabled = !enabled; },
};
const controller = new DocumentController(view);
save.addEventListener('click', () => { void controller.save(); });
reload.addEventListener('click', () => {
  if (!controller.hasDrafts() || window.confirm('미저장 입력을 버리고 저장된 문서를 다시 불러올까요?')) void controller.load();
});
window.addEventListener('keydown', (event) => {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
    event.preventDefault(); void controller.save();
  }
});
window.addEventListener('beforeunload', (event) => {
  if (controller.hasDrafts()) { event.preventDefault(); event.returnValue = ''; }
});
window.addEventListener('pagehide', () => controller.dispose(), { once: true });
void controller.load();
