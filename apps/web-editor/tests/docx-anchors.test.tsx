import { expect,it } from 'vitest';
import { compareParagraphs, restoreAnchoredPoint } from '../src/docx/paragraphs';
import type { Snapshot } from '../src/docx/contracts';

function block(id:string,text:string,anchor?:string):Snapshot['blocks'][number]{
  return {type:'paragraph',id,paragraph:{id,anchor,structureEditable:true,runs:[{id:id+'-r',text,editable:true,bold:false,italic:false}]}};
}
const original:Snapshot={id:'document',name:'docx',revision:1,sequence:0,warnings:[],
  blocks:[block('a','same','00000001'),block('b','original','00000002'),block('plain','ambiguous')]};

it('follows native anchors through moved and inserted blocks without guessing other targets',()=>{
  const next={...original,revision:2,blocks:[
    block('new','inserted'),block('b2','updated','00000002'),block('plain2','not a reliable match'),block('a2','same','00000001'),
  ]};
  expect(compareParagraphs(original,next)).toEqual([{
    key:'1',anchor:'00000002',label:'본문 문단 2',before:'original',after:'updated',
  }]);
  expect(compareParagraphs(original,{...next,revision:3})).toEqual([]);
});
it('uses positions only for a server-declared text-only commit',()=>{
  const next={...original,revision:2,blocks:[...original.blocks.slice(0,2),block('plain2','text edit')]};
  expect(compareParagraphs(original,next)).toEqual([]);
  expect(compareParagraphs(original,next,true)).toMatchObject([{key:'2',before:'ambiguous',after:'text edit'}]);
});

it('restores a point in the same paragraph after history changes its position',()=>{
  const next={...original,revision:2,blocks:[block('b2','short','00000002'),block('a2','same','00000001')]};
  expect(restoreAnchoredPoint(original,next,{runId:'b-r',offset:7})).toEqual({runId:'b2-r',offset:5});
  expect(restoreAnchoredPoint(original,{...next,blocks:next.blocks.slice(1)},{runId:'b-r',offset:3})).toBeNull();
  expect(restoreAnchoredPoint(original,next,{runId:'plain-r',offset:0})).toBeUndefined();
});

it('maps a stable anchor to editable text even if paragraph structure is protected', () => {
  const block = original.blocks[1]!;
  if (block.type !== 'paragraph') throw new Error('expected a paragraph fixture');
  const after: Snapshot = {...original, blocks: [
    {...block, paragraph:{...block.paragraph, structureEditable:false, runs:[
      {...block.paragraph.runs[0]!, id:'b2-r'},
    ]}},
  ]};
  expect(restoreAnchoredPoint(original, after, {runId:'b-r', offset:2})).toEqual({runId:'b2-r', offset:2});
});
