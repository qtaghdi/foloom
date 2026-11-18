import { afterEach, expect, it, vi } from 'vitest';
import { DocumentController } from '../src/docx/controller';
import { StructureDraft } from '../src/docx/structure-draft';
import { snapshotSchema, type Snapshot } from '../src/docx/contracts';
import { previewLists } from '../src/docx/lists';

afterEach(()=>vi.unstubAllGlobals());
const base:Snapshot={id:'listSession',name:'list.docx',revision:1,sequence:0,warnings:[],blocks:[
 {id:'p',type:'paragraph',paragraph:{id:'p',structureEditable:true,runs:[{id:'r',text:'item',editable:true,bold:false,italic:false}]}},
]};
const selection={anchor:{runId:'r',offset:4},focus:{runId:'r',offset:4}};

it('previews a list and keeps its reference through paragraph split and undo state',()=>{
 const draft=new StructureDraft(base),before=draft.capture();
 expect(draft.formatParagraph(selection,{list:{key:'newList',level:0}})).toBeDefined();
 const first=draft.replace(selection,'\n');
 expect(first).toBeDefined();
 const snapshot=draft.snapshot([{key:'newList',kind:'decimal',start:4}]);
 expect(snapshot.blocks.flatMap(b=>b.type==='paragraph'?[b.paragraph.listLabel]:[])).toEqual(['4.','5.']);
 expect(draft.operations()[0]?.paragraphs.every(p=>p.format?.list?.key==='newList')).toBe(true);
 draft.restore(before);
 expect(draft.operations()).toHaveLength(0);
 expect(snapshotSchema.safeParse(snapshot).success).toBe(true);
});

it('stores list definitions and paragraph applications in one batch, and ends an empty list with Enter',async()=>{
 let rendered:Snapshot=base;
 const fetcher=vi.fn(async(_path:string,_init?:RequestInit)=>({ok:true,status:200,json:async()=>base}));
 vi.stubGlobal('fetch',fetcher);
 vi.stubGlobal('EventSource',class{addEventListener(){} close(){} onerror=null;});
 const controller=new DocumentController({render:s=>{rendered=s;},status(){},error(){},canSave(){}});
 await controller.load();
 expect(controller.applyList(selection,{kind:'decimal',start:3})).toBe(true);
 expect(rendered.blocks[0]?.type==='paragraph'&&rendered.blocks[0].paragraph.listLabel).toBe('3.');
 expect(await controller.restore('undo')).toBe(true);
 expect(controller.hasDrafts()).toBe(false);
 expect(await controller.restore('redo')).toBe(true);
 expect(controller.replaceSelection(selection,'\n')).toBe(true);
 const p=rendered.blocks[1];
 if(p?.type!=='paragraph')throw new Error('missing new list item');
 expect(p.paragraph.listLabel).toBe('4.');
 const run=p.paragraph.runs[0]!;
 const end={anchor:{runId:run.id,offset:0},focus:{runId:run.id,offset:0}};
 expect(controller.replaceSelection(end,'\n')).toBe(true);
 expect(rendered.blocks).toHaveLength(2);
 expect(rendered.blocks[1]?.type==='paragraph'&&rendered.blocks[1].paragraph.listLabel).toBeUndefined();
 await controller.save();
 const write=fetcher.mock.calls.find(call=>(call[1] as RequestInit|undefined)?.method==='POST');
 const request=JSON.parse(String((write?.[1] as RequestInit)?.body));
 expect(request.operations[0]).toMatchObject({type:'define_list',listDefinition:{kind:'decimal',start:3}});
 expect(request.operations[1].paragraphs[0].format.list.key).toBe(request.operations[0].listDefinition.key);
 expect(request.operations[1].paragraphs[1].format.list).toEqual({id:0,level:0});
 controller.dispose();
});

it('continues existing numbering through table cells and leaves unsupported formats explicit',()=>{
 const numbered=structuredClone(base);
 numbered.lists=[{id:7,levels:[{level:0,kind:'decimal',text:'%1.',start:2}]}];
 if(numbered.blocks[0]?.type==='paragraph')numbered.blocks[0].paragraph.list={id:7,level:0};
 numbered.blocks.push({id:'table',type:'table',rows:[{cells:[{id:'cell',paragraphs:[{id:'cp',runs:[],list:{id:7,level:0}}]}]}]});
 const result=previewLists(numbered);
 expect(result.blocks[0]?.type==='paragraph'&&result.blocks[0].paragraph.listLabel).toBe('2.');
 expect(result.blocks[1]?.type==='table'&&result.blocks[1].rows[0]?.cells[0]?.paragraphs[0]?.listLabel).toBe('3.');
});

it('keeps Roman and legal list labels during draft preview', () => {
  const numbered=structuredClone(base);
  numbered.lists=[{id:7,levels:[
    {level:0,kind:'upperRoman',text:'%1.',start:4},
    {level:1,kind:'lowerRoman',text:'%1.%2.',start:9},
    {level:2,kind:'lowerRoman',text:'%1.%2.%3.',start:2,legal:true},
  ]}];
  const first=numbered.blocks[0];
  if (first?.type!=='paragraph') throw new Error('paragraph fixture missing');
  first.paragraph.list={id:7,level:0};
  numbered.blocks.push({id:'next',type:'paragraph',paragraph:{id:'child',runs:[],list:{id:7,level:1}}});
  numbered.blocks.push({id:'legal',type:'paragraph',paragraph:{id:'grandchild',runs:[],list:{id:7,level:2}}});
  const preview=previewLists(numbered);
  expect(preview.blocks.flatMap(block=>block.type==='paragraph'?[block.paragraph.listLabel]:[])).toEqual(['IV.','IV.ix.','4.9.2.']);
});

it('keeps continuous levels and resets configured levels in draft list labels', () => {
  const numbered=structuredClone(base);
  numbered.lists=[{id:7,levels:[
    {level:0,kind:'decimal',text:'%1.',start:1},
    {level:1,kind:'decimal',text:'%2.',start:2,restart:0},
    {level:2,kind:'upperRoman',text:'%3.',start:4,restart:1},
  ]}];
  numbered.blocks=[0,1,2,1,2,0,1,2].map((level,index)=>({
    id:'block'+index,type:'paragraph',paragraph:{id:'paragraph'+index,runs:[],list:{id:7,level}},
  }));
  const preview=previewLists(numbered);
  expect(preview.blocks.map(block=>block.type==='paragraph'&&block.paragraph.listLabel))
    .toEqual(['1.','2.','IV.','3.','V.','2.','4.','IV.']);
});


it('repeats the same letter after each group of 26 in imported and new lists', () => {
  const numbered=structuredClone(base);
  const p=numbered.blocks.find(block=>block.type==='paragraph');
  if(p?.type!=='paragraph')throw new Error('paragraph fixture missing');
  numbered.blocks=['first','second','third'].map(id=>({type:'paragraph',id,paragraph:{...p.paragraph,id,list:{id:7}}}));
  numbered.lists=[{id:7,levels:[{level:0,kind:'upperLetter',text:'%1.',start:26}]}];
  const labels=(snapshot:typeof numbered)=>snapshot.blocks.flatMap(block=>block.type==='paragraph'?[block.paragraph.listLabel]:[]);
  expect(labels(previewLists(numbered))).toEqual(['Z.','AA.','BB.']);
  for(const block of numbered.blocks)if(block.type==='paragraph')block.paragraph.list={key:'letters'};
  expect(labels(previewLists(numbered,[{key:'letters',kind:'lowerLetter',start:26}]))).toEqual(['z.','aa.','bb.']);
});
