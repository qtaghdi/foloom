import { restoreTextSelection, type TextPoint, type TextSelection } from './selection';
import type { SearchItem, SearchOptions, SearchResults } from '../ui/document-search';

const maxResults = 200;
const wordSegments = new Intl.Segmenter(undefined,{granularity:'word'});
const literal = (value:string) => value.replace(/[.*+?^${}()|[\]\\]/g,'\\$&');

type Match = { item:SearchItem; selection?:TextSelection; current:()=>boolean };

export function searchDocument(
  root:HTMLElement|null,
  query:string,
  options:SearchOptions = {},
  replace?: (ranges:TextSelection[],text:string)=>boolean,
):SearchResults {
  if (!query.trim() || !root || query.length>200) return {items:[]};
  const pattern = new RegExp(literal(query),options.matchCase?'gu':'giu');
  const found:Match[] = [];
  let truncated = false;
  for (const paragraph of root.querySelectorAll<HTMLElement>('.docx-paragraph')) {
    const inTable = Boolean(paragraph.closest('td,th'));
    if (options.scope==='table' && !inTable || options.scope==='body' && inTable) continue;
    const readFields=()=>[...paragraph.querySelectorAll<HTMLElement>('textarea, [data-docx-run], [data-docx-text], img')];
    const fields = readFields();
    // A picture is a non-text boundary. Do not match across it.
    const valueOf = (field:HTMLElement) => field instanceof HTMLTextAreaElement ? field.value :
      field instanceof HTMLImageElement ? '\ufffc' : field.textContent ?? '';
    const values = fields.map(valueOf);
    const text = values.join('');
    pattern.lastIndex = 0;
    if (!pattern.test(text)) continue;
    pattern.lastIndex = 0;
    const boundaries = options.wholeWord ? new Set([...wordSegments.segment(text)].flatMap(({index,segment})=>[index,index+segment.length])) : undefined;
    for (const match of text.matchAll(pattern)) {
      const start = match.index, end = start+match[0].length;
      if (boundaries && (!boundaries.has(start)||!boundaries.has(end))) continue;
      if (found.length===maxResults) {truncated=true;break;}
      let offset=0, first=-1,last=-1, from=0,to=0;
      for (const [i,value] of values.entries()) {
        if(first<0 && start<offset+value.length){first=i;from=start-offset;}
        if(end<=offset+value.length){last=i;to=end-offset;break;}
        offset+=value.length;
      }
      const anchor=fields[first],focus=fields[last];
      const point = (field:HTMLElement|undefined,offset:number):TextPoint|undefined =>
        field?.dataset.docxRun ? {runId:field.dataset.docxRun,offset}:undefined;
      const a=point(anchor,from),b=point(focus,to);
      const selection=a&&b&&fields.slice(first,last+1).every(field=>field.dataset.docxRun) ? {anchor:a,focus:b}:undefined;
      const current=()=>{
        if(!paragraph.isConnected)return false;
        const live=readFields();
        return live.length===fields.length&&live.every((field,i)=>field===fields[i]&&valueOf(field)===values[i]);
      };
      const locate=()=>{
        if(!current())return;
        paragraph.scrollIntoView({block:'center'});
        if(anchor instanceof HTMLTextAreaElement){anchor.focus({preventScroll:true});anchor.setSelectionRange(from,first===last?to:values[first]!.length);}
        else if(selection){
          const editor=paragraph.closest<HTMLElement>('[data-continuous-editor]');
          if(editor)restoreTextSelection(editor,selection);
        }else paragraph.focus({preventScroll:true});
      };
      found.push({selection,current,item:{
        label:`본문·표 위치 ${found.length+1}`,
        text:text.slice(Math.max(0,start-40),end+120).replaceAll('\ufffc',' '),
        locate,
        replace:replace&&selection ? replacement=>current()&&replace([selection],replacement):undefined,
      }});
    }
    if(truncated)break;
  }
  const editable=found.filter((match):match is Match&{selection:TextSelection}=>Boolean(match.selection));
  return {
    items:found.map(match=>match.item),truncated,
    replaceAll:replace&&!truncated&&editable.length ? replacement=>
      editable.every(match=>match.current())&&replace(editable.map(match=>match.selection),replacement):undefined,
    protectedCount:found.length-editable.length,
  };
}
