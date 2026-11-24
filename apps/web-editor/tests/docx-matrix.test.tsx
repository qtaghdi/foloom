import { render } from 'solid-js/web';
import { expect,it,vi } from 'vitest';
import { DocumentMathToolbar } from '../src/docx/document-objects';
import type { Paragraph } from '../src/docx/contracts';

it('inserts a rectangular matrix without losing input when dimensions grow',()=>{
  const host=document.createElement('div');
  document.body.append(host);
  const target:Paragraph={id:'p',structureEditable:true,runs:[]};
  const onApply=vi.fn(()=>true);
  const dispose=render(()=><DocumentMathToolbar target={target} pending={false} disabled={false} onApply={onApply}/>,host);
  const change=(select:HTMLSelectElement,value:string)=>{
    select.value=value;
    select.dispatchEvent(new Event('change',{bubbles:true}));
  };
  const enter=(input:HTMLInputElement,value:string)=>{
    input.value=value;
    input.dispatchEvent(new Event('input',{bubbles:true}));
  };
  change(host.querySelector('select')!,'insert_matrix');
  const inputs=()=>Array.from(host.querySelectorAll<HTMLInputElement>('[role=group] input'));
  const first=inputs()[0]!;
  enter(first,'a');
  expect(inputs()[0]).toBe(first);
  change(host.querySelectorAll('select')[1]!,'3');
  expect(inputs()).toHaveLength(6);
  expect(inputs()[0]?.value).toBe('a');
  inputs().slice(1).forEach((input,i)=>enter(input,String(i+1)));
  host.querySelector('button')?.click();
  expect(onApply).toHaveBeenCalledWith({type:'update_math',targetId:'p',math:{action:'insert_matrix',matrix:[['a','1'],['2','3'],['4','5']]}});
  dispose();
  host.remove();
});
