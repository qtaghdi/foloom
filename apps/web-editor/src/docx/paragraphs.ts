import type { Snapshot } from './contracts';
import type { TextPoint } from './selection';

export function paragraphs(snapshot: Snapshot) {
  return snapshot.blocks.flatMap((block, index) => block.type === 'paragraph'
    ? [{ key: `${index}`, label: `본문 문단 ${index + 1}`, paragraph: block.paragraph }]
    : block.rows.flatMap((row, r) => row.cells.flatMap((cell, c) => cell.paragraphs.map((paragraph, p) => ({
        key: `${index}-${r}-${c}-${p}`, label: `표 ${r + 1}행 ${c + 1}열`, paragraph,
      })))));
}

export function restoreAnchoredPoint(previous: Snapshot, next: Snapshot, point: TextPoint): TextPoint | null | undefined {
  if (previous.id!==next.id) return null;
  const previousParagraphs=paragraphs(previous);
  const source=previousParagraphs.find(({paragraph})=>paragraph.runs.some(run=>run.id===point.runId))?.paragraph;
  if (!source?.anchor) return undefined;
  if (previousParagraphs.filter(({paragraph})=>paragraph.anchor===source.anchor).length!==1) return undefined;
  const targets=paragraphs(next).filter(({paragraph})=>paragraph.anchor===source.anchor);
  if (targets.length!==1) return null;
  const target=targets[0]!.paragraph;
  const index=source.runs.findIndex(run=>run.id===point.runId);
  const run=target.runs[Math.min(index,target.runs.length-1)];
  if (!run?.editable) return null;
  return {runId:run.id,offset:Math.min(point.offset,run.text.length)};
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
