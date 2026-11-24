import { render } from 'solid-js/web';
import { expect,it,vi } from 'vitest';
import { DocumentObjectTextToolbar } from '../src/docx/document-object-text-toolbar';
import type { Paragraph, Snapshot } from '../src/docx/contracts';

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

it('creates a dropdown with an explicit initial selection',()=>{
  const host=document.createElement('div');document.body.append(host);
  const target:Paragraph={id:'p',structureEditable:true,runs:[]};
  const onApply=vi.fn(()=>true);
  const dispose=render(()=><DocumentObjectTextToolbar target={target} kind="control" pending={false} disabled={false} onApply={onApply}/>,host);
  Array.from(host.querySelectorAll('button')).find(button=>button.textContent==='새 드롭다운')?.click();
  const input=host.querySelector<HTMLTextAreaElement>('textarea[name=newControlChoices]')!;
  input.value='대기\n완료';input.dispatchEvent(new Event('input',{bubbles:true}));
  const select=host.querySelector<HTMLSelectElement>('select[name=initialControlChoice]')!;
  select.value='완료';select.dispatchEvent(new Event('change',{bubbles:true}));
  input.form?.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
  expect(onApply).toHaveBeenCalledWith({type:'insert_control_choice',targetId:'p',expectedText:'',text:'완료',choiceOptions:[{label:'대기',value:'대기'},{label:'완료',value:'완료'}]});
  dispose();host.remove();
});
