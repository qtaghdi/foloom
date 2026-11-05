export type TextPoint = { runId: string; offset: number };
export type TextSelection = { anchor: TextPoint; focus: TextPoint };

export function graphemeOffsets(text: string): number[] {
  if (typeof Intl.Segmenter === 'function') {
    return [...new Intl.Segmenter(undefined, { granularity: 'grapheme' }).segment(text)].map(s => s.index).concat(text.length);
  }
  // Older hosts cannot safely infer combining and emoji boundaries.
  return text.length ? [0, text.length] : [0];
}
export function isTextBoundary(text: string, offset: number): boolean {
  return Number.isInteger(offset) && graphemeOffsets(text).includes(offset);
}
function point(root: HTMLElement, node: Node, offset: number): TextPoint | undefined {
  if (!root.contains(node)) return;
  const element = node.nodeType === Node.ELEMENT_NODE ? node as Element : node.parentElement;
  const run = element?.closest<HTMLElement>('[data-docx-run]');
  if (run && root.contains(run)) {
    const range = root.ownerDocument.createRange();
    range.setStart(run, 0); range.setEnd(node, offset);
    return { runId: run.dataset.docxRun!, offset: range.toString().length };
  }
  const range = root.ownerDocument.createRange(); range.setStart(node, offset); range.collapse(true);
  let previous: HTMLElement | undefined;
  for (const span of root.querySelectorAll<HTMLElement>('[data-docx-run]')) {
    if (range.comparePoint(span, 0) > 0) return { runId: span.dataset.docxRun!, offset: 0 };
    previous = span;
  }
  if (previous) return { runId: previous.dataset.docxRun!, offset: previous.textContent?.length ?? 0 };
}
export function targetTextSelection(root: HTMLElement, range: Pick<StaticRange, 'startContainer' | 'startOffset' | 'endContainer' | 'endOffset'>): TextSelection | undefined {
  const anchor = point(root, range.startContainer, range.startOffset), focus = point(root, range.endContainer, range.endOffset);
  return anchor && focus ? { anchor, focus } : undefined;
}
export function readTextSelection(root: HTMLElement): TextSelection | undefined {
  const selection = root.ownerDocument.getSelection();
  if (!selection?.anchorNode || !selection.focusNode) return;
  const anchor = point(root, selection.anchorNode, selection.anchorOffset);
  const focus = point(root, selection.focusNode, selection.focusOffset);
  return anchor && focus ? { anchor, focus } : undefined;
}
export function restoreTextSelection(root: HTMLElement, value: TextSelection): boolean {
  const spans = [...root.querySelectorAll<HTMLElement>('[data-docx-run]')];
  const resolve = (point: TextPoint) => {
    const span = spans.find(s => s.dataset.docxRun === point.runId);
    if (!span) return;
    const walker = root.ownerDocument.createTreeWalker(span, NodeFilter.SHOW_TEXT);
    let offset = point.offset;
    let node = walker.nextNode();
    while (node) {
      if (offset <= (node.textContent?.length ?? 0)) return { node, offset };
      offset -= node.textContent?.length ?? 0; node = walker.nextNode();
    }
    if (point.offset === 0) { const text = root.ownerDocument.createTextNode(''); span.prepend(text); return { node: text, offset: 0 }; }
  };
  const anchor = resolve(value.anchor), focus = resolve(value.focus);
  if (!anchor || !focus) return false;
  root.focus({ preventScroll: true });
  root.ownerDocument.getSelection()?.setBaseAndExtent(anchor.node, anchor.offset, focus.node, focus.offset);
  return true;
}
