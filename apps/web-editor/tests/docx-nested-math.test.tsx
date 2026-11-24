import { render } from 'solid-js/web';
import { expect,it,vi } from 'vitest';
import { DocumentMathToolbar } from '../src/docx/document-objects';
import type { Paragraph, Snapshot } from '../src/docx/contracts';

it('creates nested OMML and edits a selected leaf',()=>{
  const host=document.createElement('div');document.body.append(host);
  const target:Paragraph={id:'p',structureEditable:true,runs:[]};
  const snapshot:Snapshot={id:'s',name:'math.docx',revision:1,sequence:0,warnings:[],blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:false,runs:[{id:'r',text:'',editable:false,bold:false,italic:false,object:{kind:'math',editable:true,nestedMath:{kind:'root_fraction',numerator:'a',denominator:'b'}}}]}}]};
  const onApply=vi.fn(()=>true);const dispose=render(()=><DocumentMathToolbar target={target} snapshot={snapshot} pending={false} disabled={false} onApply={onApply}/>,host);
  const action=host.querySelector<HTMLSelectElement>('select[aria-label="수식 작업"]')!;
  const change=(select:HTMLSelectElement,value:string)=>{select.value=value;select.dispatchEvent(new Event('change',{bubbles:true}));};
  const enter=(input:HTMLInputElement,value:string)=>{input.value=value;input.dispatchEvent(new Event('input',{bubbles:true}));};
  change(action,'insert_nested');
  enter(host.querySelector<HTMLInputElement>('input[aria-label="분자"]')!,'a');enter(host.querySelector<HTMLInputElement>('input[aria-label="분모"]')!,'b');
  host.querySelector('button')?.click();
  expect(onApply).toHaveBeenCalledWith({type:'update_math',targetId:'p',math:{action:'insert_nested',nested:{kind:'root_fraction',numerator:'a',denominator:'b'}}});
  change(action,'edit_nested_leaf');const input=host.querySelector<HTMLInputElement>('input[type=text]')!;expect(input.value).toBe('a');enter(input,'c');host.querySelector('button')?.click();
  expect(onApply).toHaveBeenCalledWith({type:'update_math',targetId:'r',math:{action:'edit_nested_leaf',nestedLeaf:{leaf:'numerator',expectedText:'a',text:'c'}}});
  dispose();host.remove();
});
