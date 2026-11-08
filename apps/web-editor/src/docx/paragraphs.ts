import type { Snapshot } from './contracts';

export function paragraphs(snapshot: Snapshot) {
  return snapshot.blocks.flatMap((block, index) => block.type === 'paragraph'
    ? [{ key: `${index}`, label: `본문 문단 ${index + 1}`, paragraph: block.paragraph }]
    : block.rows.flatMap((row, r) => row.cells.flatMap((cell, c) => cell.paragraphs.map((paragraph, p) => ({
        key: `${index}-${r}-${c}-${p}`, label: `표 ${r + 1}행 ${c + 1}열`, paragraph,
      })))));
}

export function compareParagraphs(previous: Snapshot, next: Snapshot, textOnly = false) {
  if (previous.id !== next.id || next.revision !== previous.revision + 1) return [];
  const before = paragraphs(previous), after = paragraphs(next);
  const beforeByAnchor = new Map(before.flatMap(entry => entry.paragraph.anchor ? [[entry.paragraph.anchor,entry] as const]:[]));
  const sameTopology = textOnly && before.length === after.length &&
    before.every((entry,index)=>entry.key===after[index]!.key);
  const preview = (value:string) => value.length>280 ? `${value.slice(0,280)}…`:value;
  return after.flatMap((entry,index)=>{
    const anchor=entry.paragraph.anchor;
    const old=anchor ? beforeByAnchor.get(anchor) : sameTopology ? before[index] : undefined;
    if(!old)return [];
    const oldText=old.paragraph.runs.map(run=>run.text).join('');
    const newText=entry.paragraph.runs.map(run=>run.text).join('');
    if(oldText===newText)return [];
    return [{key:entry.key,anchor,label:entry.label,before:preview(oldText),after:preview(newText)}];
  });
}
