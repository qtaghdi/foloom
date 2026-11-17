import type { Snapshot, ListDefinition, Paragraph } from './contracts';

const romanDigits = [
  [1000,'M'],[900,'CM'],[500,'D'],[400,'CD'],[100,'C'],[90,'XC'],[50,'L'],
  [40,'XL'],[10,'X'],[9,'IX'],[5,'V'],[4,'IV'],[1,'I'],
] as const;

function formatListNumber(value: number, kind?: string): string | undefined {
  if (kind==='decimal'||kind==='bullet') return String(value);
  if (kind!=='upperRoman'&&kind!=='lowerRoman'||value<1||value>3999) return undefined;
  let remaining=value, result='';
  for (const [number,symbol] of romanDigits) {
    const count=Math.floor(remaining/number);
    result+=symbol.repeat(count);
    remaining-=number*count;
  }
  return kind==='lowerRoman'?result.toLowerCase():result;
}

// This preview deliberately labels unsupported imported numbering formats.
// The original numbering XML remains authoritative when reopened in Word.
export function previewLists(snapshot: Snapshot, drafts: ListDefinition[] = []): Snapshot {
  const catalog = new Map<string, NonNullable<Snapshot['lists']>[number]['levels']>(
    snapshot.lists?.map(info => [String(info.id), info.levels]),
  );
  for (const d of drafts) catalog.set(d.key, Array.from({length:9}, (_,level)=>({
    level,kind:d.kind,text:d.kind==='bullet'?['•','◦','▪'][level%3]!:`%${level+1}.`,start:level===0?d.start??1:1,
  })));
  const counters = new Map<string, Array<number | undefined>>();
  const paragraph = (p: Paragraph): Paragraph => {
    const ref = p.list;
    if (!ref || !ref.id && !ref.key) return {...p,listLabel:undefined};
    const id = ref.key ?? String(ref.id), level = ref.level ?? 0;
    const levels = catalog.get(id), definition = levels?.find(l=>l.level===level);
    if (!definition) return {...p,listLabel:undefined};
    const values = counters.get(id) ?? [];
    values[level] = values[level]===undefined?Math.max(1,definition.start):values[level]!+1;
    values.length = level+1; counters.set(id,values);
    let supported=true;
    const label=definition.text.replace(/%([1-9])/g, (_match,n:string)=>{
      const i=Number(n)-1,d=levels?.find(l=>l.level===i);
      const formatted=formatListNumber(values[i]??Math.max(1,d?.start??1),definition.legal?'decimal':d?.kind);
      if (formatted===undefined) supported=false;
      return formatted??'';
    });
    return {...p,listLabel:supported?label:`[${definition.kind}]`};
  };
  return {...snapshot,blocks:snapshot.blocks.map(b=>b.type==='paragraph'?{...b,paragraph:paragraph(b.paragraph)}:
    {...b,rows:b.rows.map(r=>({...r,cells:r.cells.map(c=>({...c,paragraphs:c.paragraphs.map(paragraph)}))}))})};
}
