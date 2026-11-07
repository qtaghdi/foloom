import { render } from 'solid-js/web';
import { expect,it,vi } from 'vitest';
import { DocumentSectionToolbar } from '../src/docx/document-section-toolbar';
import { previewSection,validateSection } from '../src/docx/sections';
import type { SectionInfo } from '../src/docx/contracts';

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
