import type { Snapshot } from './contracts';

export function paragraphs(snapshot: Snapshot) {
  return snapshot.blocks.flatMap((block, index) => block.type === 'paragraph'
    ? [{ key: `${index}`, label: `본문 문단 ${index + 1}`, paragraph: block.paragraph }]
    : block.rows.flatMap((row, r) => row.cells.flatMap((cell, c) => cell.paragraphs.map((paragraph, p) => ({
        key: `${index}-${r}-${c}-${p}`, label: `표 ${r + 1}행 ${c + 1}열`, paragraph,
      })))));
}

export function compareParagraphs(previous: Snapshot, next: Snapshot) {
  // Snapshot IDs change on every revision. Positions are safe to compare for a
  // single observed text-only commit in the same session.
  if (previous.id !== next.id || next.revision !== previous.revision + 1) return [];
  const before = paragraphs(previous);
  const after = paragraphs(next);
  if (before.length !== after.length || before.some((entry, i) =>
    entry.key !== after[i]!.key || entry.paragraph.runs.length !== after[i]!.paragraph.runs.length)) return [];
  return after.flatMap((entry, i) => {
    const oldText = before[i]!.paragraph.runs.map(run => run.text).join('');
    const newText = entry.paragraph.runs.map(run => run.text).join('');
    if (oldText === newText) return [];
    const preview = (value: string) => value.length > 280 ? `${value.slice(0, 280)}…` : value;
    return [{ key: entry.key, label: entry.label, before: preview(oldText), after: preview(newText) }];
  });
}
