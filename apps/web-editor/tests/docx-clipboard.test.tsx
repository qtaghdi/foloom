import { expect, it, vi } from 'vitest';
import { parseClipboardHTML, clipboardHTML } from '../src/docx/clipboard';
import { StructureDraft } from '../src/docx/structure-draft';
import type { Snapshot } from '../src/docx/contracts';

it('extracts bounded text and basic style from inert markup without adding active resources', () => {
 const fetcher=vi.spyOn(globalThis,'fetch');
 const result=parseClipboardHTML('<p>plain<strong>B</strong><em style="color:rgb(17,34,51);font-size:14pt">I</em><a href="https://example.invalid">link</a><img src="https://example.invalid/image" onerror="alert(1)" alt="image"></p><p><br>second</p><script>alert(1)</script><iframe src="https://example.invalid"></iframe>',document)!;
 expect(result.text).toBe('plainBIlinkimage\n\nsecond');
 expect(result.paragraphs[0]?.[1]?.style?.bold).toBe(true);
 expect(result.paragraphs[0]?.[2]?.style).toMatchObject({italic:true,color:'#112233',sizeHalfPoints:28});
 expect(result.stripped).toBe(true);expect(document.querySelector('img,script,iframe')).toBeNull();expect(fetcher).not.toHaveBeenCalled();
});
it('preserves empty clipboard paragraphs and refuses excessive nesting and markup size', () => {
 expect(parseClipboardHTML('<p></p><p></p><p>last</p>',document)?.text).toBe('\n\nlast');
 expect(parseClipboardHTML('<div>'.repeat(40)+'deep'+'</div>'.repeat(40),document)).toBeUndefined();
 expect(parseClipboardHTML('x'.repeat(1024*1024+1),document)).toBeUndefined();
});
it('encodes only known clipboard styles and escaped text', () => {
 const html=clipboardHTML([[{text:'<img>&\ntext',style:{bold:true,font:'name; color:red'}}]]);
 expect(html).toContain('&lt;img&gt;&amp;<br>text');expect(html).not.toContain('<img>');
 expect(html).toContain('font-family:&quot;name; color:red&quot;');
});
it('keeps prefix and suffix source formatting separate from pasted character styles', () => {
 const snapshot:Snapshot={id:'s',name:'x.docx',revision:1,sequence:0,warnings:[],blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[{id:'r',text:'prefix suffix',editable:true,bold:true,italic:false}]}}]};
 const draft=new StructureDraft(snapshot);
 draft.replaceRich({anchor:{runId:'r',offset:7},focus:{runId:'r',offset:7}},[[{text:'new',style:{bold:false,italic:true,color:'#112233'}}]]);
 const runs=draft.operations()[0]?.paragraphs[0]?.runs;
 expect(runs).toEqual([{sourceId:'r',text:'prefix '},{sourceId:'r',text:'new',style:{bold:false,italic:true,color:'#112233'}},{sourceId:'r',text:'suffix'}]);
});
