import { render } from 'solid-js/web';
import { expect,it,vi } from 'vitest';
import { layoutPreviewPages, previewPages } from '../src/docx/document-content';
import { DocumentSectionToolbar } from '../src/docx/document-section-toolbar';
import { previewSection,validateSection } from '../src/docx/sections';
import type { SectionInfo, Snapshot } from '../src/docx/contracts';

const section:SectionInfo={id:'s',startBlock:0,endBlock:0,page:{width:11906,height:16838,top:1440,right:1440,bottom:1440,left:1440},orientation:'portrait',columns:1,columnSpace:720,breakType:'nextPage',editable:true};
it('keeps local form changes pending until explicitly applied to the chosen section',async()=>{
 const host=document.createElement('div'),apply=vi.fn(()=>true);document.body.append(host);
 const dispose=render(()=><DocumentSectionToolbar sections={[section]} disabled={false} onApply={apply}/>,host);
 await Promise.resolve();
 const direction=host.querySelector<HTMLSelectElement>('[aria-label="용지 방향"]')!;direction.value='landscape';direction.dispatchEvent(new Event('change',{bubbles:true}));
 expect(apply).not.toHaveBeenCalled();
 [...host.querySelectorAll('button')].find(button=>button.textContent==='선택 구역 적용')!.click();
 expect(apply).toHaveBeenCalledWith('s',expect.objectContaining({width:16838,height:11906,orientation:'landscape'}));dispose();host.remove();
});
it('rejects margins or column gaps that leave no content area',()=>{
 expect(validateSection(previewSection(section,{left:11000}))).not.toBe('');
 expect(validateSection(previewSection(section,{columns:12,columnSpace:14400}))).not.toBe('');
 expect(section.page.left).toBe(1440);
});

it('stages continuation without replacing an imported numbering format',async()=>{
 const host=document.createElement('div'),apply=vi.fn(()=>true);document.body.append(host);
 const existing={...section,pageNumberStart:12,pageNumberFormat:'chicago'};
 const dispose=render(()=><DocumentSectionToolbar sections={[existing]} disabled={false} onApply={apply}/>,host);
 await Promise.resolve();
 const start=host.querySelector<HTMLSelectElement>('[aria-label="쪽 번호 시작 방식"]')!;
 start.value='continue';start.dispatchEvent(new Event('change',{bubbles:true}));
 expect(apply).not.toHaveBeenCalled();
 [...host.querySelectorAll('button')].find(button=>button.textContent==='선택 구역 적용')!.click();
 expect(apply).toHaveBeenCalledWith('s',{pageNumberStart:-1});
 expect(previewSection(existing,{pageNumberStart:-1})).toMatchObject({pageNumberStart:undefined,pageNumberFormat:'chicago'});
 dispose();host.remove();
});


it('inserts only the needed blank page and carries parity into the next ordinary section',()=>{
 const first={...section,id:'a'};
 const second={...section,id:'b',firstPage:true,breakType:'oddPage'};
 const third={...section,id:'c'};
 const blocks:Snapshot['blocks']=['a','b','b','c'].map((sectionId,index)=>({
  id:String(index),type:'paragraph',paragraph:{id:String(index),sectionId,runs:[],structureEditable:true,pageBreakBefore:index===2},
 }));
 const pages=previewPages(blocks);
 expect(pages.map(page=>page.blocks.map(item=>item.index))).toEqual([[0],[1],[2],[3]]);
 const describePages=(sections:SectionInfo[],evenAndOddHeaders=true)=>layoutPreviewPages(pages,sections,evenAndOddHeaders).map(page=>[
  page.section?.id,page.number,page.blankBefore,page.variant,
 ]);
 expect(describePages([first,second,third])).toEqual([
  ['a',1,false,'default'],['b',3,true,'first'],['b',4,false,'even'],['c',5,false,'default'],
 ]);
 expect(describePages([first,{...second,breakType:'evenPage'},third])).toEqual([
  ['a',1,false,'default'],['b',2,false,'first'],['b',3,false,'default'],['c',4,false,'even'],
 ]);
 expect(describePages([first,second,third],false)).toEqual([
  ['a',1,false,'default'],['b',3,true,'first'],['b',4,false,'default'],['c',5,false,'default'],
 ]);
 expect(describePages([{...first,pageNumberStart:5},second,third]).at(-1)?.[1]).toBe(4);
 expect(describePages([first,{...second,pageNumberStart:8},third]).at(-1)?.[1]).toBe(4);
});

it('pads a requested even start after two explicit pages and counts the blank only once',()=>{
 const first={...section,id:'a'};
 const second={...section,id:'b',breakType:'evenPage'};
 const blocks:Snapshot['blocks']=['a','a','b'].map((sectionId,index)=>({
  id:String(index),type:'paragraph',paragraph:{id:String(index),sectionId,runs:[],structureEditable:true,pageBreakBefore:index===1},
 }));
 const pages=layoutPreviewPages(previewPages(blocks),[first,second],true);
 expect(pages.map(page=>[page.number,page.blankBefore,page.variant])).toEqual([
  [1,false,'default'],[2,false,'even'],[4,true,'even'],
 ]);
});
