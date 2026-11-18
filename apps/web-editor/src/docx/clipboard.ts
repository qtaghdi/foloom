import { utf8Length } from './validation';
import type { InlineStyle } from './contracts';
export type ClipboardRun = { text: string; style?: InlineStyle };
export type ClipboardParagraph = ClipboardRun[];
export type PasteMode = 'text' | 'basic';

const blockTags = new Set(['P','DIV','LI','H1','H2','H3','H4','H5','H6','PRE','TD','TH']);
const discarded = new Set(['SCRIPT','STYLE','IFRAME','OBJECT','EMBED','LINK','META','BASE','SVG','MATH','VIDEO','AUDIO','CANVAS','NOSCRIPT']);
const colors: Record<string,string> = { black:'#000000',white:'#ffffff',red:'#ff0000',green:'#008000',blue:'#0000ff',yellow:'#ffff00',gray:'#808080',silver:'#c0c0c0',maroon:'#800000',purple:'#800080',fuchsia:'#ff00ff',lime:'#00ff00',olive:'#808000',navy:'#000080',teal:'#008080',aqua:'#00ffff' };
function color(value: string): string | undefined {
  const normalized=value.trim().toLowerCase();
  if (/^#[0-9a-f]{6}$/.test(normalized)) return normalized;
  if (/^#[0-9a-f]{3}$/.test(normalized)) return '#'+[...normalized.slice(1)].map(c=>c+c).join('');
  const rgb=/^rgb\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*\)$/.exec(normalized);
  if (rgb && rgb.slice(1).every(n=>Number(n)<=255)) return '#'+rgb.slice(1).map(n=>Number(n).toString(16).padStart(2,'0')).join('');
  return colors[normalized];
}

// A template's content is inert and is never attached to the live document.
// Only bounded text and typed style values leave this parser. Attributes, URLs,
// foreign markup and resource-bearing nodes never enter the editor or server.
export function parseClipboardHTML(html: string, document: Document): { paragraphs: ClipboardParagraph[]; text: string; stripped: boolean } | undefined {
  if (!html || html.length>1024*1024) return;
  const template=document.createElement('template'); template.innerHTML=html;
  const paragraphs: ClipboardParagraph[]=[[]]; let nodes=0,bytes=0,stripped=false;
  const current=()=>paragraphs.at(-1)!;
  const append=(text:string,style:InlineStyle,preserve:boolean)=>{
    const value=preserve?text.replace(/\r\n?/g,'\n'):text.replace(/[\r\n\t ]+/g,' ');
    if (!value || !preserve && !current().length && !value.trim()) return;
    bytes+=value.length; if (bytes>1024*1024) throw new Error('clipboard limit');
    const previous=current().at(-1);
    if (previous && JSON.stringify(previous.style)===JSON.stringify(style)) previous.text+=value;
    else current().push({text:value,style:{...style}});
  };
  const boundary=()=>{ if (current().length) paragraphs.push([]); if (paragraphs.length>1001) throw new Error('clipboard paragraph limit'); };
  const walk=(node:Node,inherited:InlineStyle,preserve:boolean,depth:number)=>{
    if (++nodes>10000 || depth>32) throw new Error('clipboard structure limit');
    if (node.nodeType===Node.TEXT_NODE) {append(node.textContent??'',inherited,preserve);return;}
    if (!(node instanceof HTMLElement)) { if (node.nodeType===Node.ELEMENT_NODE) stripped=true; return; }
    const tag=node.tagName;
    if (discarded.has(tag)) {stripped=true;return;}
    if (tag==='IMG') {stripped=true;if (node.getAttribute('alt')) append(node.getAttribute('alt')!,inherited,preserve);return;}
    if (tag==='BR') {append('\n',inherited,true);return;}
    const style:InlineStyle={...inherited};
    if (tag==='B' || tag==='STRONG' || /^H[1-6]$/.test(tag)) style.bold=true;
    if (tag==='I' || tag==='EM') style.italic=true;
    const css=node.style;
    const decoration=(css.textDecorationLine||css.textDecoration||'').trim().split(/\s+/u).filter(Boolean);
    style.underline=Boolean(inherited.underline || decoration.includes('underline') || !decoration.length && (tag==='U'||tag==='INS'));
    style.strike=Boolean(inherited.strike || decoration.includes('line-through') || !decoration.length && (tag==='S'||tag==='STRIKE'||tag==='DEL'));
    if (decoration.some(line=>!['underline','line-through','none','solid'].includes(line)) ||
        css.textDecorationStyle && css.textDecorationStyle!=='solid' || css.textDecorationColor && css.textDecorationColor!=='currentcolor') stripped=true;
    if (css.fontWeight) { if (/^(?:normal|bold|[1-9]00)$/.test(css.fontWeight)) style.bold=css.fontWeight==='bold' || Number(css.fontWeight)>=600;else stripped=true; }
    if (css.fontStyle) { if (['normal','italic','oblique'].includes(css.fontStyle)) style.italic=css.fontStyle!=='normal';else stripped=true; }
    const font=css.fontFamily.split(',')[0]?.trim().replace(/^['"]|['"]$/g,'');
    if (font) { if (utf8Length(font)<=120 && !/["\\\u0000-\u001f\u007f]/u.test(font)) style.font=font; else stripped=true; }
    if (css.color || node.getAttribute('color')) {const value=color(css.color||node.getAttribute('color')!);if (value) style.color=value;else stripped=true;}
    const size=/^(\d+(?:\.\d+)?)(pt|px)$/.exec(css.fontSize);
    if (css.fontSize) {const value=size?Math.round(Number(size[1])*(size[2]==='pt'?2:1.5)):0;if (value>=8 && value<=288) style.sizeHalfPoints=value;else stripped=true;}
    for (const property of Array.from({length:css.length},(_,i)=>css.item(i))) if (!['font-weight','font-style','font-family','font-size','color','white-space','text-decoration','text-decoration-line','text-decoration-style','text-decoration-color'].includes(property)) stripped=true;
    for (const attribute of [...node.attributes]) if (!['style','color'].includes(attribute.name)) stripped=true;
    if (['A','TABLE','TR','OL','UL','LI'].includes(tag)) stripped=true;
    const block=blockTags.has(tag);if (block) boundary();
    const before=paragraphs.length;
    for (const child of node.childNodes) walk(child,style,preserve||tag==='PRE'||css.whiteSpace.startsWith('pre'),depth+1);
    if (block) { if (paragraphs.length===before && !current().length) current().push({text:'',style:{...style}}); boundary(); }
  };
  try {for (const child of template.content.childNodes) walk(child,{bold:false,italic:false,underline:false,strike:false},false,0);}
  catch {return;}
  while (paragraphs.length && !paragraphs.at(-1)!.length) paragraphs.pop();
  if (!paragraphs.length || paragraphs.length>1000) return;
  return {paragraphs,text:paragraphs.map(p=>p.map(r=>r.text).join('')).join('\n'),stripped};
}
const escape=(text:string)=>text.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;');
export function clipboardHTML(paragraphs: ClipboardParagraph[]): string {
  return paragraphs.map(p=>'<p style="white-space:pre-wrap">'+p.map(run=>{
    const s=run.style??{};
    const css=[`font-weight:${s.bold?'700':'400'}`,`font-style:${s.italic?'italic':'normal'}`];
    const decoration = [s.underline?'underline':'',s.strike?'line-through':''].filter(Boolean).join(' ');
    if (decoration) css.push(`text-decoration-line:${decoration}`);
    if (s.color && /^#[0-9a-f]{6}$/i.test(s.color)) css.push(`color:${s.color}`);
    if (s.font) css.push(`font-family:${JSON.stringify(s.font)}`);
    if (s.sizeHalfPoints) css.push(`font-size:${s.sizeHalfPoints/2}pt`);
    return `<span style="${escape(css.join(';'))}">${escape(run.text).replaceAll('\n','<br>')}</span>`;
  }).join('')+'</p>').join('');
}
