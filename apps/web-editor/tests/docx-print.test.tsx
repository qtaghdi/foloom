import { expect, it, vi } from 'vitest';
import type { SectionInfo, Snapshot } from '../src/docx/contracts';
import { printDocument, printPaperFor, printPaperRules } from '../src/docx/print-layout';

const portrait = {width:11906,height:16838,top:1440,right:1440,bottom:1440,left:1440};
const landscape: SectionInfo = {
  id:'section-2',startBlock:1,endBlock:1,columns:1,columnSpace:720,
  orientation:'landscape',breakType:'nextPage',editable:true,
  page:{...portrait,width:16838,height:11906,left:720},
};
const snapshot: Snapshot = {id:'document',name:'print.docx',revision:1,sequence:0,blocks:[],warnings:[],page:portrait,sections:[landscape]};

it('uses physical sizes and distinct margins for mixed sections', () => {
  const rules = printPaperRules(snapshot);
  expect(rules).toContain(`@page ${printPaperFor(snapshot)} { size: 595.3pt 841.9pt; margin: 72pt 72pt 72pt 72pt; }`);
  expect(rules).toContain(`@page ${printPaperFor(snapshot,landscape)} { size: 841.9pt 595.3pt; margin: 72pt 72pt 72pt 36pt; }`);
});

it('waits for image decoding and stops when the document changes', async () => {
  const article = document.createElement('article');
  const image = document.createElement('img');
  const decode = vi.fn().mockResolvedValue(undefined);
  image.decode = decode;
  image.loading = 'lazy';
  article.append(image);
  const print = vi.spyOn(window,'print').mockImplementation(()=>{});
  try {
    expect(await printDocument(article,()=>false)).toBe(false);
    expect(print).not.toHaveBeenCalled();
    expect(decode).toHaveBeenCalledOnce();
    expect(image.loading).toBe('eager');
    expect(await printDocument(article,()=>true)).toBe(true);
    expect(print).toHaveBeenCalledOnce();
  } finally { print.mockRestore(); }
});
