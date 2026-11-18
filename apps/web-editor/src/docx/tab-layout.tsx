import { For } from 'solid-js';
import type { TabStop } from './contracts';

// Keep the original tab character in the text tree so selection and clipboard
// offsets stay identical to the authoritative run. Leaders are decoration only.
export function TabText(props:{text:string}) {
 return <For each={props.text.split('\t')}>{(part,index)=><>{index()>0?<span data-docx-tab class="inline-block whitespace-pre" style={{'text-indent':'0',color:'transparent'}}>{'\t'}</span>:null}{part}</>}</For>;
}

export function tabAdvance(current:number,following:number,decimal:number,stops:TabStop[],defaultTabStop=720):{width:number;leader?:string} {
 const next=stops.find(stop=>stop.align!=='bar'&&stop.align!=='clear'&&stop.position/15>current+0.5);
 const interval = defaultTabStop / 15;
 const target = next ? next.position / 15 : (Math.floor((current + 0.5) / interval) + 1) * interval;
 const shift=next?.align==='right'?following:next?.align==='center'?following/2:next?.align==='decimal'?decimal:0;
 return {width:Math.max(0,target-current-shift),leader:next?.leader};
}

export function layoutTabCharacters(editor:HTMLElement, defaultTabStop?: number) {
 let budget=1000;
 for(const paragraph of editor.querySelectorAll<HTMLElement>('[data-docx-tab-stops]')) {
  let stops:TabStop[];
  try {stops=JSON.parse(paragraph.dataset.docxTabStops??'[]') as TabStop[];}catch{continue;}
  const box=paragraph.getBoundingClientRect(),scale=paragraph.offsetWidth?box.width/paragraph.offsetWidth:1;
  if(!scale||!Number.isFinite(scale))continue;
  const origin=box.left-(parseFloat(getComputedStyle(paragraph).marginLeft)||0)*scale;
  const tabs=[...paragraph.querySelectorAll<HTMLElement>('[data-docx-tab]')];
  for(let i=0;i<tabs.length&&budget-->0;i++) {
   const tab=tabs[i]!;
   const current=(tab.getBoundingClientRect().left-origin)/scale;
   const range=document.createRange();range.setStartAfter(tab);
   if(tabs[i+1])range.setEndBefore(tabs[i+1]!);else {const last=paragraph.querySelectorAll('[data-docx-run]');const end=last.item(last.length-1);if(!end)continue;range.setEnd(end,end.childNodes.length);}
   const following=range.getBoundingClientRect().width/scale;
   // Decimal alignment uses the first visible dot or comma in the following
   // field. Complex script shaping remains the browser's responsibility.
   const walker=document.createTreeWalker(paragraph,NodeFilter.SHOW_TEXT);
   let node:Node|null,started=false,decimal=following;
   while((node=walker.nextNode())) {
    if(tab.contains(node)){started=true;continue;}
    if(!started)continue;
    if(tabs[i+1]?.contains(node))break;
    const offset=(node.textContent??'').search(/[.,]/);
    if(offset>=0){const prefix=range.cloneRange();prefix.setEnd(node,offset);decimal=prefix.getBoundingClientRect().width/scale;break;}
   }
   const result=tabAdvance(current,following,decimal,stops,defaultTabStop);
   tab.style.width=`${Math.min(result.width,Math.max(0,(box.right-origin)/scale-current))}px`;
   tab.style.borderBottom=result.leader==='dot'||result.leader==='middleDot'?'1px dotted currentColor':result.leader==='hyphen'?'1px dashed currentColor':result.leader==='underscore'?'1px solid currentColor':result.leader==='heavy'?'2px solid currentColor':'';
   tab.style.borderBottomColor='var(--docx-tab-leader, #475569)';
  }
  if(budget<=0)break;
 }
}
