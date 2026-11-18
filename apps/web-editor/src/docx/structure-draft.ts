import { previewLists } from './lists';
import type { Snapshot, Paragraph, Run, ParagraphOperation, DraftFocus, StructureAction, InlineStyle, ParagraphFormat, ListDefinition } from './contracts';
import type { ClipboardParagraph } from './clipboard';
import { isTextBoundary, graphemeOffsets, type TextSelection, type TextPoint } from './selection';
import { validateRunText } from './validation';

type DraftRun = Run & { sourceId: string; style?: InlineStyle };
type DraftParagraph = Omit<Paragraph, 'runs'> & { sourceId: string; format?: ParagraphFormat; runs: DraftRun[] };
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
  currentText(runId: string): string | undefined { return this.find(runId)?.run.text ?? this.otherText.get(runId); }
  capture(): string {
    return JSON.stringify({ serial: this.serial, groups: this.groups.map(g => ({ current: g.current, changed: g.changed })), otherText: [...this.otherText] });
  }
  restore(captured: string): void {
    const state = JSON.parse(captured) as { serial: number; groups: Array<{ current: DraftParagraph[]; changed: boolean }>; otherText: Array<[string, string]> };
    this.serial = state.serial;
    state.groups.forEach((g, i) => { this.groups[i]!.current = g.current; this.groups[i]!.changed = g.changed; });
    this.otherText = new Map(state.otherText);
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
      paragraphs: group.current.map(p => ({ sourceId: p.sourceId, ...(p.format ? {format:p.format}:{}), runs: p.runs.map(r => ({ sourceId: r.sourceId, text: r.text, ...(r.style ? { style: r.style } : {}) })) })),
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
  private orderedRuns() {
    const snapshot = this.snapshot();
    return snapshot.blocks.flatMap(block => (block.type === 'paragraph' ? [block.paragraph] : block.rows.flatMap(r => r.cells.flatMap(c => c.paragraphs)))
      .flatMap(paragraph => paragraph.runs.map(run => ({ paragraph, run }))));
  }
  normalized(selection: TextSelection) {
    const runs = this.orderedRuns();
    const a = runs.findIndex(x => x.run.id === selection.anchor.runId), b = runs.findIndex(x => x.run.id === selection.focus.runId);
    if (a < 0 || b < 0) return;
    const backward = a > b || a === b && selection.anchor.offset > selection.focus.offset;
    const start = backward ? selection.focus : selection.anchor, end = backward ? selection.anchor : selection.focus;
    const first = backward ? b : a, last = backward ? a : b;
    if (!isTextBoundary(runs[first]!.run.text, start.offset) || !isTextBoundary(runs[last]!.run.text, end.offset)) return;
    return { start, end, runs: runs.slice(first, last + 1) };
  }
  selectedText(selection: TextSelection): string | undefined {
    const range = this.normalized(selection); if (!range) return;
    let previous: string | undefined;
    return range.runs.map(({ paragraph, run }, index) => {
      const separator = previous && previous !== paragraph.id ? '\n' : '';
      previous = paragraph.id;
      return separator + run.text.slice(index === 0 ? range.start.offset : 0, index === range.runs.length - 1 ? range.end.offset : undefined);
    }).join('');
  }
  deletion(selection: TextSelection, direction: 'backward' | 'forward', unit: 'grapheme' | 'word' = 'grapheme'): TextSelection | undefined {
    const normalized = this.normalized(selection); if (!normalized) return;
    if (normalized.start.runId !== normalized.end.runId || normalized.start.offset !== normalized.end.offset) return selection;
    const runs = this.orderedRuns(); const index = runs.findIndex(x => x.run.id === selection.anchor.runId);
    const { run, paragraph } = runs[index]!;
    const offset = selection.anchor.offset;
    const boundaries = graphemeOffsets(run.text);
    let target: TextPoint | undefined;
    if (direction === 'backward' && offset > 0 || direction === 'forward' && offset < run.text.length) {
      let next = direction === 'backward' ? boundaries.filter(n => n < offset).at(-1)! : boundaries.find(n => n > offset)!;
      if (unit === 'word' && typeof Intl.Segmenter === 'function') {
        const words = [...new Intl.Segmenter(undefined, { granularity: 'word' }).segment(run.text)];
        if (direction === 'backward') {
          const candidates = words.filter(word => word.index < offset);
          while (candidates.length > 1 && !candidates.at(-1)!.segment.trim()) candidates.pop();
          next = candidates.at(-1)?.index ?? 0;
        } else {
          const candidates = words.filter(word => word.index + word.segment.length > offset);
          while (candidates.length > 1 && !candidates[0]!.segment.trim()) candidates.shift();
          const word = candidates[0]; next = word ? word.index + word.segment.length : run.text.length;
        }
      }
      target = { runId: run.id, offset: next };
    } else {
      const neighbor = runs[index + (direction === 'backward' ? -1 : 1)];
      if (!neighbor || !neighbor.run.editable) return;
      const current = this.find(run.id), adjacent = this.find(neighbor.run.id);
      if (!current || !adjacent || current.group !== adjacent.group) return;
      if (neighbor.paragraph.id !== paragraph.id) target = { runId: neighbor.run.id, offset: direction === 'backward' ? neighbor.run.text.length : 0 };
      else {
        const offsets = graphemeOffsets(neighbor.run.text);
        target = { runId: neighbor.run.id, offset: direction === 'backward' ? offsets.at(-2) ?? 0 : offsets[1] ?? 0 };
      }
    }
    return { anchor: selection.anchor, focus: target };
  }
  replace(selection: TextSelection, text: string, inline = false, rich?: ClipboardParagraph[]): DraftFocus | undefined {
    if (text.length > 1024 * 1024 || text.split('\n').some(value => validateRunText(value))) return;
    const range = this.normalized(selection);
    if (!range || range.runs.some(x => !x.run.editable)) return;
    const first = this.find(range.start.runId), last = this.find(range.end.runId);
    if (!first || !last) return;
    // Cells and discontinuous body groups remain separate structural containers.
    // One selection may clear several containers, but never merge their topology.
    const spans = new Map<Group, { first: ReturnType<StructureDraft['find']> & {}; last: ReturnType<StructureDraft['find']> & {}; start: number; end: number }>();
    for (const [index, item] of range.runs.entries()) {
      const found = this.find(item.run.id); if (!found) return;
      const existing = spans.get(found.group);
      if (existing) { existing.last = found; existing.end = index === range.runs.length - 1 ? range.end.offset : found.run.text.length; }
      else spans.set(found.group, { first: found, last: found, start: index === 0 ? range.start.offset : 0, end: index === range.runs.length - 1 ? range.end.offset : found.run.text.length });
    }
    const prepared: Array<{ group: Group; from: number; count: number; paragraphs: DraftParagraph[] }> = [];
    let focus: DraftFocus | undefined;
    for (const [group, span] of spans) {
      const insertion = group === first.group ? text : '';
      const pieces = inline ? [insertion] : insertion.split('\n');
      const { first: a, last: b } = span;
      if (group.current.length - (b.p - a.p + 1) + (rich && group === first.group ? rich.length : pieces.length) > 1000) return;
      if (rich && group === first.group) {
        const prefix = { ...a.run, text: a.run.text.slice(0, span.start) };
        const result: DraftParagraph[] = rich.map((paragraph,index) => ({ ...a.paragraph, id:index===0?a.paragraph.id:this.id(), runs:paragraph.map(run => ({
          ...a.run, id:this.id(), text:run.text,
          style:{...a.run.style,...run.style}, ...run.style,
        })) }));
        if (prefix.text) result[0]!.runs.unshift(...a.paragraph.runs.slice(0,a.r),prefix);
        else result[0]!.runs.unshift(...a.paragraph.runs.slice(0,a.r));
        const final=result.at(-1)!;const inserted=final.runs.at(-1)!;
        focus={runId:inserted.id,start:inserted.text.length,end:inserted.text.length};
        const suffix={...b.run,id:result.some(p=>p.runs.some(r=>r.id===b.run.id))?this.id():b.run.id,text:b.run.text.slice(span.end)};
        if (suffix.text) final.runs.push(suffix);
        final.runs.push(...b.paragraph.runs.slice(b.r+1));
        prepared.push({group,from:a.p,count:b.p-a.p+1,paragraphs:result});
        continue;
      }
      const left = { ...a.run, text: a.run.text.slice(0, span.start) + pieces[0] };
      const result: DraftParagraph[] = [{ ...a.paragraph, runs: [...a.paragraph.runs.slice(0, a.r), left] }];
      for (const piece of pieces.slice(1)) result.push({ ...a.paragraph, id: this.id(), runs: [{ ...a.run, id: this.id(), text: piece }] });
      const final = result.at(-1)!, inserted = final.runs.at(-1)!;
      if (group === first.group) focus = { runId: inserted.id, start: inserted.text.length, end: inserted.text.length };
      const suffix = { ...b.run, id: a.run.id === b.run.id ? this.id() : b.run.id, text: b.run.text.slice(span.end) };
      // Retain a suffix run only when it carries content. A zero-width typing
      // position already uses the insertion run's original formatting source.
      if (suffix.text) final.runs.push(suffix);
      final.runs.push(...b.paragraph.runs.slice(b.r + 1));
      prepared.push({ group, from: a.p, count: b.p - a.p + 1, paragraphs: result });
    }
    for (const item of prepared) { item.group.current.splice(item.from, item.count, ...item.paragraphs); item.group.changed = true; }
    return focus;
  }
  formatParagraph(selection: TextSelection, format: ParagraphFormat): DraftFocus | undefined {
    const range=this.normalized(selection);if(!range)return;
    const found=range.runs.map(item=>this.find(item.run.id));if(found.some(item=>!item))return;
    const selected=new Set<DraftParagraph>();
    found.forEach((item,index)=>{if(index===found.length-1 && index>0 && range.end.offset===0 && !selected.has(item!.paragraph))return;selected.add(item!.paragraph);});
    for(const p of selected){
      const merged={...p.format,...format};
      if(format.firstLine!==undefined)delete merged.hanging;
      if(format.hanging!==undefined)delete merged.firstLine;
      if(format.tabs) {
        const updates=new Map((p.format?.tabs??[]).map(tab=>[tab.position,tab]));
        for(const tab of format.tabs)updates.set(tab.position,tab);
        merged.tabs=[...updates.values()].sort((a,b)=>a.position-b.position);
      }
      const previousTabs=p.layout?.tabs??[];
      p.format=merged;if(format.list!==undefined)p.list=format.list;if(format.styleId!==undefined)p.styleId=format.styleId;p.layout={...p.layout,...format};
      if(format.tabs) {
        const current=new Map((previousTabs).map(tab=>[tab.position,tab]));
        for(const tab of merged.tabs??[]){if(tab.align==='clear')current.delete(tab.position);else current.set(tab.position,tab);}
        p.layout.tabs=[...current.values()].sort((a,b)=>a.position-b.position);
      }
      if(format.firstLine!==undefined)delete p.layout.hanging;
      if(format.hanging!==undefined)delete p.layout.firstLine;
      this.paragraphGroup.get(p.sourceId)!.changed=true;
    }
    return {runId:selection.focus.runId,start:selection.focus.offset,end:selection.focus.offset,selection};
  }
  selectedParagraphs(selection:TextSelection):Paragraph[] {
    const range=this.normalized(selection);if(!range)return [];const paragraphs=new Map<string,Paragraph>();
    range.runs.forEach((item,index)=>{if(index===range.runs.length-1&&index>0&&range.end.offset===0&&!paragraphs.has(item.paragraph.id))return;paragraphs.set(item.paragraph.id,item.paragraph);});return [...paragraphs.values()];
  }
  format(selection: TextSelection, style: InlineStyle): DraftFocus | undefined {
    const range=this.normalized(selection);if (!range) return;
    const found=range.runs.map(item=>this.find(item.run.id));
    if (found.some(item=>!item)) return;
    const collapsed=range.start.runId===range.end.runId && range.start.offset===range.end.offset;
    let first:TextPoint|undefined,last:TextPoint|undefined;
    for (const [index,item] of found.entries()) {
      const {run,paragraph,group}=item!;
      const start=index===0?range.start.offset:0,end=index===found.length-1?range.end.offset:run.text.length;
      if (!collapsed && start===end) continue;
      const combined:InlineStyle=style.reset?{
        ...(run.style?.proofingLanguage && {proofingLanguage:run.style.proofingLanguage}),
        ...(run.style?.eastAsiaProofingLanguage && {eastAsiaProofingLanguage:run.style.eastAsiaProofingLanguage}),
        ...style,
      }:{...run.style,...style};
      if (style.font) {
        if (style.latinFont === undefined) delete combined.latinFont;
        if (style.eastAsiaFont === undefined) delete combined.eastAsiaFont;
      }
      const baseline=combined.reset ? {bold:false,italic:false,underline:false,strike:false,color:undefined,font:undefined,eastAsiaFont:undefined,sizeHalfPoints:undefined,highlight:undefined,vertical:undefined,...run.baseStyle} : run;
      const selected:DraftRun={...run,...baseline,...combined,font:combined.latinFont??combined.font??baseline.font,...combined.font?{eastAsiaFont:combined.eastAsiaFont??combined.font}:{},...combined.styleId!==undefined?{characterStyleId:combined.styleId}:{},id:this.id(),text:run.text.slice(start,end),style:combined};
      const pieces:DraftRun[]=[];
      if(start) pieces.push({...run,text:run.text.slice(0,start)});
      pieces.push(selected);
      if(end<run.text.length)pieces.push({...run,id:this.id(),text:run.text.slice(end)});
      const position=paragraph.runs.findIndex(r=>r.id===run.id);
      paragraph.runs.splice(position,1,...pieces);group.changed=true;
      first??={runId:selected.id,offset:0};last={runId:selected.id,offset:selected.text.length};
    }
    if(!first || !last)return;
    const forward=selection.anchor.runId===range.start.runId && selection.anchor.offset===range.start.offset;
    const selected={anchor:forward?first:last,focus:forward?last:first};
    return {runId:selected.focus.runId,start:selected.focus.offset,end:selected.focus.offset,selection:selected};
  }
  replaceRich(selection: TextSelection, paragraphs: ClipboardParagraph[]): DraftFocus | undefined {
    if (!paragraphs.length || paragraphs.length>1000 || paragraphs.some(p=>!p.length)) return;
    return this.replace(selection,paragraphs.map(p=>p.map(r=>r.text).join('')).join('\n'),false,paragraphs);
  }
  selectedRuns(selection:TextSelection):Run[] {
    const range=this.normalized(selection);if(!range)return [];
    const collapsed=range.start.runId===range.end.runId&&range.start.offset===range.end.offset;
    return range.runs.filter((item,index)=>collapsed||(index===range.runs.length-1?range.end.offset:item.run.text.length)>(index===0?range.start.offset:0)).map(item=>item.run);
  }
  selectedRich(selection: TextSelection): ClipboardParagraph[] | undefined {
    const range=this.normalized(selection);if (!range) return;
    const result:ClipboardParagraph[]=[];let previous:string|undefined;
    for (const [index,{paragraph,run}] of range.runs.entries()) {
      if (previous!==paragraph.id) result.push([]);previous=paragraph.id;
      result.at(-1)!.push({text:run.text.slice(index===0?range.start.offset:0,index===range.runs.length-1?range.end.offset:undefined),style:{bold:run.bold,italic:run.italic,underline:Boolean(run.underline),strike:Boolean(run.strike),highlight:run.highlight??'none',vertical:run.vertical??'baseline',...run.font?{font:run.font}:{},...run.color?{color:run.color}:{},...run.sizeHalfPoints?{sizeHalfPoints:run.sizeHalfPoints}:{}}});
    }
    return result;
  }
  snapshot(lists: ListDefinition[] = []): Snapshot {
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
    return previewLists({ ...this.original, blocks }, lists);
  }
}
