export function docxName(input:string):string|undefined {
 let name=input.trim().normalize('NFC');
 if(!name||name.startsWith('.')||/[/\\:*?"<>|\u0000-\u001f\u007f]/u.test(name)||/^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(?:\.|$)/i.test(name))return;
 if(!/\.docx$/i.test(name)){
  if(/\.(xlsx|xls|pdf|pptx|doc|txt)$/i.test(name))return;
  name+='.docx';
 }
 if(new TextEncoder().encode(name).length>240||/\.\.docx$/i.test(name))return;
 return name;
}
export function uniqueDOCXName(base:string,names:string[]):string {
 const used=new Set(names.map(name=>name.normalize('NFC').toLowerCase()));
 for(let count=1;count<=9;count++){
  const name=docxName(`${base}${count===1?'':` (${count})`}`)!;
  if(!used.has(name.toLowerCase()))return name;
 }
 return docxName(`${base} (10)`)!;
}
