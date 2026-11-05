import type { Snapshot, Paragraph, Run, ParagraphOperation, DraftFocus, StructureAction } from './contracts';
import { validateRunText } from './validation';

type DraftRun = Run & { sourceId: string };
type DraftParagraph = Omit<Paragraph, 'runs'> & { sourceId: string; runs: DraftRun[] };
type Group = { originals: Paragraph[]; current: DraftParagraph[]; changed: boolean };

// Local identities survive splits and moves until commit. Source identities are
// revision-scoped formatting references and are never submitted as new OOXML IDs.
export class StructureDraft {
  private serial = 0;
  private otherText = new Map<string, string>();
  private groups: Group[] = [];
  private paragraphGroup = new Map<string, Group>();
  constructor(private readonly original: Snapshot) {
    const collect = (paragraphs: Paragraph[]) => {
      let group: Group | undefined;
      for (const paragraph of paragraphs) {
        if (!paragraph.structureEditable) { group = undefined; continue; }
        if (!group || group.originals.length >= 500) { group = { originals: [], current: [], changed: false }; this.groups.push(group); }
        group.originals.push(paragraph);
        group.current.push({ ...paragraph, sourceId: paragraph.id, runs: paragraph.runs.map(run => ({ ...run, sourceId: run.id })) });
        this.paragraphGroup.set(paragraph.id, group);
      }
    };
    let body: Paragraph[] = [];
    for (const block of original.blocks) {
      if (block.type === 'paragraph') body.push(block.paragraph);
      else { collect(body); body = []; for (const row of block.rows) for (const cell of row.cells) collect(cell.paragraphs); }
    }
    collect(body);
  }
  private id(): string { return `draft:${++this.serial}`; }
  private find(runId: string) {
    for (const group of this.groups) for (const [p, paragraph] of group.current.entries()) {
      const r = paragraph.runs.findIndex(run => run.id === runId);
      if (r >= 0) return { group, p, r, paragraph, run: paragraph.runs[r]! };
    }
  }
  // Text inputs stay mounted while typing and IME composition is in progress.
  edit(runId: string, text: string): boolean {
    const found = this.find(runId);
    if (!found) { this.otherText.set(runId, text); return false; }
    found.run.text = text;
    return found.group.changed;
  }
  hasChanges(): boolean { return this.groups.some(group => group.changed); }
  operations(): ParagraphOperation[] {
    return this.groups.filter(group => group.changed).map(group => ({
      type: 'replace_paragraphs', targetId: group.originals[0]!.id, endTargetId: group.originals.at(-1)!.id,
      expectedText: group.originals.map(p => p.runs.map(r => r.text).join('')).join('\n'), text: '',
      paragraphs: group.current.map(p => ({ sourceId: p.sourceId, runs: p.runs.map(r => ({ sourceId: r.sourceId, text: r.text })) })),
    }));
  }
  ownsOriginal(runId: string): boolean {
    return this.groups.some(group => group.changed && group.originals.some(p => p.runs.some(r => r.id === runId)));
  }
  change(action: StructureAction, runId: string, start: number, end: number, insertion?: string): DraftFocus | undefined {
    const found = this.find(runId);
    if (!found || this.groups.some(g => g.current.some(p => p.runs.some(r => validateRunText(r.text))))) return;
    const { group, p, r, paragraph, run } = found;
    if (start < 0 || end < start || end > run.text.length || validateRunText(run.text.slice(0, start)) || validateRunText(run.text.slice(end))) return;
    let focus: DraftFocus;
    if (action === 'split') {
      if (insertion && insertion.length > 1024 * 1024) return;
      const pieces = (insertion ?? '\n').replace(/\r\n?/g, '\n').split('\n');
      if (pieces.length < 2 || pieces.length + group.current.length - 1 > 1000 || pieces.some(text => validateRunText(text))) return;
      const left: DraftParagraph = { ...paragraph, runs: [...paragraph.runs.slice(0, r), { ...run, text: run.text.slice(0, start) + pieces[0] }] };
      const next: DraftParagraph[] = pieces.slice(1).map(text => ({ ...paragraph, id: this.id(), runs: [{ ...run, id: this.id(), text }] }));
      const last = next.at(-1)!;
      focus = { runId: last.runs[0]!.id, start: last.runs[0]!.text.length, end: last.runs[0]!.text.length };
      last.runs[0]!.text += run.text.slice(end);
      last.runs.push(...paragraph.runs.slice(r + 1));
      group.current.splice(p, 1, left, ...next);
    } else if (action === 'mergeBackward' || action === 'mergeForward') {
      if (start !== end) return;
      const backward = action === 'mergeBackward';
      if (backward ? r !== 0 || start !== 0 || p === 0 : r !== paragraph.runs.length - 1 || start !== run.text.length || p === group.current.length - 1) return;
      const first = group.current[backward ? p - 1 : p]!;
      const second = group.current[backward ? p : p + 1]!;
      const boundary = first.runs.at(-1)!;
      focus = { runId: boundary.id, start: boundary.text.length, end: boundary.text.length };
      group.current.splice(backward ? p - 1 : p, 2, { ...first, runs: [...first.runs, ...second.runs] });
    } else {
      const next = p + (action === 'moveUp' ? -1 : 1);
      if (next < 0 || next >= group.current.length) return;
      [group.current[p], group.current[next]] = [group.current[next]!, group.current[p]!];
      focus = { runId, start, end };
    }
    group.changed = true;
    return focus;
  }
  snapshot(): Snapshot {
    const expand = (paragraphs: Paragraph[]): Paragraph[] => {
      const emitted = new Set<Group>();
      return paragraphs.flatMap(p => {
        const group = this.paragraphGroup.get(p.id);
        if (!group) return [{ ...p, runs: p.runs.map(run => this.otherText.has(run.id) ? { ...run, text: this.otherText.get(run.id)! } : run) }];
        if (emitted.has(group)) return [];
        emitted.add(group);
        return group.current;
      });
    };
    let pending: Extract<Snapshot['blocks'][number], { type: 'paragraph' }>[] = [];
    const blocks: Snapshot['blocks'] = [];
    const flush = () => { blocks.push(...expand(pending.map(b => b.paragraph)).map(paragraph => ({ id: paragraph.id, type: 'paragraph' as const, paragraph }))); pending = []; };
    for (const block of this.original.blocks) {
      if (block.type === 'paragraph') pending.push(block);
      else { flush(); blocks.push({ ...block, rows: block.rows.map(row => ({ ...row, cells: row.cells.map(cell => ({ ...cell, paragraphs: expand(cell.paragraphs) })) })) }); }
    }
    flush();
    return { ...this.original, blocks };
  }
}
