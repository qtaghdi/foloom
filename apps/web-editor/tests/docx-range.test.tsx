import { expect, it } from 'vitest';
import { StructureDraft } from '../src/docx/structure-draft';
import { graphemeOffsets, type TextSelection } from '../src/docx/selection';
import type { Snapshot, Paragraph } from '../src/docx/contracts';
const paragraph = (id: string, texts: string[]): Paragraph => ({ id, structureEditable: true, runs: texts.map((text,index) => ({ id: `${id}:${index}`, text, editable: true, bold: index===0, italic: index===1 })) });
const snapshot: Snapshot = { id:'s', name:'range.docx', revision:1, sequence:0, warnings:[], blocks:[
  { id:'p1', type:'paragraph', paragraph:paragraph('p1',['ab😀','CDe\u0301']) },
  { id:'p2', type:'paragraph', paragraph:paragraph('p2',['한글','next']) },
  { id:'table', type:'table', rows:[{cells:[{id:'c1', paragraphs:[paragraph('c1p',['cell 1'])]},{id:'c2',paragraphs:[paragraph('c2p',['cell 2'])]}]}] },
] };
const selection = (a:string,start:number,b=a,end=start):TextSelection => ({anchor:{runId:a,offset:start},focus:{runId:b,offset:end}});
it('replaces reverse selections across formatting and paragraph boundaries while preserving suffix styles', () => {
 const draft=new StructureDraft(snapshot);
 const focus=draft.replace(selection('p2:1',2,'p1:0',1),'X\nY');
 expect(focus).toBeDefined();
 const [op]=draft.operations(); expect(op?.paragraphs).toEqual([
  { sourceId:'p1', runs:[{sourceId:'p1:0',text:'aX'}] },
  { sourceId:'p1', runs:[{sourceId:'p1:0',text:'Y'},{sourceId:'p2:1',text:'xt'}] },
 ]);
 expect(snapshot.blocks[0]?.type==='paragraph' && snapshot.blocks[0].paragraph.runs[0]?.text).toBe('ab😀');
});
it('uses Unicode grapheme boundaries for combining marks, emoji modifiers and joined emoji', () => {
 const text='a👨‍👩‍👧‍👦e\u0301👍🏽'; const offsets=graphemeOffsets(text);
 expect(offsets).toEqual([0,1,12,14,18]);
 const draft=new StructureDraft(snapshot);
 expect(draft.replace(selection('p1:0',3),'bad')).toBeUndefined();
 const range=draft.deletion(selection('p1:1',4),'backward')!;
 expect(range.focus.offset).toBe(2); expect(draft.replace(range,'')).toBeDefined();
 expect(draft.operations()[0]?.paragraphs[0]?.runs.map(r=>r.text).join('')).toBe('ab😀CD');
});
it('clears a selection across table cells without deleting or merging cells', () => {
 const draft=new StructureDraft(snapshot);
 expect(draft.replace(selection('c1p:0',2,'c2p:0',4),'X')).toBeDefined();
 const ops=draft.operations();expect(ops).toHaveLength(2);
 expect(ops[0]?.paragraphs[0]?.runs.map(r=>r.text).join('')).toBe('ceX');
 expect(ops[1]?.paragraphs[0]?.runs.map(r=>r.text).join('')).toBe(' 2');
 const table=draft.snapshot().blocks.at(-1)!;
 expect(table.type==='table' && table.rows[0]?.cells.length).toBe(2);
});
it('keeps inline line breaks separate from paragraph insertion', () => {
 const draft=new StructureDraft(snapshot);
 draft.replace(selection('p1:0',1),'\n',true);
 expect(draft.operations()[0]?.paragraphs).toHaveLength(2);
 expect(draft.operations()[0]?.paragraphs[0]?.runs.map(r=>r.text).join('')).toBe('a\nb😀CDe\u0301');
});

it('splits only selected formatting runs and keeps reversed selection and source properties',()=>{
 const draft=new StructureDraft(snapshot);
 const focus=draft.format(selection('p1:1',2,'p1:0',1),{underline:true,color:'#112233'});
 expect(focus?.selection?.anchor.offset).toBe(2);
 const op=draft.operations()[0]!;
 expect(op.paragraphs[0]!.runs.map(r=>[r.sourceId,r.text,r.style])).toEqual([
  ['p1:0','a',undefined],['p1:0','b😀',{underline:true,color:'#112233'}],['p1:1','CD',{underline:true,color:'#112233'}],['p1:1','e\u0301',undefined],
 ]);
 expect(op.paragraphs[1]!.runs.every(r=>!r.style)).toBe(true);
});
it('formats a collapsed typing position and restores inherited style on direct reset',()=>{
 const initial:Snapshot={...snapshot,blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[{id:'r',text:'ab',editable:true,bold:true,italic:true,font:'Direct',baseStyle:{bold:true,italic:false,font:'Inherited'}}]}}]};
 const draft=new StructureDraft(initial);const focus=draft.format(selection('r',1),{reset:true});
 expect(focus).toBeDefined();const p=draft.snapshot().blocks[0];if(p?.type!=='paragraph')throw new Error('paragraph missing');
 expect(p.paragraph.runs.map(r=>[r.text,r.bold,r.italic,r.font])).toEqual([['a',true,true,'Direct'],['',true,false,'Inherited'],['b',true,true,'Direct']]);
 draft.edit(focus!.runId,'new');expect(draft.operations()[0]!.paragraphs[0]!.runs[1]).toMatchObject({text:'new',style:{reset:true}});
});

it('formats touched paragraphs without including a selection ending at the next paragraph start',()=>{
 const draft=new StructureDraft(snapshot);const range=selection('p1:0',1,'p2:0',0);
 expect(draft.formatParagraph(range,{align:'center',firstLine:240})).toBeDefined();
 expect(draft.selectedParagraphs(range)).toHaveLength(1);
 expect(draft.operations()[0]!.paragraphs.map(p=>p.format)).toEqual([{align:'center',firstLine:240},undefined]);
 draft.formatParagraph(selection('p1:0',0),{hanging:120});
 expect(draft.operations()[0]!.paragraphs[0]!.format).toEqual({align:'center',hanging:120});
});

it('preserves language metadata when direct formatting is cleared before save',()=>{
 const initial:Snapshot={...snapshot,blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[
  {id:'r',text:'Latin 日本語',editable:true,bold:true,italic:false,proofingLanguage:'en-US',eastAsiaProofingLanguage:'ko-KR'},
 ]}}]};
 const draft=new StructureDraft(initial);
 const change=draft.format(selection('r',6,'r',9),{eastAsiaProofingLanguage:'ja-JP'});
 expect(draft.selectedRuns(change!.selection!)[0]).toMatchObject({proofingLanguage:'en-US',eastAsiaProofingLanguage:'ja-JP'});
 const reset=draft.format(change!.selection!,{reset:true});
 expect(draft.selectedRuns(reset!.selection!)[0]).toMatchObject({proofingLanguage:'en-US',eastAsiaProofingLanguage:'ja-JP',bold:false});
 expect(draft.operations()[0]!.paragraphs[0]!.runs.map(r=>[r.text,r.style])).toEqual([
  ['Latin ',undefined],['日本語',{eastAsiaProofingLanguage:'ja-JP',reset:true}],
 ]);
});

it('keeps East Asian font overrides separate and lets a later all-script choice replace them',()=>{
 const initial:Snapshot={...snapshot,blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[
  {id:'r',text:'Latin 한글',editable:true,bold:false,italic:false,font:'Latin',eastAsiaFont:'원본',baseStyle:{eastAsiaFont:'상속'}},
 ]}}]};
 const draft=new StructureDraft(initial);
 const east=draft.format(selection('r',6,'r',8),{eastAsiaFont:'새 글꼴'});
 expect(east?.selection).toBeDefined();
 expect(draft.selectedRuns(east!.selection!)[0]).toMatchObject({font:'Latin',eastAsiaFont:'새 글꼴'});
 expect(draft.operations()[0]!.paragraphs[0]!.runs.map(r=>[r.text,r.style])).toEqual([
  ['Latin ',undefined],['한글',{eastAsiaFont:'새 글꼴'}],
 ]);
 const all=draft.format(east!.selection!,{font:'All scripts'});
 expect(draft.selectedRuns(all!.selection!)[0]).toMatchObject({font:'All scripts',eastAsiaFont:'All scripts'});
 expect(draft.operations()[0]!.paragraphs[0]!.runs[1]!.style).toEqual({font:'All scripts'});
 const reset=draft.format(all!.selection!,{reset:true});
 expect(draft.selectedRuns(reset!.selection!)[0]!.eastAsiaFont).toBe('상속');
});
