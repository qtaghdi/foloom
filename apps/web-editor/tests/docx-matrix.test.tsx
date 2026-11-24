import { render } from 'solid-js/web';
import { expect,it,vi } from 'vitest';
import { DocumentMathToolbar } from '../src/docx/document-objects';
import type { Paragraph, Snapshot } from '../src/docx/contracts';

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

it('inserts a row at the selected position with explicit cells',()=>{
  const host=document.createElement('div');document.body.append(host);
  const snapshot:Snapshot={id:'s',name:'matrix.docx',revision:1,sequence:0,warnings:[],blocks:[{id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:false,runs:[{id:'r',text:'',editable:false,bold:false,italic:false,object:{kind:'math',editable:true,matrix:[['a','b'],['c','d']]}}]}}]};
  const onApply=vi.fn(()=>true);const dispose=render(()=><DocumentMathToolbar snapshot={snapshot} pending={false} disabled={false} onApply={onApply}/>,host);
  const select=host.querySelector<HTMLSelectElement>('select[aria-label="수식 작업"]')!;
  select.value='edit_matrix_axis';select.dispatchEvent(new Event('change',{bubbles:true}));
  const cells=Array.from(host.querySelectorAll<HTMLInputElement>('input[type=text]'));
  expect(cells.length).toBe(2);cells.forEach((input,i)=>{input.value=String(i+1);input.dispatchEvent(new Event('input',{bubbles:true}));});
  host.querySelector('button')?.click();
  expect(onApply).toHaveBeenCalledWith({type:'update_math',targetId:'r',math:{action:'edit_matrix_axis',matrixAxis:{axis:'row',mode:'insert',index:1,values:['1','2']}}});
  dispose();host.remove();
});
