import { expect, it } from 'vitest';
import { StructureDraft } from '../src/docx/structure-draft';
import type { Snapshot } from '../src/docx/contracts';

const snapshot: Snapshot = { id: 'session', revision: 1, sequence: 0, name: 'test.docx', warnings: [], blocks: [
  { id: 'p1', type: 'paragraph', paragraph: { id: 'p1', structureEditable: true, runs: [
    { id: 'r1', text: '한글 😀', editable: true, bold: true, italic: false },
    { id: 'r2', text: 'styled', editable: true, bold: false, italic: true },
  ] } },
  { id: 'p2', type: 'paragraph', paragraph: { id: 'p2', structureEditable: true, runs: [{ id: 'r3', text: 'next', editable: true, bold: false, italic: false }] } },
] };
it('keeps source text immutable and composes typing, repeated splits, merge and movement before one commit', () => {
  const draft = new StructureDraft(snapshot);
  draft.edit('r1', '새 한글 😀');
  const focus = draft.change('split', 'r1', 2, 2)!;
  expect(focus.start).toBe(0);
  expect(draft.snapshot().blocks).toHaveLength(3);
  const second = draft.change('split', focus.runId, 0, 0, '붙여\r\n넣기\n')!;
  draft.edit(second.runId, '최종 초안');
  expect(draft.change('mergeBackward', second.runId, 0, 0)).toBeDefined();
  expect(draft.change('moveUp', second.runId, 0, 0)).toBeDefined();
  const [op] = draft.operations();
  expect(op?.expectedText).toBe('한글 😀styled\nnext');
  expect(op?.paragraphs.flatMap(p => p.runs).find(r => r.text === '최종 초안')?.sourceId).toBe('r1');
  expect(snapshot.blocks[0]?.type === 'paragraph' && snapshot.blocks[0].paragraph.runs[0]?.text).toBe('한글 😀');
  expect(draft.ownsOriginal('r2')).toBe(true);
});
it('does not split surrogate pairs or cross a protected paragraph or table cell', () => {
  const draft = new StructureDraft(snapshot);
  expect(draft.change('split', 'r1', 4, 4)).toBeUndefined();
  expect(draft.change('mergeBackward', 'r1', 0, 0)).toBeUndefined();
  const protectedDoc: Snapshot = { ...snapshot, blocks: snapshot.blocks.map(b => b.type === 'paragraph' ? { ...b, paragraph: { ...b.paragraph, structureEditable: false } } : b) };
  expect(new StructureDraft(protectedDoc).change('split', 'r1', 0, 0)).toBeUndefined();
});

it('keeps text drafts outside the structurally edited group when projecting a new snapshot', () => {
  const first = snapshot.blocks[0]!;
  if (first.type !== 'paragraph') throw new Error('paragraph fixture required');
  const input: Snapshot = { ...snapshot, blocks: [{ ...first, paragraph: { ...first.paragraph, structureEditable: false } }, snapshot.blocks[1]!] };
  const draft = new StructureDraft(input);
  draft.edit('r1', 'protected paragraph draft');
  draft.change('split', 'r3', 0, 0);
  const projected = draft.snapshot().blocks[0]!;
  expect(projected.type === 'paragraph' && projected.paragraph.runs[0]?.text).toBe('protected paragraph draft');
  expect(draft.ownsOriginal('r1')).toBe(false);
});

it('moves selected plain body paragraphs but excludes section, page and list boundaries', () => {
  const first = snapshot.blocks[0]!;
  const second = snapshot.blocks[1]!;
  if (first.type !== 'paragraph' || second.type !== 'paragraph') throw new Error('body paragraph fixture required');
  const backwards = { anchor: { runId: 'r2', offset: 3 }, focus: { runId: 'r1', offset: 1 } };
  const draft = new StructureDraft(snapshot);
  expect(draft.canMoveBodyParagraph(-1, backwards)).toBe(false);
  expect(draft.canMoveBodyParagraph(1, backwards)).toBe(true);
  draft.change('moveDown', 'r1', 1, 1);
  expect(draft.snapshot().blocks.map(block => block.id)).toEqual(['p2', 'p1']);
  expect(draft.canMoveBodyParagraph(1, backwards)).toBe(false);
  expect(draft.canMoveBodyParagraph(-1, backwards)).toBe(true);
  expect(draft.operations()[0]?.paragraphs.map(paragraph => paragraph.sourceId)).toEqual(['p2', 'p1']);
  for (const boundary of [{ sectionId: 'different' }, { pageBreakBefore: true }, { pageBreakAfter: true }, { list: { id: 4 } }, { structureEditable: false }]) {
    const guarded = new StructureDraft({ ...snapshot, blocks: [first, { ...second, paragraph: { ...second.paragraph, ...boundary } }] });
    expect(guarded.canMoveBodyParagraph(1, backwards)).toBe(false);
  }
  expect(draft.canMoveBodyParagraph(1, { ...backwards, focus: { runId: 'r3', offset: 0 } })).toBe(false);
});
