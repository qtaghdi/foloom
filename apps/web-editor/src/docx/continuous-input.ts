import type { Snapshot, Run, StructureAction, TextOperation } from './contracts';
import { readTextSelection, restoreTextSelection, targetTextSelection, type TextSelection } from './selection';
import { validateRunText } from './validation';

type InputCallbacks = {
  snapshot: () => Snapshot; saving: () => boolean;
  edit: (operation: TextOperation) => void;
  replace: (selection: TextSelection, text: string, inline?: boolean) => boolean;
  delete: (selection: TextSelection, direction: 'backward' | 'forward', word: boolean) => boolean;
  structure: (action: StructureAction, id: string, start: number, end: number) => boolean;
  composition: (active: boolean) => void;
  refresh: (selection: TextSelection) => void;
  text: (selection: TextSelection) => string | undefined;
  focus: (run: Run) => void;
  notice: (message: 'protected' | 'invalid' | '') => void;
};

// Native selection and within-run text input remain browser-owned. Structural
// edits cross formatting boundaries through the same revision-safe draft model.
// Composition owns the DOM until its final text is applied once at compositionend.
export function continuousInput(root: () => HTMLElement, callbacks: InputCallbacks) {
  let composition: TextSelection | undefined;
  let softBreak = false;
  let composed = false;
  const eligible = (event: Event) => !(event.target as Element | null)?.closest('textarea, button, input, select');
  const selection = () => readTextSelection(root());
  const apply = (range: TextSelection | undefined, text: string, inline = false) => {
    if (!range || !callbacks.replace(range, text, inline)) { callbacks.notice('protected'); return false; }
    callbacks.notice(''); return true;
  };
  const runs = () => callbacks.snapshot().blocks.flatMap(block => (block.type === 'paragraph' ? [block.paragraph] : block.rows.flatMap(r => r.cells.flatMap(c => c.paragraphs)))
    .flatMap(p => p.runs.map(run => ({ run, type: block.type === 'paragraph' ? 'replace_run_text' as const : 'replace_cell_text' as const }))));
  const sync = () => {
    const current = new Map(runs().map(item => [item.run.id, item]));
    for (const span of root().querySelectorAll<HTMLElement>('[data-docx-run]')) {
      const item = current.get(span.dataset.docxRun!); if (!item) continue;
      const text = span.textContent ?? '';
      if (text) span.querySelector('[data-docx-empty]')?.remove();
      if (text !== item.run.text) callbacks.edit({ type: item.type, targetId: item.run.id, expectedText: item.run.text, text });
      if (validateRunText(text)) callbacks.notice('invalid');
    }
    const range = selection(); const focused = range && current.get(range.focus.runId);
    if (focused) callbacks.focus(focused.run);
  };
  return {
    keydown(event: KeyboardEvent) {
      if (!eligible(event) || composition || event.isComposing || callbacks.saving()) return;
      const range = selection(); if (!range) return;
      softBreak = event.key === 'Enter' && event.shiftKey;
      if (event.key === 'Enter') { event.preventDefault(); apply(range, '\n', softBreak); return; }
      if (event.key === 'Backspace' || event.key === 'Delete') {
        event.preventDefault(); callbacks.delete(range, event.key === 'Backspace' ? 'backward' : 'forward', event.altKey || event.ctrlKey); return;
      }
      if (event.altKey && event.shiftKey && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) {
        event.preventDefault(); callbacks.structure(event.key === 'ArrowUp' ? 'moveUp' : 'moveDown', range.focus.runId, range.focus.offset, range.focus.offset);
      }
    },
    beforeinput(event: InputEvent) {
      if (!eligible(event) || callbacks.saving() || !event.cancelable) return;
      if (composition || event.isComposing) return;
      if (event.inputType.includes('Composition') || composed && event.inputType === 'insertText') { event.preventDefault(); composed = false; return; }
      const target = event.getTargetRanges?.()[0];
      const range = target ? targetTextSelection(root(), target) : selection();
      if (!range) { event.preventDefault(); return; }
      if (event.inputType === 'insertText' || event.inputType === 'insertReplacementText') {
        const text = event.data ?? '';
        if (validateRunText(text)) { event.preventDefault(); callbacks.notice('invalid'); return; }
        if (range.anchor.runId === range.focus.runId) {
          // Empty spans may have a BR placeholder rather than a text node.
          // Normalize that position before letting the browser insert text.
          const actual = root().ownerDocument.getSelection()?.anchorNode;
          if (actual?.nodeType !== Node.TEXT_NODE) restoreTextSelection(root(), range);
          return;
        }
        event.preventDefault(); apply(range, text); return;
      }
      if (event.inputType === 'insertParagraph' || event.inputType === 'insertLineBreak') {
        event.preventDefault(); apply(range, '\n', softBreak); softBreak = false; return;
      }
      if (event.inputType.startsWith('delete')) {
        event.preventDefault(); callbacks.delete(range, event.inputType.endsWith('Forward') ? 'forward' : 'backward', event.inputType.includes('Word')); return;
      }
      if (!event.inputType.startsWith('history')) event.preventDefault();
    },
    input(event: InputEvent) { if (eligible(event) && !composition && !event.isComposing) sync(); },
    compositionstart(event: CompositionEvent) {
      if (!eligible(event) || callbacks.saving()) return;
      composition = selection(); if (composition) callbacks.composition(true);
    },
    compositionend(event: CompositionEvent) {
      if (!composition) return;
      const original = composition; composition = undefined;
      if (event.data) apply(original, event.data);
      else callbacks.refresh(original);
      composed = true; queueMicrotask(() => { composed = false; });
      callbacks.composition(false);
    },
    paste(event: ClipboardEvent) {
      if (!eligible(event)) return;
      event.preventDefault(); if (callbacks.saving() || composition) return;
      const text = event.clipboardData?.getData('text/plain');
      if (text !== undefined) apply(selection(), text.replace(/\r\n?/g, '\n'));
    },
    copy(event: ClipboardEvent) {
      if (!eligible(event)) return;
      const range = selection(), text = range && callbacks.text(range);
      if (text === undefined || !event.clipboardData) return;
      event.preventDefault(); event.clipboardData.setData('text/plain', text);
    },
    cut(event: ClipboardEvent) {
      if (!eligible(event)) return;
      event.preventDefault(); if (callbacks.saving() || composition || !event.clipboardData) return;
      const range = selection(), text = range && callbacks.text(range);
      if (text === undefined || !range) return;
      // Deletion is committed to the local draft only after clipboard writing
      // succeeds. A failed clipboard operation must not destroy selected text.
      try { event.clipboardData.setData('text/plain', text); apply(range, ''); } catch { callbacks.notice('protected'); }
    },
    drop(event: DragEvent) { if (eligible(event)) event.preventDefault(); },
    focus() {
      const range = selection(); const run = range && runs().find(item => item.run.id === range.focus.runId)?.run;
      if (run) callbacks.focus(run);
    },
  };
}
