import { render } from 'solid-js/web';
import { expect,it,vi } from 'vitest';
import { DocumentShapeToolbar } from '../src/docx/document-objects';
import type { Snapshot } from '../src/docx/contracts';

it('updates only the dimensions of a supported line',()=>{
  const host=document.createElement('div');document.body.append(host);
  const snapshot:Snapshot={id:'s',name:'line.docx',revision:1,sequence:0,warnings:[],blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:false,runs:[{id:'r',text:'',editable:false,bold:false,italic:false,object:{kind:'shape',editable:true,deleteEditable:true,line:{widthEmu:1270000,heightEmu:635000,stroke:'0F766E'}}}]}}]};
  const onApply=vi.fn(()=>true);
  const dispose=render(()=><DocumentShapeToolbar snapshot={snapshot} disabled={false} pending={false} onApply={onApply}/>,host);
  const input=host.querySelector<HTMLInputElement>('input[name=resizeLineWidth]')!;
  expect(input.valueAsNumber).toBe(100);
  input.value='120';
  input.form?.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
  expect(onApply).toHaveBeenCalledWith({type:'resize_line',targetId:'r',lineSize:{widthEmu:1524000,heightEmu:635000}});
  Array.from(host.querySelectorAll('button')).find(button=>button.textContent==='선 삭제')?.click();
  expect(onApply).toHaveBeenCalledWith({type:'delete_line',targetId:'r'});
  dispose();host.remove();
});
