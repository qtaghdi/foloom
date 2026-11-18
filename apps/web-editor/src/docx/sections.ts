import type { SectionFormat,SectionInfo,HeaderFooterInfo } from './contracts';

export const defaultHeaderFooterDistance = 720;

export function previewSection(original:SectionInfo,format:SectionFormat):SectionInfo {
 const page={...original.page};
 for(const key of ['width','height','top','right','bottom','left'] as const)if(format[key]!==undefined)page[key]=format[key]!;
 return {
  ...original,page,gutter:format.gutter??original.gutter,
  headerDistance:format.headerDistance??original.headerDistance,
  footerDistance:format.footerDistance??original.footerDistance,
  pageNumberStart:format.pageNumberStart===-1?undefined:format.pageNumberStart??original.pageNumberStart,
  pageNumberFormat:format.pageNumberFormat??original.pageNumberFormat,
  firstPage:format.firstPage??original.firstPage,
  orientation:format.orientation??original.orientation,
  columns:format.columns??original.columns,
  columnSpace:format.columnSpace??original.columnSpace,
  breakType:format.breakType??original.breakType,
 };
}
export function sectionPaper(section:SectionInfo):SectionInfo['page'] {
 const page=section.page;
 if(section.gutterAtTop) return {...page,top:page.top+topBindingSpace(section)};
 const safeGutter=Math.min(section.gutter??0,Math.max(0,page.width-page.left-page.right-1200));
 return safeGutter?{...page,left:page.left+safeGutter}:page;
}

export function topBindingSpace(section:SectionInfo):number {
 const {page}=section;
 return section.gutterAtTop ? Math.min(section.gutter??0,Math.max(0,page.height-page.top-page.bottom-1200)) : 0;
}

export function validateSection(section:SectionInfo):string {
 const {page,columns,columnSpace}=section;
 const gutter=section.gutter??0;
 const bodyWidth=page.width-page.left-page.right-(section.gutterAtTop?0:gutter);
 const bodyHeight=page.height-page.top-page.bottom-(section.gutterAtTop?gutter:0);
 if(bodyWidth<1200||bodyHeight<1200)return '여백을 줄여 본문 영역을 확보하세요.';
 if(bodyWidth-(columns-1)*columnSpace<columns*240)return '단 수나 간격을 줄여 본문 폭을 확보하세요.';
 return '';
}


// Only explicit page groups contribute to this model. A section that starts on
// an odd or even page can require one blank page before its first body page.
export type PreviewPageLayout = {
 section?:SectionInfo;
 variant:HeaderFooterInfo['variant'];
 blankBefore:boolean;
 number:number;
};

export function previewPageLayout(
 pages: readonly (SectionInfo|undefined)[],
 evenAndOddHeaders?: boolean,
): PreviewPageLayout[] {
 let previous: string|undefined;
 let ordinal=0;
 let nextNumber: number|undefined=1;
 let continuedWithoutRestart=true;
 let number=0;
 return pages.map((section,index)=>{
  const first=index===0||section?.id!==previous;
  let blankBefore=false;
  if(first){
   ordinal=0;
   const breakType=section?.breakType;
   if(section?.pageNumberStart!==undefined){
    continuedWithoutRestart=false;
    nextNumber=section.pageNumberStart;
   }else if(index>0&&breakType!=='nextPage'){
    if(continuedWithoutRestart&&nextNumber!==undefined&&(breakType==='oddPage'||breakType==='evenPage')){
     const parity=breakType==='oddPage'?1:0;
     blankBefore=nextNumber%2!==parity;
     if(blankBefore)nextNumber++;
    }else{
     nextNumber=undefined;
    }
   }
  }
  previous=section?.id;
  ordinal++;
  const pageNumber=nextNumber??ordinal;
  if(nextNumber!==undefined)nextNumber++;
  const variant=first&&section?.firstPage ? 'first' : evenAndOddHeaders&&pageNumber%2===0 ? 'even' : 'default';
  number+=blankBefore?2:1;
  return {section,variant,blankBefore,number};
 });
}

export function previewSections(original:SectionInfo[],updates:Map<string,SectionFormat>):SectionInfo[] {
 const previous=new Map<string,HeaderFooterInfo>();
 return original.map(section=>{
  const patch=updates.get(section.id)??{},next=previewSection(section,patch);
  next.headerFooters=section.headerFooters?.map(slot=>{
   const key=slot.kind+':'+slot.variant,update=patch.headerFooters?.find(item=>item.kind===slot.kind&&item.variant===slot.variant);
   let info={...slot};
   if(update&&!update.linkPrevious) {
    const pageNumber=update.pageNumber??slot.pageNumber;
    const totalPages=update.totalPages??slot.totalPages;
    info={
     ...info,text:update.text??'',pageNumber,totalPages,
     compactPageNumbers:pageNumber&&totalPages&&(update.compactPageNumbers??slot.compactPageNumbers),
     linked:false,sourceSectionId:section.id,editable:true,truncated:false,
    };
   } else if(update?.linkPrevious||slot.linked) {
    const inherited=previous.get(key);
    info={
     ...info,text:'',linked:true,pageNumber:inherited?.pageNumber,totalPages:inherited?.totalPages,
     compactPageNumbers:inherited?.compactPageNumbers,sourceSectionId:inherited?.sourceSectionId,
     editable:inherited?.editable??true,truncated:inherited?.truncated,
    };
   }
   previous.set(key,info);return info;
  });
  return next;
 });
}
