import { render } from 'solid-js/web';
import { expect,it,vi } from 'vitest';
import { DocumentHeaderFooter,resolveHeaderText } from '../src/docx/document-header-footer';
import { previewPageLayout, previewSections } from '../src/docx/sections';
import type { SectionInfo } from '../src/docx/contracts';

const first:SectionInfo={id:'a',startBlock:0,endBlock:0,page:{width:11906,height:16838,top:1440,right:1440,bottom:1440,left:1440},orientation:'portrait',columns:1,columnSpace:720,breakType:'nextPage',editable:true,headerFooters:[{kind:'header',variant:'default',sourceSectionId:'a',text:'original',linked:false,editable:true}]};
const second:SectionInfo={...first,id:'b',startBlock:1,endBlock:1,headerFooters:[{kind:'header',variant:'default',sourceSectionId:'a',text:'',linked:true,editable:true}]};
it('resolves inherited text without copying it into every snapshot section',()=>{
 expect(resolveHeaderText([first,second],second.headerFooters![0])).toBe('original');
 const preview=previewSections([first,second],new Map([['a',{headerFooters:[{kind:'header',variant:'default',text:'updated'}]}]]));
 expect(resolveHeaderText(preview,preview[1]!.headerFooters![0])).toBe('updated');
 expect(first.headerFooters![0]!.text).toBe('original');
});
it('applies independent text and the first-page flag only after an explicit action',async()=>{
 const host=document.createElement('div');document.body.append(host);const apply=vi.fn(()=>true);
 const dispose=render(()=><DocumentHeaderFooter sections={[first,second]} disabled={false} onApply={apply}/>,host);await Promise.resolve();
 const text=host.querySelector('textarea')!;text.value='new header';text.dispatchEvent(new Event('input',{bubbles:true}));
 const checkbox=host.querySelector<HTMLInputElement>('input[type=checkbox]')!;checkbox.checked=true;checkbox.dispatchEvent(new Event('change',{bubbles:true}));
 expect(apply).not.toHaveBeenCalled();[...host.querySelectorAll('button')].find(button=>button.textContent==='내용 적용')!.click();
 expect(apply).toHaveBeenCalledWith('a',expect.objectContaining({firstPage:true,headerFooters:[{kind:'header',variant:'default',text:'new header'}]}));dispose();host.remove();
});
it('changes first-page mode without flattening a protected header',async()=>{
 const host=document.createElement('div');document.body.append(host);const apply=vi.fn(()=>true);
 const protectedSection={...first,headerFooters:first.headerFooters!.map(slot=>({...slot,editable:false}))};
 const dispose=render(()=><DocumentHeaderFooter sections={[protectedSection]} disabled={false} onApply={apply}/>,host);await Promise.resolve();
 expect(host.querySelector('textarea')!.disabled).toBe(true);
 const checkbox=host.querySelector<HTMLInputElement>('input[type=checkbox]')!;checkbox.checked=true;checkbox.dispatchEvent(new Event('change',{bubbles:true}));
 [...host.querySelectorAll('button')].find(button=>button.textContent==='첫 페이지 구분 적용')!.click();
 expect(apply).toHaveBeenCalledWith('a',{firstPage:true});dispose();host.remove();
});

const headerVariants = (...args:Parameters<typeof previewPageLayout>)=>previewPageLayout(...args).map(page=>page.variant);

it('follows continued and restarted page numbers while giving a section first page precedence',()=>{
 const third={...second,id:'c',pageNumberStart:8};
 expect(headerVariants([first,second,second,third,third],true)).toEqual(['default','even','default','even','default']);
 expect(headerVariants([first,{...third,firstPage:true},third],true)).toEqual(['default','first','default']);
 expect(headerVariants([first,second,third,third],false)).toEqual(['default','default','default','default']);
 expect(headerVariants([{...first,pageNumberStart:0},first],true)).toEqual(['even','default']);
});

it('resumes known parity at a restart after a boundary the preview does not paginate',()=>{
 const continuous={...second,breakType:'continuous'};
 const third={...second,id:'c'};
 const restarted={...second,id:'d',pageNumberStart:6};
 expect(headerVariants([first,continuous,continuous,third,restarted,restarted],true))
  .toEqual(['default','default','even','default','even','default']);
});
