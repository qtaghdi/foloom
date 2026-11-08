import { render } from 'solid-js/web';
import { expect,it } from 'vitest';
import { tabAdvance,TabText } from '../src/docx/tab-layout';
import { StructureDraft } from '../src/docx/structure-draft';
import type { Snapshot } from '../src/docx/contracts';

it('aligns left, right, center, decimal and default tabs without changing text',()=>{
 expect(tabAdvance(20,30,10,[{position:1440,align:'left',leader:'dot'}])).toEqual({width:76,leader:'dot'});
 expect(tabAdvance(20,30,10,[{position:1440,align:'right'}]).width).toBe(46);
 expect(tabAdvance(20,30,10,[{position:1440,align:'center'}]).width).toBe(61);
 expect(tabAdvance(20,30,10,[{position:1440,align:'decimal'}]).width).toBe(66);
 expect(tabAdvance(49,0,0,[{position:1440,align:'bar'}]).width).toBe(47);
 const host=document.createElement('div');const dispose=render(()=><TabText text={'a\tb\tc'}/>,host);
 expect(host.textContent).toBe('a\tb\tc');expect(host.querySelectorAll('[data-docx-tab]')).toHaveLength(2);dispose();
});

it('patches tab positions independently and keeps explicit clears in the save batch',()=>{
 const input:Snapshot={id:'s',revision:1,sequence:0,name:'tabs.docx',warnings:[],blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,layout:{tabs:[{position:720,align:'left'},{position:1440,align:'right'}]},runs:[{id:'r',text:'a\tb',editable:true,bold:false,italic:false}]}}]};
 const draft=new StructureDraft(input),point={runId:'r',offset:1},selection={anchor:point,focus:point};
 draft.formatParagraph(selection,{tabs:[{position:720,align:'clear'}]});
 draft.formatParagraph(selection,{tabs:[{position:2160,align:'decimal',leader:'dot'}]});
 const p=draft.snapshot().blocks[0];
 expect(p.type==='paragraph'&&p.paragraph.layout?.tabs).toEqual([{position:1440,align:'right'},{position:2160,align:'decimal',leader:'dot'}]);
 expect(draft.operations()[0]?.paragraphs[0]?.format?.tabs).toContainEqual({position:720,align:'clear'});
 expect(input.blocks[0].type==='paragraph'&&input.blocks[0].paragraph.layout?.tabs).toHaveLength(2);
});
