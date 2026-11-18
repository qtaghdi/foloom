import { expect, it, vi } from 'vitest';
import { parseClipboardHTML, clipboardHTML } from '../src/docx/clipboard';
import { continuousInput } from '../src/docx/continuous-input';
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
it('round-trips inherited underline and strike while normalizing unsupported decoration', () => {
 const source='<p><u>하나 <s>둘</s><span style="text-decoration:none"> 셋</span></u><span style="text-decoration-line:line-through overline"> 넷</span></p>';
 const result=parseClipboardHTML(source,document)!;
 expect(result.stripped).toBe(true);
 expect(result.paragraphs[0].map(({text,style})=>[text,style?.underline,style?.strike]))
  .toEqual([['하나 ',true,false],['둘',true,true],[' 셋',true,false],[' 넷',false,true]]);
 const restored=parseClipboardHTML(clipboardHTML(result.paragraphs),document)!;
 expect(restored.paragraphs).toEqual(result.paragraphs);
 expect(restored.stripped).toBe(false);
});
it('keeps prefix and suffix source formatting separate from pasted character styles', () => {
 const snapshot:Snapshot={id:'s',name:'x.docx',revision:1,sequence:0,warnings:[],blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[{id:'r',text:'prefix suffix',editable:true,bold:true,italic:false}]}}]};
 const draft=new StructureDraft(snapshot);
 draft.replaceRich({anchor:{runId:'r',offset:7},focus:{runId:'r',offset:7}},[[{text:'new',style:{bold:false,italic:true,color:'#112233'}}]]);
 const runs=draft.operations()[0]?.paragraphs[0]?.runs;
 expect(runs).toEqual([{sourceId:'r',text:'prefix '},{sourceId:'r',text:'new',style:{bold:false,italic:true,color:'#112233'}},{sourceId:'r',text:'suffix'}]);
});

it('scopes text-only paste to one active keyboard shortcut', () => {
 const root=document.createElement('div');
 root.innerHTML='<span data-docx-run="r">start</span>';
 document.body.append(root);
 try {
  const node=root.firstElementChild!.firstChild!;
  document.getSelection()!.setBaseAndExtent(node,5,node,5);
  const replace=vi.fn(()=>true);
  const pasteRich=vi.fn(()=>true);
  let saving=false;
  const input=continuousInput(()=>root,{
   history:vi.fn(),snapshot:()=>({id:'s',name:'x.docx',revision:1,sequence:0,warnings:[],blocks:[]}),
   saving:()=>saving,edit:vi.fn(),replace,delete:()=>false,structure:()=>false,
   composition:vi.fn(),refresh:vi.fn(),text:()=>undefined,focus:vi.fn(),notice:vi.fn(),
   pasteMode:()=>'basic',pasteRich,richText:()=>undefined,
  });
  const shortcut=()=>input.keydown(new KeyboardEvent('keydown',{key:'V',shiftKey:true,metaKey:true}));
  const paste=()=>{
   const event=new Event('paste',{cancelable:true});
   Object.defineProperty(event,'clipboardData',{value:{getData:(kind:string)=>kind==='text/html'?'<b>formatted</b>':'formatted'}});
   input.paste(event as ClipboardEvent);
  };

  shortcut();
  paste();
  expect(replace).toHaveBeenCalledTimes(1);
  expect(pasteRich).not.toHaveBeenCalled();

  shortcut();
  input.endPasteShortcut();
  paste();
  expect(pasteRich).toHaveBeenCalledTimes(1);

  shortcut();
  saving=true;
  paste();
  saving=false;
  paste();
  expect(pasteRich).toHaveBeenCalledTimes(2);
  expect(replace).toHaveBeenCalledTimes(1);
 } finally {
  root.remove();
 }
});
