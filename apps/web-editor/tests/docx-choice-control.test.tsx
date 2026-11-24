import { render } from 'solid-js/web';
import { expect,it,vi } from 'vitest';
import { DocumentObjectTextToolbar } from '../src/docx/document-object-text-toolbar';
import type { Snapshot } from '../src/docx/contracts';

it('selects a listed option instead of allowing arbitrary text',()=>{
  const host=document.createElement('div');document.body.append(host);
  const snapshot:Snapshot={id:'s',name:'choice.docx',revision:1,sequence:0,warnings:[],blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:false,runs:[{id:'r',text:'대기',editable:false,bold:false,italic:false,object:{kind:'control',editable:true,text:'대기',choiceControl:{value:'대기',options:['대기','완료']}}}]}}]};
  const onApply=vi.fn(()=>true);
  const dispose=render(()=><DocumentObjectTextToolbar snapshot={snapshot} kind="control" pending={false} disabled={false} onApply={onApply}/>,host);
  const picker=host.querySelector<HTMLSelectElement>('select[aria-label="편집할 콘텐츠 컨트롤"]')!;
  picker.value='r';picker.dispatchEvent(new Event('change',{bubbles:true}));
  const select=host.querySelector<HTMLSelectElement>('select[name=controlChoice]')!;
  expect(select.value).toBe('대기');
  expect(host.querySelector('input[name=objectText]')).toBeNull();
  select.value='완료';select.form?.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
  expect(onApply).toHaveBeenCalledWith({type:'update_control_choice',targetId:'r',expectedText:'대기',text:'완료'});
  dispose();host.remove();
});
