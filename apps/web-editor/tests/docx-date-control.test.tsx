import { render } from 'solid-js/web';
import { expect,it,vi } from 'vitest';
import { DocumentObjectTextToolbar } from '../src/docx/document-object-text-toolbar';
import type { Paragraph } from '../src/docx/contracts';

it('submits a native date control for the selected paragraph',()=>{
  const host=document.createElement('div');
  document.body.append(host);
  const target:Paragraph={id:'p',structureEditable:true,runs:[]};
  const onApply=vi.fn(()=>true);
  const dispose=render(()=><DocumentObjectTextToolbar kind="control" target={target} pending={false} disabled={false} onApply={onApply}/>,host);
  const input=host.querySelector<HTMLInputElement>('input[name=newControlDate]')!;
  input.value='2032-02-29';
  input.form?.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
  expect(onApply).toHaveBeenCalledWith({type:'insert_control_date',targetId:'p',expectedText:'',text:'2032-02-29'});
  expect(input.value).toBe('');
  dispose();host.remove();
});
